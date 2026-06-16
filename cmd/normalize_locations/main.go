package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

type LocationMap struct {
	Raw     string  `json:"raw"`
	Country *string `json:"country"`
	City    *string `json:"city"`
}

func main() {
	godotenv.Load()

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

	// Fetch distinct locations
	rows, err := conn.Query(ctx, "SELECT DISTINCT location_raw FROM job_posting WHERE location_raw IS NOT NULL AND location_country IS NULL")
	if err != nil {
		log.Fatalf("Failed to query locations: %v", err)
	}

	var locations []string
	for rows.Next() {
		var loc string
		if err := rows.Scan(&loc); err != nil {
			log.Printf("Scan error: %v", err)
			continue
		}
		locations = append(locations, loc)
	}
	rows.Close()

	if len(locations) == 0 {
		log.Println("No distinct locations found that need normalization.")
		return
	}

	log.Printf("Found %d distinct locations to normalize.", len(locations))

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY not set")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		log.Fatalf("Failed to create GenAI client: %v", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-2.5-flash-lite")
	model.ResponseMIMEType = "application/json"

	promptTemplate := `You are a geolocation normalizer. Given a list of freeform location strings, normalize each into structured country and city names.
Rules:
- Country: use full English name ("India", "United States", "United Kingdom").
- City: use most common English name ("Bangalore" not "Bengaluru", "New York" not "New York City").
- If no city determinable: null.
- If fully remote with no city: country = null, city = null.
- Return a JSON array matching the exact format: [{"raw": "...", "country": "...", "city": "..."}]
- Ensure the "raw" field matches the input exactly.

Input Locations:
%s`

	locationsJSON, _ := json.Marshal(locations)
	prompt := fmt.Sprintf(promptTemplate, string(locationsJSON))

	var mapping []LocationMap
	mapFile := "data/location_normalization_map.json"
	
	if fileBytes, err := os.ReadFile(mapFile); err == nil {
		log.Println("Found existing mapping file, bypassing Gemini...")
		if err := json.Unmarshal(fileBytes, &mapping); err != nil {
			log.Fatalf("Failed to parse existing map: %v", err)
		}
	} else {
		log.Println("Calling Gemini API...")
		reqCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
		defer cancel()
		
		resp, err := model.GenerateContent(reqCtx, genai.Text(prompt))
		if err != nil {
			log.Fatalf("Generate content failed: %v", err)
		}

		if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
			log.Fatal("Empty response candidates")
		}

		part := resp.Candidates[0].Content.Parts[0]
		textPart, ok := part.(genai.Text)
		if !ok {
			log.Fatal("No text part in response")
		}

		rawJSON := string(textPart)
		if err := json.Unmarshal([]byte(rawJSON), &mapping); err != nil {
			log.Fatalf("JSON parse failure. Raw: %s\nError: %v", rawJSON, err)
		}

		outBytes, _ := json.MarshalIndent(mapping, "", "  ")
		os.MkdirAll("data", 0755)
		if err := os.WriteFile(mapFile, outBytes, 0644); err != nil {
			log.Printf("Failed to write map to file: %v", err)
		}
		log.Printf("Saved %d mappings to %s", len(mapping), mapFile)
	}

	// Create map for easy lookup
	locMap := make(map[string]LocationMap)
	for _, m := range mapping {
		locMap[m.Raw] = m
	}

	log.Println("Updating database...")
	// Update the database
	updateRows, err := conn.Query(ctx, "SELECT id, location_raw FROM job_posting WHERE location_raw IS NOT NULL")
	if err != nil {
		log.Fatalf("Failed to select rows for update: %v", err)
	}

	type UpdateJob struct {
		ID  string
		Raw string
	}
	var toUpdate []UpdateJob
	for updateRows.Next() {
		var u UpdateJob
		if err := updateRows.Scan(&u.ID, &u.Raw); err != nil {
			log.Printf("Scan error for update: %v", err)
			continue
		}
		toUpdate = append(toUpdate, u)
	}
	updateRows.Close()

	updatedCount := 0
	for _, u := range toUpdate {
		lowerLoc := strings.ToLower(u.Raw)
		var workType string
		if strings.Contains(lowerLoc, "remote") {
			workType = "Remote"
		} else if strings.Contains(lowerLoc, "hybrid") {
			workType = "Hybrid"
		} else {
			workType = "Onsite"
		}

		m, ok := locMap[u.Raw]
		var country, city *string
		if ok {
			country = m.Country
			city = m.City
		}

		_, err := conn.Exec(ctx, "UPDATE job_posting SET location_country = $1, location_city = $2, work_type = $3 WHERE id = $4", country, city, workType, u.ID)
		if err != nil {
			log.Printf("Failed to update job ID %s: %v", u.ID, err)
		} else {
			updatedCount++
		}
	}

	log.Printf("Successfully updated %d job postings.", updatedCount)
}
