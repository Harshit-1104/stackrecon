package main

import (
	"context"
	"encoding/json"
	"fmt"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

type ExtractedJob struct {
	PostingID        string `json:"posting_id"`
	CompanyName      string `json:"company_name"`
	RawTitle         string `json:"raw_title"`
	ApplyURL         string `json:"apply_url"`
	PostedAt         string `json:"posted_at"`
	LastSeenAt       string `json:"last_seen_at"`
	CleanedContent   string `json:"cleaned_content"`
	ExtractionStatus string `json:"extraction_status"`
	MinYearsRequired *int   `json:"min_years_required"`
	Skills           []struct {
		Name             string `json:"name"`
		RequirementLevel string `json:"requirement_level"`
	} `json:"skills"`
}

type CompanyV2 struct {
	Name    string `json:"name"`
	Website string `json:"website"`
}

type LogFile struct {
	RunAt                     string   `json:"run_at"`
	ActiveThresholdDays       int      `json:"active_threshold_days"`
	TotalCompaniesProcessed   int      `json:"total_companies_processed"`
	TotalJobsInserted         int      `json:"total_jobs_inserted"`
	TotalJobsUpdated          int      `json:"total_jobs_updated"`
	TotalJobsSkipped          int      `json:"total_jobs_skipped"`
	TotalSkillsUpserted       int      `json:"total_skills_upserted"`
	TotalSkillsBlocklisted    int      `json:"total_skills_blocklisted"`
	TotalSkillSignalsComputed int      `json:"total_skill_signals_computed"`
	Errors                    []string `json:"errors"`
}

func main() {
	mode := flag.String("mode", "default", "Mode to run in: default or reextract")
	inputDir := flag.String("input", "data/extracted/tech", "Input directory for files")
	flag.Parse()

	startTime := time.Now()
	var logData LogFile
	logData.RunAt = startTime.Format(time.RFC3339)

	thresholdStr := os.Getenv("ACTIVE_THRESH OLD_DAYS")
	if thresholdStr == "" {
		thresholdStr = "30"
	}
	var err error
	logData.ActiveThresholdDays = 30
	fmt.Sscanf(thresholdStr, "%d", &logData.ActiveThresholdDays)

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

	// --- Step 0: Load config files ---
	aliasMap := loadAliasMap("data/alias_map.json", &logData)
	blocklist := loadBlocklist("data/skill_blocklist_resolved.json", &logData)
	skillToCategory := loadSkillCategories("data/skill_categories.json", &logData)
	companiesMap := loadCompanies("data/companies_v2.json", &logData)

	// --- Step 1: Seed SkillCategory ---
	categoryIDMap := make(map[string]int)
	uniqueCategories := make(map[string]bool)
	for _, cat := range skillToCategory {
		uniqueCategories[cat] = true
	}

	for catName := range uniqueCategories {
		var id int
		err := conn.QueryRow(ctx, `
			INSERT INTO skill_category (name)
			VALUES ($1)
			ON CONFLICT (name) DO NOTHING
			RETURNING id`, catName).Scan(&id)
		if err == pgx.ErrNoRows {
			conn.QueryRow(ctx, `SELECT id FROM skill_category WHERE name = $1`, catName).Scan(&id)
		} else if err != nil {
			logData.Errors = append(logData.Errors, fmt.Sprintf("Error seeding category %s: %v", catName, err))
			continue
		}
		categoryIDMap[catName] = id
	}

	// --- Step 2: Process extracted files ---
	var globPath string
	if *mode == "reextract" {
		globPath = filepath.Join(*inputDir, "*/*.json") // data/reextracted/{slug}/{job_id}.json
	} else {
		globPath = filepath.Join(*inputDir, "*.json")
	}

	files, err := filepath.Glob(globPath)
	if err != nil {
		log.Fatalf("Error globbing extracted files: %v\n", err)
	}

	reextractedCompanyIDs := make(map[int]bool)

	for _, file := range files {
		slug := strings.TrimSuffix(filepath.Base(file), ".json")
		
		data, err := os.ReadFile(file)
		if err != nil {
			logData.Errors = append(logData.Errors, fmt.Sprintf("Error reading file %s: %v", file, err))
			continue
		}

		var jobs []ExtractedJob
		if *mode == "reextract" {
			type Skill struct {
				Name             string `json:"skill_name"`
				RequirementLevel string `json:"requirement_level"`
				ExtractedBy      string `json:"extracted_by"`
			}
			var singleJob struct {
				JobID            string  `json:"job_id"`
				Slug             string  `json:"slug"`
				IsTech           bool    `json:"is_tech"`
				MinYearsRequired *int    `json:"min_years_required"`
				Skills           []Skill `json:"skills"`
			}
			if err := json.Unmarshal(data, &singleJob); err != nil {
				logData.Errors = append(logData.Errors, fmt.Sprintf("Error unmarshaling file %s: %v", file, err))
				continue
			}
			
			// Map to ExtractedJob
			var mappedSkills []struct{Name string `json:"name"`; RequirementLevel string `json:"requirement_level"`}
			for _, s := range singleJob.Skills {
				mappedSkills = append(mappedSkills, struct{Name string `json:"name"`; RequirementLevel string `json:"requirement_level"`}{Name: s.Name, RequirementLevel: s.RequirementLevel})
			}
			jobs = []ExtractedJob{{
				PostingID:        singleJob.JobID,
				MinYearsRequired: singleJob.MinYearsRequired,
				Skills:           mappedSkills,
			}}
		} else {
			if err := json.Unmarshal(data, &jobs); err != nil {
				logData.Errors = append(logData.Errors, fmt.Sprintf("Error unmarshaling file %s: %v", file, err))
				continue
			}
		}

		if len(jobs) == 0 {
			continue
		}

		if *mode == "reextract" {
			// For reextract mode, skip company/source upserts and job upserts
			// We only delete old skills, insert new ones, and update min_years_required
			for _, job := range jobs {
				_, err = conn.Exec(ctx, "DELETE FROM job_posting_skill WHERE posting_id = $1", job.PostingID)
				if err != nil {
					logData.Errors = append(logData.Errors, fmt.Sprintf("Error deleting old skills %s: %v", job.PostingID, err))
					continue
				}

				if job.MinYearsRequired != nil {
					_, err = conn.Exec(ctx, "UPDATE job_posting SET min_years_required = COALESCE($1, min_years_required) WHERE id = $2", *job.MinYearsRequired, job.PostingID)
					if err != nil {
						logData.Errors = append(logData.Errors, fmt.Sprintf("Error updating min years %s: %v", job.PostingID, err))
					}
				}

				for _, skill := range job.Skills {
					canonical := aliasMap[skill.Name]
					if canonical == "" {
						canonical = skill.Name
					}

					if blocklist[canonical] {
						logData.TotalSkillsBlocklisted++
						continue
					}

					catName := skillToCategory[canonical]
					var catID *int
					if id, ok := categoryIDMap[catName]; ok {
						catID = &id
					}

					var skillID int
					err = conn.QueryRow(ctx, `
						INSERT INTO skill (canonical_name, category_id)
						VALUES ($1, $2)
						ON CONFLICT (canonical_name) DO NOTHING
						RETURNING id`, canonical, catID).Scan(&skillID)
					if err == pgx.ErrNoRows {
						err = conn.QueryRow(ctx, `SELECT id FROM skill WHERE canonical_name = $1`, canonical).Scan(&skillID)
					}
					if err != nil {
						logData.Errors = append(logData.Errors, fmt.Sprintf("Error upserting skill %s: %v", canonical, err))
						continue
					}
					logData.TotalSkillsUpserted++

					reqLvl := strings.ToLower(skill.RequirementLevel)
					if reqLvl != "required" && reqLvl != "preferred" {
						reqLvl = "mentioned"
					}

					_, err = conn.Exec(ctx, `
						INSERT INTO job_posting_skill (posting_id, skill_id, requirement_level, extracted_by)
						VALUES ($1, $2, $3, 'llm')
						ON CONFLICT (posting_id, skill_id) DO NOTHING`,
						job.PostingID, skillID, reqLvl)
					if err != nil {
						logData.Errors = append(logData.Errors, fmt.Sprintf("Error upserting job_posting_skill %s-%d: %v", job.PostingID, skillID, err))
					}
				}
				
				// Get company ID to recompute later
				var compID int
				conn.QueryRow(ctx, "SELECT company_id FROM job_posting WHERE id = $1", job.PostingID).Scan(&compID)
				if compID > 0 {
					reextractedCompanyIDs[compID] = true
				}
			}
		} else {
			logData.TotalCompaniesProcessed++

			// 2a - Upsert Company
			companyName := jobs[0].CompanyName
			if companyName == "" {
				companyName = slug // fallback
			}
			website := companiesMap[companyName]
			
			var companyID int
			err = conn.QueryRow(ctx, `
				INSERT INTO company (name, website)
				VALUES ($1, $2)
				ON CONFLICT (name) DO UPDATE SET website = EXCLUDED.website
				RETURNING id`, companyName, website).Scan(&companyID)
			if err != nil {
				logData.Errors = append(logData.Errors, fmt.Sprintf("Error upserting company %s: %v", companyName, err))
				continue
			}

			// 2b - Upsert CompanySource
			var sourceID int
			err = conn.QueryRow(ctx, `
				INSERT INTO company_source (company_id, source_type, source_identifier, is_primary, last_scraped_at, active)
				VALUES ($1, 'greenhouse', $2, true, NOW(), true)
				ON CONFLICT (company_id, source_type, source_identifier)
				DO UPDATE SET last_scraped_at = NOW()
				RETURNING id`, companyID, slug).Scan(&sourceID)
			if err != nil {
				logData.Errors = append(logData.Errors, fmt.Sprintf("Error upserting source %s: %v", slug, err))
				continue
			}

			// 2c - Process jobs
			for _, job := range jobs {
				// Parse last seen
				lastSeen, _ := time.Parse(time.RFC3339, job.LastSeenAt)
				active := time.Since(lastSeen) <= time.Duration(logData.ActiveThresholdDays)*24*time.Hour
				
				var postedAt *time.Time
				if t, err := time.Parse(time.RFC3339, job.PostedAt); err == nil {
					postedAt = &t
				}

				// limit lengths to avoid schema errors
				rawTitle := job.RawTitle
				if len(rawTitle) > 500 {
					rawTitle = rawTitle[:500]
				}
				applyUrl := job.ApplyURL
				if len(applyUrl) > 1000 {
					applyUrl = applyUrl[:1000]
				}

				var xmax int
				err = conn.QueryRow(ctx, `
					INSERT INTO job_posting (
						id, company_id, company_source_id, raw_title, content, apply_url, posted_at, last_seen_at, active, role_type_id, min_years_required
					) VALUES (
						$1, $2, $3, $4, $5, $6, $7, $8, $9, NULL, $10
					)
					ON CONFLICT (id) DO UPDATE SET
						last_seen_at = EXCLUDED.last_seen_at,
						active = EXCLUDED.active,
						min_years_required = COALESCE(EXCLUDED.min_years_required, job_posting.min_years_required)
					RETURNING xmax::text::int`,
					job.PostingID, companyID, sourceID, rawTitle, job.CleanedContent, applyUrl, postedAt, lastSeen, active, job.MinYearsRequired,
				).Scan(&xmax)
				
				if err != nil {
					logData.Errors = append(logData.Errors, fmt.Sprintf("Error upserting job %s: %v", job.PostingID, err))
					continue
				}

				if xmax == 0 {
					logData.TotalJobsInserted++
				} else {
					logData.TotalJobsUpdated++
				}

				if job.ExtractionStatus != "success" && job.ExtractionStatus != "" {
					// In reextract format, ExtractionStatus might not be present or may be empty, handle it
					logData.TotalJobsSkipped++
					continue
				}

				for _, skill := range job.Skills {
					canonical := aliasMap[skill.Name]
					if canonical == "" {
						canonical = skill.Name
					}

					if blocklist[canonical] {
						logData.TotalSkillsBlocklisted++
						continue
					}

					catName := skillToCategory[canonical]
					var catID *int
					if id, ok := categoryIDMap[catName]; ok {
						catID = &id
					}

					var skillID int
					err = conn.QueryRow(ctx, `
						INSERT INTO skill (canonical_name, category_id)
						VALUES ($1, $2)
						ON CONFLICT (canonical_name) DO NOTHING
						RETURNING id`, canonical, catID).Scan(&skillID)
					if err == pgx.ErrNoRows {
						err = conn.QueryRow(ctx, `SELECT id FROM skill WHERE canonical_name = $1`, canonical).Scan(&skillID)
					}
					if err != nil {
						logData.Errors = append(logData.Errors, fmt.Sprintf("Error upserting skill %s: %v", canonical, err))
						continue
					}
					logData.TotalSkillsUpserted++

					reqLvl := strings.ToLower(skill.RequirementLevel)
					if reqLvl != "required" && reqLvl != "preferred" {
						reqLvl = "mentioned"
					}

					_, err = conn.Exec(ctx, `
						INSERT INTO job_posting_skill (posting_id, skill_id, requirement_level, extracted_by)
						VALUES ($1, $2, $3, 'llm')
						ON CONFLICT (posting_id, skill_id) DO NOTHING`,
						job.PostingID, skillID, reqLvl)
					if err != nil {
						logData.Errors = append(logData.Errors, fmt.Sprintf("Error upserting job_posting_skill %s-%d: %v", job.PostingID, skillID, err))
					}
				}
			}
		}
	}

	// --- Step 3: Compute CompanySkillSignal ---
	var whereClause string
	if *mode == "reextract" && len(reextractedCompanyIDs) > 0 {
		var ids []string
		for id := range reextractedCompanyIDs {
			ids = append(ids, fmt.Sprintf("%d", id))
		}
		whereClause = fmt.Sprintf("WHERE jp.company_id IN (%s)", strings.Join(ids, ","))
	}

	res, err := conn.Exec(ctx, fmt.Sprintf(`
		INSERT INTO company_skill_signal (
			company_id, skill_id, jd_count, jd_expired_count, github_count, blog_count, active, last_computed_at
		)
		SELECT
			jp.company_id,
			jps.skill_id,
			COUNT(CASE WHEN jp.active THEN 1 END) as jd_count,
			COUNT(CASE WHEN NOT jp.active THEN 1 END) as jd_expired_count,
			0, 0,
			(COUNT(CASE WHEN jp.active THEN 1 END) > 0) as active,
			NOW()
		FROM job_posting jp
		JOIN job_posting_skill jps ON jp.id = jps.posting_id
		%s
		GROUP BY jp.company_id, jps.skill_id
		ON CONFLICT (company_id, skill_id) DO UPDATE SET
			jd_count = EXCLUDED.jd_count,
			jd_expired_count = EXCLUDED.jd_expired_count,
			active = EXCLUDED.active,
			last_computed_at = NOW()
	`, whereClause))
	if err != nil {
		logData.Errors = append(logData.Errors, fmt.Sprintf("Error computing signals: %v", err))
	} else {
		logData.TotalSkillSignalsComputed = int(res.RowsAffected())
	}

	// --- Step 4 & 5: Output ---
	os.MkdirAll("data/logs", 0755)
	logFilename := fmt.Sprintf("data/logs/pipeline3_%d.json", time.Now().Unix())
	b, _ := json.MarshalIndent(logData, "", "  ")
	os.WriteFile(logFilename, b, 0644)

	fmt.Printf("Companies processed:       %d\n", logData.TotalCompaniesProcessed)
	fmt.Printf("Jobs inserted:             %d\n", logData.TotalJobsInserted)
	fmt.Printf("Jobs updated:              %d\n", logData.TotalJobsUpdated)
	fmt.Printf("Jobs skipped:              %d\n", logData.TotalJobsSkipped)
	fmt.Printf("Skills upserted:           %d\n", logData.TotalSkillsUpserted)
	fmt.Printf("Skills blocklisted:        %d\n", logData.TotalSkillsBlocklisted)
	fmt.Printf("Skill signals computed:    %d\n", logData.TotalSkillSignalsComputed)
	fmt.Printf("Log: %s\n", logFilename)
}

func loadAliasMap(path string, logData *LogFile) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		logData.Errors = append(logData.Errors, fmt.Sprintf("Error reading %s: %v", path, err))
		return make(map[string]string)
	}
	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		logData.Errors = append(logData.Errors, fmt.Sprintf("Error parsing %s: %v", path, err))
		return make(map[string]string)
	}
	return m
}

func loadBlocklist(path string, logData *LogFile) map[string]bool {
	data, err := os.ReadFile(path)
	if err != nil {
		logData.Errors = append(logData.Errors, fmt.Sprintf("Error reading %s: %v", path, err))
		return make(map[string]bool)
	}
	var list []string
	if err := json.Unmarshal(data, &list); err != nil {
		logData.Errors = append(logData.Errors, fmt.Sprintf("Error parsing %s: %v", path, err))
		return make(map[string]bool)
	}
	m := make(map[string]bool)
	for _, item := range list {
		m[item] = true
	}
	return m
}

func loadSkillCategories(path string, logData *LogFile) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		logData.Errors = append(logData.Errors, fmt.Sprintf("Error reading %s: %v", path, err))
		return make(map[string]string)
	}
	var cmap map[string][]string
	if err := json.Unmarshal(data, &cmap); err != nil {
		logData.Errors = append(logData.Errors, fmt.Sprintf("Error parsing %s: %v", path, err))
		return make(map[string]string)
	}
	m := make(map[string]string)
	for cat, skills := range cmap {
		for _, s := range skills {
			m[s] = cat
		}
	}
	return m
}

func loadCompanies(path string, logData *LogFile) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		logData.Errors = append(logData.Errors, fmt.Sprintf("Error reading %s: %v", path, err))
		return make(map[string]string)
	}
	var list []CompanyV2
	if err := json.Unmarshal(data, &list); err != nil {
		logData.Errors = append(logData.Errors, fmt.Sprintf("Error parsing %s: %v", path, err))
		return make(map[string]string)
	}
	m := make(map[string]string)
	for _, c := range list {
		if c.Name != "" {
			m[c.Name] = c.Website
		}
	}
	return m
}
