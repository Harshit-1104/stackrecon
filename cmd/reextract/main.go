package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"golang.org/x/time/rate"
	"google.golang.org/api/option"
)

type JD struct {
	ID         string
	Content    string
	CompanyID  int
	Slug       string
	SkillCount int
}

type Skill struct {
	Name             string `json:"skill_name"`
	RequirementLevel string `json:"requirement_level"`
	ExtractedBy      string `json:"extracted_by"`
}

type OutputFormat struct {
	JobID            string  `json:"job_id"`
	Slug             string  `json:"slug"`
	IsTech           bool    `json:"is_tech"`
	MinYearsRequired *int    `json:"min_years_required"`
	Skills           []Skill `json:"skills"`
}

type GeminiSkill struct {
	Name             string `json:"name"`
	RequirementLevel string `json:"requirement_level"`
}

type GeminiResponse struct {
	IsTech           bool          `json:"is_tech"`
	MinYearsRequired *int          `json:"min_years_required"`
	Skills           []GeminiSkill `json:"skills"`
}

func getPromptTemplate() string {
	return `You are a technical job description classifier and skill extractor.

First, determine if this is a technical/engineering role (software, hardware,
data, ML, security engineering, etc.).
Non-technical roles include: business analytics, sales, marketing, HR,
operations, finance, product management, design.

If it IS technical, extract all technical skills with requirement level.
If NOT technical, return is_tech: false with empty skills array.

Requirement levels:
- "required": must-have, explicitly required
- "preferred": nice-to-have, bonus, preferred
- "mentioned": appears in context, not classified above

Skill extraction rules:
- Technical skills only: languages, frameworks, libraries, databases,
  cloud platforms, tools, protocols, ML frameworks, hardware/IC tools
- Only extract from responsibilities and qualifications sections.
  Ignore company description/intro paragraphs entirely.
- No soft skills, no years of experience, no domain knowledge
- Canonical names only (e.g. "PostgreSQL" not "Postgres database")
- Use section headings (Required/Preferred/Nice to have) to infer level
- Do NOT extract overly generic terms as skills: "AI", "data", "cloud",
  "hardware", "software", "automation", "monitoring", "infrastructure",
  "full-stack development", "backend systems", "frontend interfaces"
- Do NOT extract certifications (e.g. "AWS Solutions Architect cert") or
  compliance standards (e.g. "GDPR", "HIPAA") as skills
- Do NOT extract methodology/process terms (e.g. "Agile", "Scrum", "TDD",
  "responsive design", "performance profiling") as skills
- When alternatives are listed explicitly (e.g. "FastAPI, Django, or Flask" /
  "React, Angular, or Vue.js"), mark all as "mentioned" not "required" —
  any one satisfies the requirement
- If the same technology appears under two names (e.g. "GCP" and
  "Google Cloud Platform"), include only the canonical form: "GCP"

Also extract:
"min_years_required": integer or null

Rules:
- Look for explicit statements only: "X+ years", "minimum X years", "at least X years of experience"
- Do NOT infer from seniority titles (Senior, Staff etc.)
- Return null if not explicitly stated
- If a range is given (3-5 years), return the lower bound (3)

Return ONLY valid JSON, no markdown:
{ "is_tech": true, "min_years_required": 3, "skills": [{ "name": "X", "requirement_level": "required" }] }

EXAMPLES:

Input:
"""
Data Protection Engineer | Coinbase | Engineering - Security
Build and maintain DLP controls across iOS and Chrome environments.
Drive automation leveraging LLMs and agentic AI.
Required: 3+ years security engineering, hands-on DLP implementation,
experience with SIEM, UBA, endpoint detection, ML/AI tooling.
Nice to haves: Experience in Web3 and crypto organizations.
"""
Output:
{"is_tech":true,"min_years_required":3,"skills":[{"name":"DLP","requirement_level":"required"},{"name":"SIEM","requirement_level":"required"},{"name":"UBA","requirement_level":"required"},{"name":"LLMs","requirement_level":"required"},{"name":"iOS","requirement_level":"mentioned"},{"name":"Chrome","requirement_level":"mentioned"},{"name":"Web3","requirement_level":"preferred"}]}

Input:
"""
Business Analytics Lead | PhonePe | Business Analytics
Own analytics delivery for Merchant Business Unit.
Requirements: 7+ years analytics, strong SQL skills, QlikSense is a plus,
Python or R is a good-to-have.
"""
Output:
{"is_tech":false,"min_years_required":null,"skills":[]}

Input:
"""
Full Stack Engineer | Acme Corp | Software Engineering
Required: Python with FastAPI, Django, or Flask. Proficiency in React
and TypeScript. Cloud deployments on Azure or GCP.
Nice to have: Docker, Kubernetes.
"""
Output:
{"is_tech":true,"min_years_required":null,"skills":[{"name":"Python","requirement_level":"required"},{"name":"FastAPI","requirement_level":"mentioned"},{"name":"Django","requirement_level":"mentioned"},{"name":"Flask","requirement_level":"mentioned"},{"name":"React","requirement_level":"required"},{"name":"TypeScript","requirement_level":"required"},{"name":"Azure","requirement_level":"mentioned"},{"name":"GCP","requirement_level":"mentioned"},{"name":"Docker","requirement_level":"preferred"},{"name":"Kubernetes","requirement_level":"preferred"}]}

Job Description:
%s`
}

func callGemini(ctx context.Context, client *genai.Client, prompt string) (*GeminiResponse, error) {
	model := client.GenerativeModel("gemini-2.5-flash-lite")
	model.ResponseMIMEType = "application/json"

	resp, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, fmt.Errorf("generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("empty response candidates")
	}

	part := resp.Candidates[0].Content.Parts[0]
	textPart, ok := part.(genai.Text)
	if !ok {
		return nil, fmt.Errorf("no text part in response")
	}

	rawJSON := string(textPart)
	var gr GeminiResponse
	if err := json.Unmarshal([]byte(rawJSON), &gr); err != nil {
		return nil, fmt.Errorf("json parse failure. Raw: %s", rawJSON)
	}

	return &gr, nil
}

func main() {
	godotenv.Load()
	
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY not set")
	}

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:password@localhost:5432/stackrecon"
	}

	ctx := context.Background()
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer conn.Close(ctx)

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("Failed to create GenAI client: %v", err)
	}
	defer client.Close()

	query := `
	SELECT jp.id, jp.content, jp.company_id, cs.source_identifier as slug, COUNT(jps.skill_id) as skill_count
	FROM job_posting jp
	JOIN company_source cs ON cs.company_id = jp.company_id
	LEFT JOIN job_posting_skill jps ON jps.posting_id = jp.id
	WHERE jp.active = true
	GROUP BY jp.id, jp.content, jp.company_id, cs.source_identifier
	HAVING COUNT(jps.skill_id) < 5
	`

	rows, err := conn.Query(ctx, query)
	if err != nil {
		log.Fatalf("Error querying JDs: %v\n", err)
	}

	var jds []JD
	for rows.Next() {
		var jd JD
		if err := rows.Scan(&jd.ID, &jd.Content, &jd.CompanyID, &jd.Slug, &jd.SkillCount); err != nil {
			log.Fatalf("Scan error: %v", err)
		}
		jds = append(jds, jd)
	}
	rows.Close()

	fmt.Printf("Found %d JDs to reprocess\n", len(jds))

	var totalReprocessed, totalNewSkillsAdded int
	var avgSkillBefore, avgSkillAfter float64
	var statsMu sync.Mutex

	limiter := rate.NewLimiter(rate.Limit(20), 1) // 20 req/sec

	jdChan := make(chan JD, len(jds))
	for _, jd := range jds {
		jdChan <- jd
	}
	close(jdChan)

	var wg sync.WaitGroup
	workers := 50

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for jd := range jdChan {
				limiter.Wait(ctx)
				prompt := fmt.Sprintf(getPromptTemplate(), jd.Content)
				
				dir := filepath.Join("data", "reextracted", jd.Slug)
				os.MkdirAll(dir, 0755)
				outPath := filepath.Join(dir, fmt.Sprintf("%s.json", jd.ID))
				
				// check if already extracted
				if _, err := os.Stat(outPath); err == nil {
					statsMu.Lock()
					totalReprocessed++
					if totalReprocessed%100 == 0 {
						log.Printf("Reprocessed %d/%d (cached)\n", totalReprocessed, len(jds))
					}
					statsMu.Unlock()
					continue
				}

				var geminiResp *GeminiResponse
				var processErr error
				for attempt := 1; attempt <= 3; attempt++ {
					reqCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
					geminiResp, processErr = callGemini(reqCtx, client, prompt)
					cancel()
					if processErr == nil {
						break
					}
					time.Sleep(time.Duration(attempt) * time.Second)
				}

				if processErr != nil {
					log.Printf("Job %s failed extraction: %v\n", jd.ID, processErr)
					continue
				}

				outSkills := make([]Skill, 0)
				for _, sk := range geminiResp.Skills {
					reqLvl := strings.ToLower(sk.RequirementLevel)
					if reqLvl != "required" && reqLvl != "preferred" {
						reqLvl = "mentioned"
					}
					outSkills = append(outSkills, Skill{
						Name:             sk.Name,
						RequirementLevel: reqLvl,
						ExtractedBy:      "llm",
					})
				}

				output := OutputFormat{
					JobID:            jd.ID,
					Slug:             jd.Slug,
					IsTech:           geminiResp.IsTech,
					MinYearsRequired: geminiResp.MinYearsRequired,
					Skills:           outSkills,
				}

				b, _ := json.MarshalIndent(output, "", "  ")
				os.WriteFile(outPath, b, 0644)

				statsMu.Lock()
				avgSkillBefore += float64(jd.SkillCount)
				avgSkillAfter += float64(len(outSkills))
				totalNewSkillsAdded += len(outSkills)
				totalReprocessed++
				if totalReprocessed%100 == 0 {
					log.Printf("Reprocessed %d/%d\n", totalReprocessed, len(jds))
				}
				statsMu.Unlock()
			}
		}()
	}

	wg.Wait()

	if totalReprocessed > 0 {
		avgSkillBefore /= float64(totalReprocessed)
		avgSkillAfter /= float64(totalReprocessed)
	}

	fmt.Printf("\nDone!\n")
	fmt.Printf("Total processed: %d\n", totalReprocessed)
	fmt.Printf("Total new skills found: %d\n", totalNewSkillsAdded)
	fmt.Printf("Avg skill count before: %.2f\n", avgSkillBefore)
	fmt.Printf("Avg skill count after: %.2f\n", avgSkillAfter)
}
