package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/joho/godotenv"
)

func main() {
	blocklistPath := flag.String("blocklist", "data/skill_blocklist_resolved.json", "Path to blocklist JSON file")
	aliasMapPath := flag.String("alias-map", "data/alias_map.json", "Path to alias map JSON file")
	flag.Parse()

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

	// Load alias map
	aliasData, err := os.ReadFile(*aliasMapPath)
	if err != nil {
		log.Fatalf("Error reading alias map: %v", err)
	}
	var aliasMap map[string]string
	if err := json.Unmarshal(aliasData, &aliasMap); err != nil {
		log.Fatalf("Error parsing alias map: %v", err)
	}

	// Load blocklist
	blocklistData, err := os.ReadFile(*blocklistPath)
	if err != nil {
		log.Fatalf("Error reading blocklist: %v", err)
	}
	var blocklist []string
	if err := json.Unmarshal(blocklistData, &blocklist); err != nil {
		log.Fatalf("Error parsing blocklist: %v", err)
	}

	// Resolve to canonical names
	canonicalBlocklist := make(map[string]bool)
	for _, item := range blocklist {
		canonical := item
		if val, ok := aliasMap[item]; ok && val != "" {
			canonical = val
		}
		canonicalBlocklist[canonical] = true
	}

	var targets []string
	for k := range canonicalBlocklist {
		targets = append(targets, k)
	}

	if len(targets) == 0 {
		fmt.Println("Blocklist is empty. Nothing to purge.")
		return
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		log.Fatalf("Error starting transaction: %v", err)
	}
	defer tx.Rollback(ctx)

	// We need to fetch skill IDs to delete from association tables
	rows, err := tx.Query(ctx, "SELECT id, canonical_name FROM skill WHERE canonical_name = ANY($1)", targets)
	if err != nil {
		log.Fatalf("Error fetching skill IDs: %v", err)
	}

	var skillIDs []int
	var foundNames []string
	for rows.Next() {
		var id int
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			log.Fatalf("Error scanning skill row: %v", err)
		}
		skillIDs = append(skillIDs, id)
		foundNames = append(foundNames, name)
	}
	rows.Close()

	if len(skillIDs) == 0 {
		fmt.Println("None of the blacklisted skills exist in the database.")
		return
	}

	fmt.Printf("Found %d skills to purge: %v\n", len(skillIDs), foundNames)

	// Delete from company_skill_signal
	res, err := tx.Exec(ctx, "DELETE FROM company_skill_signal WHERE skill_id = ANY($1)", skillIDs)
	if err != nil {
		log.Fatalf("Error deleting from company_skill_signal: %v", err)
	}
	fmt.Printf("Deleted %d rows from company_skill_signal\n", res.RowsAffected())

	// Delete from job_posting_skill
	res, err = tx.Exec(ctx, "DELETE FROM job_posting_skill WHERE skill_id = ANY($1)", skillIDs)
	if err != nil {
		log.Fatalf("Error deleting from job_posting_skill: %v", err)
	}
	fmt.Printf("Deleted %d rows from job_posting_skill\n", res.RowsAffected())

	// Delete from skill table
	res, err = tx.Exec(ctx, "DELETE FROM skill WHERE id = ANY($1)", skillIDs)
	if err != nil {
		log.Fatalf("Error deleting from skill: %v", err)
	}
	fmt.Printf("Deleted %d rows from skill\n", res.RowsAffected())

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("Error committing transaction: %v", err)
	}

	fmt.Println("Successfully purged skills.")
}
