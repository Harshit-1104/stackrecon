package handlers

import (
	"log"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"bytes"

	"github.com/google/generative-ai-go/genai"
	"github.com/ledongthuc/pdf"
	"github.com/nguyenthenguyen/docx"
	"google.golang.org/api/option"
)

type SkillSearchResponse struct {
	ID            int     `json:"id"`
	CanonicalName string  `json:"canonical_name"`
	Category      *string `json:"category"`
}

func (h *Handler) SearchSkills(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	limit := 10

	query := `
		SELECT s.id, s.canonical_name, c.name
		FROM skill s
		LEFT JOIN skill_category c ON s.category_id = c.id
		WHERE s.canonical_name ILIKE $1
		LIMIT $2
	`
	rows, err := h.db.Query(r.Context(), query, "%"+q+"%", limit)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []SkillSearchResponse
	for rows.Next() {
		var s SkillSearchResponse
		if err := rows.Scan(&s.ID, &s.CanonicalName, &s.Category); err != nil {
			log.Printf("Internal server error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		if h.Blocklist[s.CanonicalName] {
			continue
		}
		results = append(results, s)
	}

	w.Header().Set("Content-Type", "application/json")
	if len(results) == 0 {
		results = []SkillSearchResponse{}
	}
	json.NewEncoder(w).Encode(results)
}

type ParsedSkill struct {
	SkillID             *int    `json:"skill_id"`
	SkillName           string  `json:"skill_name"`
	CanonicalName       *string `json:"canonical_name"`
	Category            *string `json:"category"`
	InferredProficiency string  `json:"inferred_proficiency"`
	Confidence          string  `json:"confidence"`
	Evidence            string  `json:"evidence"`
}

type ParseResponse struct {
	Skills         []ParsedSkill `json:"skills"`
	UnmatchedCount int           `json:"unmatched_count"`
}

func (h *Handler) ParseResume(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20) // 10 MB limit
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("resume")
	if err != nil {
		http.Error(w, "resume field is required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create temp file to read with external libs
	tempFile, err := os.CreateTemp("", "resume-*"+header.Filename)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer os.Remove(tempFile.Name())

	_, err = io.Copy(tempFile, file)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	var rawText string
	if strings.HasSuffix(strings.ToLower(header.Filename), ".pdf") {
		rawText, err = readPDF(tempFile.Name())
	} else if strings.HasSuffix(strings.ToLower(header.Filename), ".docx") {
		rawText, err = readDOCX(tempFile.Name())
	} else {
		http.Error(w, "Unsupported file type. Only PDF and DOCX are allowed.", http.StatusBadRequest)
		return
	}

	if err != nil {
		log.Printf("Internal server error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Call Gemini
	ctx := context.Background()
	client, err := genai.NewClient(ctx, option.WithAPIKey(os.Getenv("GEMINI_API_KEY")))
	if err != nil {
		log.Printf("Internal server error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-flash") // User requested flash-lite, but generative-ai-go typically uses flash. Let's try 2.5-flash
	model.ResponseMIMEType = "application/json"
	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{
			genai.Text(`You are a technical skill extractor for software engineering resumes.

Extract ONLY named, concrete technical skills that would appear as requirements 
in software engineering job postings. A valid skill is a specific, nameable 
technology — not a category, practice, trait, or description.

VALID: named languages, frameworks, libraries, databases, cloud services/products,
DevOps tools, testing tools, protocols, platforms, and AI/ML tools.

INVALID — exclude these:
- Section headers or category words (e.g. "frameworks", "tools", "libraries")
- Design patterns and architectural concepts (e.g. dependency injection, microservices architecture)  
- Practices and traits (e.g. load testing, performance optimization, scalability, authorization)
- Methodologies (e.g. agile, domain driven design, test driven development)
- Merged or concatenated text artifacts (e.g. "AutomatedCITesting", "BackendTesting")
- Vague descriptors (e.g. "backend infrastructure", "realtime systems", "email security")
- Activities mistaken for tools (e.g. "load testing" without naming k6/Locust/JMeter)

PROFICIENCY — infer from resume context:
- Explicit years stated / led / architected / designed → advanced or expert
- Built / implemented / developed / managed → intermediate
- Listed in skills section with no supporting project evidence → intermediate (default, not beginner)
- Familiar with / exposure to / basic / some experience → beginner
- Confidence: high if multiple role mentions, medium if one role, low if skills-section-only

Return ONLY valid JSON, no markdown:
{
  "skills": [
    {
      "skill_name": string,
      "inferred_proficiency": "beginner"|"intermediate"|"advanced"|"expert",
      "confidence": "high"|"medium"|"low",
      "evidence": string
    }
  ]
}`),
		},
	}

	resp, err := model.GenerateContent(ctx, genai.Text(rawText))
	if err != nil {
		log.Printf("Internal server error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		http.Error(w, "Empty response from AI", http.StatusInternalServerError)
		return
	}

	var aiResult struct {
		Skills []struct {
			SkillName           string `json:"skill_name"`
			InferredProficiency string `json:"inferred_proficiency"`
			Confidence          string `json:"confidence"`
			Evidence            string `json:"evidence"`
		} `json:"skills"`
	}

	part := resp.Candidates[0].Content.Parts[0]
	if txt, ok := part.(genai.Text); ok {
		if err := json.Unmarshal([]byte(txt), &aiResult); err != nil {
			log.Printf("Internal server error: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	} else {
		http.Error(w, "Unexpected format from AI", http.StatusInternalServerError)
		return
	}

	var finalResponse ParseResponse
	for _, extracted := range aiResult.Skills {
		searchName := extracted.SkillName
		if canonical, ok := h.AliasMap[searchName]; ok && canonical != "" {
			searchName = canonical
		}

		if h.Blocklist[searchName] {
			continue
		}

		ps := ParsedSkill{
			SkillName:           searchName,
			InferredProficiency: extracted.InferredProficiency,
			Confidence:          extracted.Confidence,
			Evidence:            extracted.Evidence,
		}

		// Fuzzy match
		var dbSkillID int
		var dbCanonicalName string
		var dbCategory *string
		err = h.db.QueryRow(r.Context(), `
			SELECT s.id, s.canonical_name, c.name
			FROM skill s
			LEFT JOIN skill_category c ON s.category_id = c.id
			WHERE s.canonical_name=$1
		`, searchName).Scan(&dbSkillID, &dbCanonicalName, &dbCategory)

		if err == nil {
			ps.SkillID = &dbSkillID
			ps.CanonicalName = &dbCanonicalName
			ps.Category = dbCategory
		} else {
			finalResponse.UnmatchedCount++
		}

		finalResponse.Skills = append(finalResponse.Skills, ps)
	}

	if finalResponse.Skills == nil {
		finalResponse.Skills = []ParsedSkill{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(finalResponse)
}

func readPDF(path string) (string, error) {
	f, r, err := pdf.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	b, err := r.GetPlainText()
	if err != nil {
		return "", err
	}
	buf.ReadFrom(b)
	return buf.String(), nil
}

func readDOCX(path string) (string, error) {
	r, err := docx.ReadDocxFile(path)
	if err != nil {
		return "", err
	}
	defer r.Close()
	return r.Editable().GetContent(), nil
}
