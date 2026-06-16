package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

var explicitPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(\d+)\+?\s*years?\s+of\s+(?:software|engineering|professional|industry|relevant)\s+experience`),
	regexp.MustCompile(`(?i)minimum\s+(?:of\s+)?(\d+)\s+years?`),
	regexp.MustCompile(`(?i)at\s+least\s+(\d+)\s+years?`),
	regexp.MustCompile(`(?i)(\d+)\s*[-–]\s*\d+\s+years?\s+of\s+experience`),
}

var skillPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(\d+)\+?\s*years?\s+of\s+(?:experience\s+(?:with|in|using)\s+)?[\w\s]+experience`),
	regexp.MustCompile(`(?i)(\d+)\+?\s*years?\s+(?:of\s+)?(?:hands[-\s]on|practical|working)\s+experience`),
	regexp.MustCompile(`(?i)(\d+)\+?\s*years?\s+(?:experience\s+)?(?:working\s+)?(?:with|in|on)\s+\w+`),
}

var seniorityMap = []struct {
	keywords []string
	minYears int
}{
	{[]string{"intern", "internship"}, 0},
	{[]string{"junior", "jr", "associate", "entry", "graduate", "new grad"}, 0},
	{[]string{"mid", "ii", "2"}, 2}, // "Engineer II", "SWE 2"
	{[]string{"senior", "sr", "iii", "3"}, 4},
	{[]string{"staff", "lead", "iv", "4"}, 7},
	{[]string{"principal", "distinguished", "v", "5"}, 10},
	{[]string{"director", "vp", "head of"}, 12},
}

type ExtractedRow struct {
	PostingID   string `json:"posting_id"`
	Source      string `json:"source"`
	PatternUsed int    `json:"pattern_used"`
	Extracted   int    `json:"extracted_years"`
	Snippet     string `json:"snippet"`
}

func main() {
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

	log.Println("Fetching jobs to process...")

	// Only process jobs that don't have explicit required years (or where inferred is missing)
	rows, err := conn.Query(ctx, `
		SELECT id, content, coalesce(raw_title, '')
		FROM job_posting 
		WHERE min_years_required IS NULL AND min_years_inferred IS NULL AND content IS NOT NULL
	`)
	if err != nil {
		log.Fatalf("Query failed: %v\n", err)
	}

	type jobRow struct {
		id      string
		content string
		title   string
	}

	var jobs []jobRow
	for rows.Next() {
		var j jobRow
		if err := rows.Scan(&j.id, &j.content, &j.title); err != nil {
			log.Fatalf("Scan failed: %v", err)
		}
		jobs = append(jobs, j)
	}
	rows.Close()

	totalProcessed := len(jobs)
	if totalProcessed == 0 {
		log.Println("No jobs to process.")
		return
	}

	stats := map[string]int{
		"regex_explicit":  0,
		"regex_skill":     0,
		"title_inference": 0,
		"null":            0,
	}

	var extractions []ExtractedRow
	log.Printf("Processing %d jobs...\n", totalProcessed)

	for i, j := range jobs {
		var matchedYears int = -1
		var source string
		var patternIdx int = -1
		var matchedStr string

		// 1. Try explicit patterns
		for idx, p := range explicitPatterns {
			matches := p.FindStringSubmatch(j.content)
			if len(matches) > 1 {
				if years, err := strconv.Atoi(matches[1]); err == nil {
					matchedYears = years
					patternIdx = idx + 1
					matchedStr = matches[0]
					source = "regex_explicit"
					break
				}
			}
		}

		// 2. Try skill-based patterns (take MAX if multiple found, or just first match)
		if source == "" {
			var maxSkillYears int = -1
			var bestMatchStr string
			var bestIdx int = -1
			for idx, p := range skillPatterns {
				matchesList := p.FindAllStringSubmatch(j.content, -1)
				for _, matches := range matchesList {
					if len(matches) > 1 {
						if years, err := strconv.Atoi(matches[1]); err == nil {
							if years > maxSkillYears && years <= 30 { // sanity check
								maxSkillYears = years
								bestIdx = idx + 1
								bestMatchStr = matches[0]
							}
						}
					}
				}
			}
			if maxSkillYears >= 0 {
				matchedYears = maxSkillYears
				source = "regex_skill"
				patternIdx = bestIdx
				matchedStr = bestMatchStr
			}
		}

		// 3. Try title inference
		if source == "" && j.title != "" {
			titleLower := strings.ToLower(j.title)
			for idx, sm := range seniorityMap {
				for _, kw := range sm.keywords {
					// Add word boundaries for robust match, though simple Contains is okay for now
					if strings.Contains(titleLower, kw) {
						matchedYears = sm.minYears
						source = "title_inference"
						patternIdx = idx + 1
						matchedStr = "Matched title keyword: " + kw
						break
					}
				}
				if source != "" {
					break
				}
			}
		}

		// Update DB based on source
		if source != "" {
			stats[source]++
			if source == "regex_explicit" {
				_, err = conn.Exec(ctx, "UPDATE job_posting SET min_years_required = $1, min_years_source = $2 WHERE id = $3", matchedYears, source, j.id)
			} else {
				_, err = conn.Exec(ctx, "UPDATE job_posting SET min_years_inferred = $1, min_years_source = $2 WHERE id = $3", matchedYears, source, j.id)
			}
			
			if err != nil {
				log.Printf("Failed to update job %s: %v", j.id, err)
			}

			snippet := matchedStr
			if len(snippet) > 200 {
				snippet = snippet[:200]
			}
			extractions = append(extractions, ExtractedRow{
				PostingID:   j.id,
				Source:      source,
				PatternUsed: patternIdx,
				Extracted:   matchedYears,
				Snippet:     snippet,
			})
		} else {
			stats["null"]++
			_, err = conn.Exec(ctx, "UPDATE job_posting SET min_years_source = 'null' WHERE id = $1", j.id)
			if err != nil {
				log.Printf("Failed to update job %s: %v", j.id, err)
			}
		}

		if (i+1)%1000 == 0 {
			log.Printf("Processed %d/%d...", i+1, totalProcessed)
		}
	}

	fmt.Printf("\n--- Backfill Summary ---\n")
	fmt.Printf("Total Processed: %d\n", totalProcessed)
	fmt.Printf("Explicit Regex:  %d\n", stats["regex_explicit"])
	fmt.Printf("Skill Regex:     %d\n", stats["regex_skill"])
	fmt.Printf("Title Inference: %d\n", stats["title_inference"])
	fmt.Printf("Still Null:      %d\n", stats["null"])

	os.MkdirAll("data/logs", 0755)
	logFilename := filepath.Join("data/logs", fmt.Sprintf("yoe_extractions_v2_%d.json", time.Now().Unix()))
	b, _ := json.MarshalIndent(extractions, "", "  ")
	err = os.WriteFile(logFilename, b, 0644)
	if err != nil {
		log.Printf("Failed to write extractions log: %v", err)
	} else {
		fmt.Printf("Extracted results logged to: %s\n", logFilename)
	}
}
