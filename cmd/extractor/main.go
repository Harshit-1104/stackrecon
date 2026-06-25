package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/joho/godotenv"
	"golang.org/x/time/rate"
	"google.golang.org/api/option"

	"github.com/stackrecon/internal/models"
)

type JobRequest struct {
	Slug string
	Job  models.GreenhouseJob
}

type SlugData struct {
	TechExtractedJobs    []models.ExtractedJob
	NonTechExtractedJobs []models.ExtractedJob
	RemainingJDs         int
	Mu                   sync.Mutex
}

var globalRunLog models.RunLog
var runLogMu sync.Mutex
var slugMap = make(map[string]*SlugData)
var slugMapMu sync.Mutex

var totalCompanies int
var companiesCompleted int
var totalJDsToProcess int
var progressMu sync.Mutex

func getSlugData(slug string) *SlugData {
	slugMapMu.Lock()
	defer slugMapMu.Unlock()
	if sd, ok := slugMap[slug]; ok {
		return sd
	}
	sd := &SlugData{}
	slugMap[slug] = sd
	return sd
}

func main() {
	inputDir := flag.String("input-dir", "data/processed/tech", "Directory containing the JSON files to process")
	workers := flag.Int("workers", 5, "Number of concurrent workers")
	flag.Parse()

	godotenv.Load()

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY not set")
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("Failed to create GenAI client: %v", err)
	}
	defer client.Close()

	files, err := filepath.Glob(filepath.Join(*inputDir, "*.json"))
	if err != nil {
		log.Fatalf("Failed to glob processed files: %v", err)
	}

	os.MkdirAll(filepath.Join("data", "extracted", "tech"), 0755)
	os.MkdirAll(filepath.Join("data", "extracted", "non_tech"), 0755)
	os.MkdirAll(dataLogsDir(), 0755)

	globalRunLog = models.RunLog{
		RunAt:      time.Now().Format(time.RFC3339),
		FailedJobs: []models.FailedJobLog{},
	}

	// 15 requests per second rate limit
	limiter := rate.NewLimiter(rate.Limit(15), 1)

	jobsChan := make(chan JobRequest, 100000)
	var wg sync.WaitGroup

	totalCompanies = len(files)

	for _, file := range files {
		slug := strings.TrimSuffix(filepath.Base(file), ".json")

		techExtractedPath := filepath.Join("data", "extracted", "tech", slug+".json")
		nonTechExtractedPath := filepath.Join("data", "extracted", "non_tech", slug+".json")

		b, err := os.ReadFile(file)
		if err != nil {
			log.Printf("Failed to read %s: %v", file, err)
			continue
		}

		var jobs []models.GreenhouseJob
		if err := json.Unmarshal(b, &jobs); err != nil {
			log.Printf("Failed to unmarshal %s: %v", file, err)
			continue
		}

		sd := getSlugData(slug)
		processedJDs := make(map[string]bool)

		if eb, err := os.ReadFile(techExtractedPath); err == nil {
			if err := json.Unmarshal(eb, &sd.TechExtractedJobs); err == nil {
				for _, ej := range sd.TechExtractedJobs {
					processedJDs[ej.PostingID] = true
				}
			}
		}

		if eb, err := os.ReadFile(nonTechExtractedPath); err == nil {
			if err := json.Unmarshal(eb, &sd.NonTechExtractedJobs); err == nil {
				for _, ej := range sd.NonTechExtractedJobs {
					processedJDs[ej.PostingID] = true
				}
			}
		}

		if len(sd.TechExtractedJobs)+len(sd.NonTechExtractedJobs) >= len(jobs) && len(jobs) > 0 {
			log.Printf("[%s] already fully processed (%d JDs), skipping", slug, len(jobs))
			progressMu.Lock()
			companiesCompleted++
			progressMu.Unlock()
			continue
		} else if len(jobs) == 0 {
			log.Printf("[%s] has 0 jobs, skipping", slug)
			progressMu.Lock()
			companiesCompleted++
			progressMu.Unlock()
			continue
		}

		var jobsToProcess int
		for _, job := range jobs {
			postingID := fmt.Sprintf("%d", job.ID)
			if processedJDs[postingID] {
				continue
			}
			jobsToProcess++
			jobsChan <- JobRequest{Slug: slug, Job: job}
		}

		sd.Mu.Lock()
		sd.RemainingJDs = jobsToProcess
		sd.Mu.Unlock()

		progressMu.Lock()
		totalJDsToProcess += jobsToProcess
		if jobsToProcess == 0 {
			companiesCompleted++
		}
		progressMu.Unlock()
	}

	for i := 0; i < *workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for req := range jobsChan {
				processJob(ctx, client, limiter, req)
			}
		}()
	}

	close(jobsChan)
	wg.Wait()

	progressFile := filepath.Join(dataLogsDir(), "progress.log")
	f, err := os.OpenFile(progressFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		for slug, sd := range slugMap {
			totalExtracted := len(sd.TechExtractedJobs) + len(sd.NonTechExtractedJobs)
			if totalExtracted > 0 {
				f.WriteString(fmt.Sprintf("%s: Company %s processing completed, %d JDs extracted\n", time.Now().Format(time.RFC3339), slug, totalExtracted))
			}
		}
		f.Close()
	}

	timestamp := time.Now().Format("20060102_150405")
	logFilename := filepath.Join(dataLogsDir(), fmt.Sprintf("extractor_%s.json", timestamp))
	logBytes, _ := json.MarshalIndent(globalRunLog, "", "  ")
	os.WriteFile(logFilename, logBytes, 0644)

	fmt.Printf("\nJobs attempted:          %d\n", globalRunLog.TotalJobsAttempted)
	fmt.Printf("Jobs succeeded:          %d\n", globalRunLog.TotalJobsSucceeded)
	fmt.Printf("Jobs skipped (non-tech): %d\n", globalRunLog.TotalJobsSkippedNonTech)
	fmt.Printf("Jobs failed:             %d\n", globalRunLog.TotalJobsFailed)
	fmt.Printf("Total skills found:      %d\n", globalRunLog.TotalSkillsExtracted)
	fmt.Printf("\nLog: %s\n", logFilename)
}

func processJob(ctx context.Context, client *genai.Client, limiter *rate.Limiter, req JobRequest) {
	slug := req.Slug
	job := req.Job
	postingID := fmt.Sprintf("%d", job.ID)

	runLogMu.Lock()
	globalRunLog.TotalJobsAttempted++
	runLogMu.Unlock()

	var depts []string
	for _, d := range job.Departments {
		depts = append(depts, d.Name)
	}

	var cleaned string
	if job.CleanedContent != "" {
		cleaned = job.CleanedContent
	} else {
		cleaned = prepareContent(job.Content)
	}
	prompt := fmt.Sprintf(getPromptTemplate(), cleaned)

	var extractedSkills []models.Skill
	status := "success"

	var geminiResp *models.GeminiResponse
	var processErr error

	// Rate limiter
	if err := limiter.Wait(ctx); err != nil {
		log.Printf("[%s %s] Rate limiter wait error: %v", slug, postingID, err)
	}

	for attempt := 1; attempt <= 4; attempt++ {
		reqCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		geminiResp, processErr = callGemini(reqCtx, client, prompt)
		cancel()

		if processErr == nil {
			break
		}

		if attempt < 4 {
			backoff := time.Duration(attempt*attempt) * 5 * time.Second
			log.Printf("[%s %s] Attempt %d failed (%v), retrying in %s", slug, postingID, attempt, processErr, backoff)
			time.Sleep(backoff)
			_ = limiter.Wait(ctx)
		}
	}

	if processErr != nil {
		log.Printf("[%s %s] API error after retries: %v", slug, postingID, processErr)
		status = "extraction_failed"
		runLogMu.Lock()
		globalRunLog.TotalJobsFailed++
		globalRunLog.FailedJobs = append(globalRunLog.FailedJobs, models.FailedJobLog{
			PostingID:   postingID,
			CompanyName: job.CompanyName,
			Reason:      processErr.Error(),
		})
		runLogMu.Unlock()
	} else {
		if geminiResp.IsTech {
			extractedSkills = geminiResp.Skills
			runLogMu.Lock()
			globalRunLog.TotalJobsSucceeded++
			globalRunLog.TotalSkillsExtracted += len(extractedSkills)
			runLogMu.Unlock()
		} else {
			status = "skipped_non_tech"
			runLogMu.Lock()
			globalRunLog.TotalJobsSkippedNonTech++
			runLogMu.Unlock()
		}
	}

	extracted := models.ExtractedJob{
		PostingID:        postingID,
		CompanyName:      job.CompanyName,
		RawTitle:         job.Title,
		ApplyURL:         job.AbsoluteURL,
		Location:         job.Location.Name,
		PostedAt:         job.FirstPublished,
		LastSeenAt:       job.UpdatedAt,
		Departments:      depts,
		CleanedContent:   cleaned,
		Skills:           extractedSkills,
		MinYearsRequired: geminiResp.MinYearsRequired,
		ExtractionStatus: status,
	}

	sd := getSlugData(slug)
	sd.Mu.Lock()
	defer sd.Mu.Unlock()

	techExtractedPath := filepath.Join("data", "extracted", "tech", slug+".json")
	nonTechExtractedPath := filepath.Join("data", "extracted", "non_tech", slug+".json")

	switch status {
	case "success", "extraction_failed":
		sd.TechExtractedJobs = append(sd.TechExtractedJobs, extracted)
		outBytes, _ := json.MarshalIndent(sd.TechExtractedJobs, "", "  ")
		os.WriteFile(techExtractedPath, outBytes, 0644)
		if status == "success" {
			log.Printf("[%s %s] extracted %d skills", slug, postingID, len(extractedSkills))
		} else {
			log.Printf("[%s %s] extraction failed", slug, postingID)
		}
	case "skipped_non_tech":
		sd.NonTechExtractedJobs = append(sd.NonTechExtractedJobs, extracted)
		outBytes, _ := json.MarshalIndent(sd.NonTechExtractedJobs, "", "  ")
		os.WriteFile(nonTechExtractedPath, outBytes, 0644)
		log.Printf("[%s %s] skipped non-tech", slug, postingID)
	}

	sd.RemainingJDs--
	slugRemaining := sd.RemainingJDs

	runLogMu.Lock()
	completedJDs := globalRunLog.TotalJobsSucceeded + globalRunLog.TotalJobsSkippedNonTech + globalRunLog.TotalJobsFailed
	runLogMu.Unlock()

	progressMu.Lock()
	if slugRemaining == 0 {
		companiesCompleted++
	}
	jdsRem := totalJDsToProcess - completedJDs
	compsRem := totalCompanies - companiesCompleted
	progressMu.Unlock()

	log.Printf("[%s] Progress -> JDs remaining: %d/%d, Companies remaining: %d/%d", slug, jdsRem, totalJDsToProcess, compsRem, totalCompanies)
}

func callGemini(ctx context.Context, client *genai.Client, prompt string) (*models.GeminiResponse, error) {
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
	var gr models.GeminiResponse
	if err := json.Unmarshal([]byte(rawJSON), &gr); err != nil {
		return nil, fmt.Errorf("json parse failure. Raw: %s", rawJSON)
	}

	return &gr, nil
}

func prepareContent(raw string) string {
	s := html.UnescapeString(raw)

	reHTML := regexp.MustCompile(`<[^>]*>`)
	s = reHTML.ReplaceAllString(s, " ")

	reSpace := regexp.MustCompile(`\s+`)
	s = reSpace.ReplaceAllString(s, " ")
	cleaned := strings.TrimSpace(s)

	return cleaned
}

func dataLogsDir() string {
	return filepath.Join("data", "logs")
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
- When alternatives are listed explicitly (e.g. "FastAPI, Django, or Flask" /
  "React, Angular, or Vue.js"), mark all as "mentioned" not "required" —
  any one satisfies the requirement
- Do NOT extract from open-ended example lists that illustrate a generic
  category using "etc.", "e.g.", or "such as" (e.g. "a backend language
  (Go, Java, Python, etc.)", "a major cloud provider, e.g. AWS or Azure").
  These name the category, not a fixed requirement — extract zero skills
  from them.
- Only CLOSED lists — a complete, fixed set of named alternatives with no
  "etc."/"e.g." (e.g. "FastAPI, Django, or Flask") — follow the "mentioned"
  rule above.

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

Input:
"""
Software Engineer | Example Co | Engineering
Required: proficiency in a backend language (Go, Java, Python, etc.) and
experience with a major cloud provider (AWS, Azure, GCP, etc.).
Must have hands-on experience with FastAPI, Django, or Flask.
"""
Output:
{"is_tech":true,"min_years_required":null,"skills":[{"name":"FastAPI","requirement_level":"mentioned"},{"name":"Django","requirement_level":"mentioned"},{"name":"Flask","requirement_level":"mentioned"}]}

Job Description:
%s
`
}
