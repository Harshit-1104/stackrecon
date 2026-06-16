package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5"
)

type Location struct {
	Name string `json:"name"`
}

type Office struct {
	Name     string `json:"name"`
	Location string `json:"location"`
}

type Job struct {
	ID       int64     `json:"id"`
	Location Location  `json:"location"`
	Offices  []Office  `json:"offices"`
}

type JobBoard struct {
	Jobs []Job `json:"jobs"`
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

	log.Println("Starting location backfill from raw JSON files...")

	totalUpdated := 0
	totalSkipped := 0

	err = filepath.Walk("data/raw", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), "jobs.json") {
			b, err := os.ReadFile(path)
			if err != nil {
				log.Printf("Failed to read file %s: %v", path, err)
				return nil
			}

			var board JobBoard
			if err := json.Unmarshal(b, &board); err != nil {
				log.Printf("Failed to parse JSON in %s: %v", path, err)
				return nil
			}

			for _, job := range board.Jobs {
				// Determine location string
				rawLoc := job.Location.Name
				if rawLoc == "" && len(job.Offices) > 0 {
					rawLoc = job.Offices[0].Name
					if rawLoc == "" {
						rawLoc = job.Offices[0].Location
					}
				}

				// Check for "remote" (case insensitive) in both location and offices
				isRemote := strings.Contains(strings.ToLower(rawLoc), "remote")
				if !isRemote {
					for _, office := range job.Offices {
						if strings.Contains(strings.ToLower(office.Name), "remote") || strings.Contains(strings.ToLower(office.Location), "remote") {
							isRemote = true
							break
						}
					}
				}

				var finalLoc *string
				if isRemote {
					r := "Remote"
					finalLoc = &r
				} else if rawLoc != "" {
					finalLoc = &rawLoc
				} else {
					finalLoc = nil
				}

				jobIDStr := fmt.Sprintf("%d", job.ID)
				likePattern := "%/jobs/" + jobIDStr

				cmd, err := conn.Exec(ctx, "UPDATE job_posting SET location = $1 WHERE apply_url LIKE $2", finalLoc, likePattern)
				if err != nil {
					log.Printf("Failed to update location for job ID %d: %v", job.ID, err)
					totalSkipped++
					continue
				}

				if cmd.RowsAffected() > 0 {
					totalUpdated++
				} else {
					totalSkipped++
				}
			}
		}
		return nil
	})

	if err != nil {
		log.Fatalf("Error walking files: %v", err)
	}

	fmt.Printf("\n--- Location Backfill Summary ---\n")
	fmt.Printf("Total Updated: %d\n", totalUpdated)
	fmt.Printf("Total Skipped (Not found in DB): %d\n", totalSkipped)
}
