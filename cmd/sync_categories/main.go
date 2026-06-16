package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(".env"); err != nil {
		log.Println("No .env file found, relying on environment variables")
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

	// Read skill_categories.json
	data, err := os.ReadFile("data/skill_categories.json")
	if err != nil {
		log.Fatalf("Error reading skill_categories.json: %v", err)
	}

	var catMap map[string][]string
	if err := json.Unmarshal(data, &catMap); err != nil {
		log.Fatalf("Error parsing JSON: %v", err)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		log.Fatalf("Error starting transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	categoriesUpserted := 0
	skillsUpserted := 0

	for catName, skills := range catMap {
		var catID int
		err := tx.QueryRow(ctx, `
			INSERT INTO skill_category (name)
			VALUES ($1)
			ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
			RETURNING id`, catName).Scan(&catID)
		if err != nil {
			log.Fatalf("Error upserting category %s: %v", catName, err)
		}
		categoriesUpserted++

		for _, skillName := range skills {
			_, err = tx.Exec(ctx, `
				INSERT INTO skill (canonical_name, category_id)
				VALUES ($1, $2)
				ON CONFLICT (canonical_name) DO UPDATE SET category_id = EXCLUDED.category_id
			`, skillName, catID)
			if err != nil {
				log.Fatalf("Error upserting skill %s: %v", skillName, err)
			}
			skillsUpserted++
		}
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("Error committing transaction: %v", err)
	}

	fmt.Printf("Successfully synced %d categories and %d skills to the database.\n", categoriesUpserted, skillsUpserted)
}
