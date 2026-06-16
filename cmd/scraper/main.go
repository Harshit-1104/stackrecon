package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Company struct {
	Type     string   `json:"type"`
	ATSLinks []string `json:"ats_links"`
}

type JobResponse struct {
	Jobs []json.RawMessage `json:"jobs"`
}

type JobData struct {
	Title       string `json:"title"`
	Departments []struct {
		Name string `json:"name"`
	} `json:"departments"`
}

type LogEntry struct {
	RunAt                   string          `json:"run_at"`
	TotalCompaniesAttempted int             `json:"total_companies_attempted"`
	TotalCompaniesSucceeded int             `json:"total_companies_succeeded"`
	TotalCompaniesFailed    int             `json:"total_companies_failed"`
	TotalTechJDs            int             `json:"total_tech_jds"`
	TotalNonTechJDs         int             `json:"total_non_tech_jds"`
	FailedCompanies         []FailedCompany `json:"failed_companies"`
}

type FailedCompany struct {
	Slug   string `json:"slug"`
	Reason string `json:"reason"`
}

func main() {
	startTime := time.Now()

	// Step 1 - Filter companies and extract Greenhouse slugs
	resp, err := http.Get("https://raw.githubusercontent.com/outscal/OpenJobs/main/data/companies_v2.json")
	if err != nil {
		log.Fatalf("Failed to fetch companies: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Failed to read companies response: %v", err)
	}

	var companies []Company
	if err := json.Unmarshal(body, &companies); err != nil {
		log.Fatalf("Failed to unmarshal companies: %v", err)
	}

	var slugs []string
	slugSet := make(map[string]bool)
	matched := 0
	skipped := 0

	for _, c := range companies {
		if c.Type != "tech" {
			continue
		}

		hasGreenhouse := false
		for _, link := range c.ATSLinks {
			if strings.Contains(link, "greenhouse.io") {
				hasGreenhouse = true
				u, err := url.Parse(link)
				if err != nil {
					continue
				}
				path := strings.TrimRight(u.Path, "/")
				parts := strings.Split(path, "/")
				if len(parts) > 0 {
					slug := parts[len(parts)-1]
					if slug != "" {
						if !slugSet[slug] {
							slugSet[slug] = true
							slugs = append(slugs, slug)
						}
					}
				}
			}
		}

		if hasGreenhouse {
			matched++
		} else {
			skipped++
		}
	}

	fmt.Printf("Companies matched: %d\n", matched)
	fmt.Printf("Companies skipped: %d\n", skipped)

	// Step 2, 3, 4 - Fetch JDs from Greenhouse API, bifurcate, and store
	dirs := []string{
		"data/raw",
		"data/processed/tech",
		"data/processed/non_tech",
		"data/logs",
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Failed to create dir %s: %v", dir, err)
		}
	}

	client := &http.Client{
		Timeout: 15 * time.Second,
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	runLog := LogEntry{
		RunAt:           startTime.Format(time.RFC3339),
		FailedCompanies: []FailedCompany{},
	}

	for _, slug := range slugs {
		// Checkpoint: skip if already processed
		processedTechPath := filepath.Join("data", "processed", "tech", slug+".json")
		if _, err := os.Stat(processedTechPath); err == nil {
			log.Printf("[%s] already processed, skipping", slug)
			continue
		}

		<-ticker.C
		runLog.TotalCompaniesAttempted++

		jobsURL := fmt.Sprintf("https://boards-api.greenhouse.io/v1/boards/%s/jobs?content=true", slug)

		rawJobs, statusCode, err := fetchWithRetry(client, jobsURL)
		if statusCode == 404 {
			log.Printf("[%s] not_found", slug)
			runLog.TotalCompaniesFailed++
			runLog.FailedCompanies = append(runLog.FailedCompanies, FailedCompany{Slug: slug, Reason: "not_found (404)"})
			continue
		}
		if err != nil {
			log.Printf("[%s] failed: %v", slug, err)
			runLog.TotalCompaniesFailed++
			runLog.FailedCompanies = append(runLog.FailedCompanies, FailedCompany{Slug: slug, Reason: err.Error()})
			continue
		}

		// Save raw API response
		rawDir := filepath.Join("data", "raw", slug)
		os.MkdirAll(rawDir, 0755)
		if err := os.WriteFile(filepath.Join(rawDir, "jobs.json"), rawJobs, 0644); err != nil {
			log.Printf("[%s] failed to save raw: %v", slug, err)
			runLog.TotalCompaniesFailed++
			runLog.FailedCompanies = append(runLog.FailedCompanies, FailedCompany{Slug: slug, Reason: "failed to save raw"})
			continue
		}

		var jobResp JobResponse
		if err := json.Unmarshal(rawJobs, &jobResp); err != nil {
			log.Printf("[%s] failed to unmarshal jobs JSON: %v", slug, err)
			runLog.TotalCompaniesFailed++
			runLog.FailedCompanies = append(runLog.FailedCompanies, FailedCompany{Slug: slug, Reason: "invalid json"})
			continue
		}

		techJobs := []json.RawMessage{}
		nonTechJobs := []json.RawMessage{}

		for _, rawJob := range jobResp.Jobs {
			var jd JobData
			if err := json.Unmarshal(rawJob, &jd); err != nil {
				nonTechJobs = append(nonTechJobs, rawJob)
				runLog.TotalNonTechJDs++
				continue
			}

			if classifyJob(jd) == "tech" {
				techJobs = append(techJobs, rawJob)
				runLog.TotalTechJDs++
			} else {
				nonTechJobs = append(nonTechJobs, rawJob)
				runLog.TotalNonTechJDs++
			}
		}

		// Store bifurcated arrays only if there are items, to avoid writing 'null' if array is empty
		if len(techJobs) > 0 {
			b, _ := json.MarshalIndent(techJobs, "", "  ")
			os.WriteFile(filepath.Join("data", "processed", "tech", slug+".json"), b, 0644)
		} else {
			os.WriteFile(filepath.Join("data", "processed", "tech", slug+".json"), []byte("[]"), 0644)
		}

		if len(nonTechJobs) > 0 {
			b, _ := json.MarshalIndent(nonTechJobs, "", "  ")
			os.WriteFile(filepath.Join("data", "processed", "non_tech", slug+".json"), b, 0644)
		} else {
			os.WriteFile(filepath.Join("data", "processed", "non_tech", slug+".json"), []byte("[]"), 0644)
		}

		runLog.TotalCompaniesSucceeded++
		log.Printf("[%s] processing complete", slug)
	}

	// Step 5 - Stdout summary
	timestamp := startTime.Format("20060102_150405")
	logFilename := filepath.Join("data", "logs", fmt.Sprintf("scraper_%s.json", timestamp))
	logBytes, _ := json.MarshalIndent(runLog, "", "  ")
	os.WriteFile(logFilename, logBytes, 0644)

	fmt.Printf("\nCompanies attempted:   %d\n", runLog.TotalCompaniesAttempted)
	fmt.Printf("Companies succeeded:   %d\n", runLog.TotalCompaniesSucceeded)
	fmt.Printf("Companies failed:      %d\n", runLog.TotalCompaniesFailed)
	fmt.Printf("Total tech JDs:        %d\n", runLog.TotalTechJDs)
	fmt.Printf("Total non-tech JDs:    %d\n", runLog.TotalNonTechJDs)
	fmt.Printf("Log: %s\n", logFilename)
}

func fetchWithRetry(client *http.Client, url string) ([]byte, int, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, 0, err
	}

	resp, err := client.Do(req)
	if err == nil {
		if resp.StatusCode == 404 {
			resp.Body.Close()
			return nil, 404, fmt.Errorf("not found")
		}
		if resp.StatusCode == 200 {
			b, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			return b, 200, err
		}
		resp.Body.Close()
	}

	// Retry once after 3s
	time.Sleep(3 * time.Second)
	resp2, err2 := client.Do(req)
	if err2 != nil {
		return nil, 0, err2
	}
	defer resp2.Body.Close()

	if resp2.StatusCode == 404 {
		return nil, 404, fmt.Errorf("not found")
	}
	if resp2.StatusCode == 200 {
		b, err := io.ReadAll(resp2.Body)
		return b, 200, err
	}
	return nil, resp2.StatusCode, fmt.Errorf("status code %d", resp2.StatusCode)
}

func classifyJob(j JobData) string {
	var depts []string
	for _, d := range j.Departments {
		depts = append(depts, strings.ToLower(d.Name))
	}
	deptStr := strings.Join(depts, " ")

	// ---------------------------------------------------------------
	// Stage 1 — NON-TECH department hard exclusion
	// NOTE: multi-word phrases listed FIRST to reduce substring false
	// positives (e.g. "sales engineering" must not hit "sales" before
	// checking context). True fix requires word-boundary matching.
	// ---------------------------------------------------------------
	nonTechDeptKeywords := []string{
		// People / HR
		"human resources", "people operations", "people ops", "people experience",
		"talent acquisition", "talent management", "talent development",
		"learning and development", "l&d", "organizational development",
		"employee experience", "diversity, equity", "diversity and inclusion",
		"dei", "compensation and benefits", "compensation", "benefits",
		"payroll", "workforce management", "workforce planning",
		"hr business partner", "hrbp", "hr",

		// Revenue / Sales
		"revenue operations", "sales operations", "sales enablement",
		"sales development", "field sales", "inside sales",
		"go-to-market", "gtm", "account management", "business development",
		"partnerships", "alliances", "channels", "revenue", "sales",

		// Marketing / Brand / Content
		"product marketing", // NOTE: intentionally non-tech; product *engineering* will match "engineering"
		"growth marketing",  // growth engineering depts typically include "engineering"
		"field marketing", "performance marketing", "demand generation", "demand gen",
		"digital marketing", "content marketing", "brand marketing",
		"marketing", "brand", "content", "editorial", "creative services",
		"social media", "communications", "public relations", "corporate communications",
		"events", "community", "influencer", "media relations",

		// Finance / Legal / Compliance
		"investor relations", "corporate development", "corporate strategy",
		"treasury", "tax", "audit", "accounting", "finance",
		"compliance", "legal", "risk management", "risk", "insurance",
		"procurement", "purchasing",

		// Ops / Admin / Facilities
		"business operations", "office management", "real estate",
		"facilities", "supply chain", "logistics", "administrative",
		"government affairs", "regulatory affairs", "policy",
		"sustainability", "esg", "philanthropy", "corporate affairs",
		"executive", "operations", "strategy",

		// Customer-facing (non-eng)
		"customer success", "customer support", "customer experience",
		"customer service", "cx", "field operations",
		"professional services", // remove if PS = solutions engineers in your corpus
		"implementation",        // borderline; remove if impl = technical implementation engineers
		"technical support",     // borderline; remove if you want support engineers as tech
		"onboarding",

		// Recruiting
		"recruiting",

		// Product
		"product",
	}
	for _, kw := range nonTechDeptKeywords {
		if strings.Contains(deptStr, kw) {
			return "non_tech"
		}
	}

	// ---------------------------------------------------------------
	// Stage 2 — TECH department inclusion
	// ---------------------------------------------------------------
	techDeptKeywords := []string{
		// Core engineering
		"software engineering", "application development",
		"engineering", "software", "backend", "frontend", "mobile",
		"fullstack", "full-stack", "full stack", "web development",
		"firmware", "embedded systems", "embedded",

		// Infra / Platform / Ops (tech)
		"site reliability", "developer experience", "developer productivity",
		"production engineering", "release engineering", "build and release",
		"technical operations", "techops", "devops", "devsecops",
		"infrastructure", "platform", "cloud", "systems", "network",
		"database", "information technology", "it operations",

		// Security
		"cybersecurity", "application security", "appsec", "infosec",
		"trust and safety", "security",

		// Data / AI / ML
		"machine learning", "artificial intelligence", "deep learning",
		"natural language processing", "nlp", "computer vision",
		"applied science", "applied research", "data science",
		"data engineering", "analytics engineering", "mlops",
		"analytics", "data", "ai", "ml", "research",

		// QA / Testing
		"quality assurance", "quality engineering", "test automation",
		"testing", "qa",

		// Specialized tech
		"developer relations", "devrel",
		"solutions engineering", // NOTE: straddles sales-eng boundary
		"hardware engineering", "hardware",
		"robotics", "automation engineering",
		"iot", "internet of things",
		"blockchain", "web3",
		"game development", "simulation",
		"architecture", "technical",
	}
	for _, kw := range techDeptKeywords {
		if strings.Contains(deptStr, kw) {
			return "tech"
		}
	}

	// ---------------------------------------------------------------
	// Stage 3 — Title keyword fallback
	// ---------------------------------------------------------------
	title := strings.ToLower(j.Title)

	techTitleKeywords := []string{
		// Core roles
		"software engineer", "software developer",
		"engineer", "developer", "programmer", "architect",
		"scientist", "researcher",

		// Seniority/scope signals (only useful combined with tech context above)
		// Not added standalone — "staff" or "principal" alone isn't enough

		// Specializations
		"backend", "frontend", "front-end", "back-end",
		"fullstack", "full-stack", "full stack",
		"mobile", "ios", "android",
		"web developer", "web engineer",
		"firmware", "embedded",

		// Infra / Ops
		"devops", "devsecops", "sre", "site reliability",
		"platform engineer", "platform",
		"infrastructure", "cloud engineer", "cloud architect", "cloud",
		"systems engineer", "systems architect", "systems",
		"network engineer", "network",
		"database administrator", "database engineer", "database",
		"sysadmin", "systems administrator",
		"production engineer", "release engineer", "build engineer",
		"techops",

		// Security
		"security engineer", "security architect", "security analyst",
		"cybersecurity", "appsec", "infosec", "penetration tester",
		"pentester", "devsecops",

		// Data / AI / ML
		"data engineer", "data scientist", "data analyst",
		"machine learning", "ml engineer", "mlops",
		"ai engineer", "artificial intelligence",
		"deep learning", "nlp engineer", "llm engineer",
		"computer vision", "generative ai", "prompt engineer",
		"applied scientist", "research scientist", "research engineer",

		// QA
		"qa engineer", "qa analyst", "quality engineer",
		"test engineer", "tester", "automation engineer", "automation",

		// Specialized
		"solutions engineer", // NOTE: also used as a sales-eng hybrid at some cos
		"hardware engineer", "hardware",
		"robotics engineer", "robotics",
		"blockchain developer", "blockchain",
		"smart contract", "web3",
		"game developer", "game engineer",
		"iot engineer", "iot",
		"founding engineer",
		"forward deployed engineer",
		"developer advocate", "developer relations",
		"technical lead", "tech lead",
		"technical writer", // borderline; remove if you want these as non-tech
		"accessibility engineer",
		"programmer analyst",
		"coder",
	}
	for _, kw := range techTitleKeywords {
		if strings.Contains(title, kw) {
			return "tech"
		}
	}

	nonTechTitleKeywords := []string{
		// Sales
		"account executive", "account manager", "sales development",
		"business development", "sales engineer", // NOTE: pure sales-side SE
		"partnership manager", "channel manager",
		"revenue manager", "renewal manager",
		"sales",

		// Marketing
		"marketing manager", "marketing analyst", "marketing specialist",
		"growth hacker", "seo manager", "seo specialist",
		"content strategist", "content manager", "content writer",
		"copywriter", "social media manager", "social media",
		"brand manager", "brand strategist",
		"communications manager", "pr manager", "publicist",
		"demand generation", "field marketing",
		"marketing",

		// HR / Recruiting
		"hr business partner", "hrbp", "people partner",
		"hr generalist", "hr specialist", "hr manager",
		"talent acquisition", "recruiter", "sourcer",
		"recruiting coordinator", "talent",

		// Finance / Legal
		"financial analyst", "finance manager",
		"accountant", "controller", "auditor", "treasurer",
		"tax specialist", "compliance officer", "risk analyst",
		"legal counsel", "paralegal", "attorney", "general counsel",
		"procurement", "buyer",

		// Creative / Design (non-UX)
		// NOTE: "designer" alone excluded — catches UX/product designers
		// Add specific non-UX ones:
		"graphic designer", "visual designer", "brand designer",
		"illustrator", "art director", "creative director",
		"photographer", "videographer",
		"copywriter",

		// Ops / Admin
		"chief of staff", // borderline; often ops, rarely tech
		"executive assistant",
		"office manager", "facilities manager",
		"operations manager", "business operations",
		"coordinator", "administrator",
		"procurement manager", "supply chain",
		"logistics manager",

		// Customer-facing
		"customer success manager", "customer success",
		"customer support", "customer service",
		"implementation manager", // keep if impl = project mgmt, remove if impl = eng
		"onboarding specialist",
		"technical account manager", // NOTE: TAM straddles tech/non-tech; remove if TAMs code

		// Product
		"product",
	}
	for _, kw := range nonTechTitleKeywords {
		if strings.Contains(title, kw) {
			return "non_tech"
		}
	}

	return "non_tech"
}
