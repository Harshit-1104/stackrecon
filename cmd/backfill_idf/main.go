package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
)

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

	query := `
		WITH company_count AS (
			SELECT COUNT(DISTINCT company_id)::float AS total 
			FROM company_skill_signal
		),
		skill_company_counts AS (
			SELECT skill_id, COUNT(DISTINCT company_id) AS companies_using
			FROM company_skill_signal
			GROUP BY skill_id
		)
		UPDATE skill s
		SET idf_score = LOG(1.0 + (SELECT total FROM company_count) / 
							GREATEST(scc.companies_using, 1))
		FROM skill_company_counts scc
		WHERE s.id = scc.skill_id;
	`

	res, err := conn.Exec(ctx, query)
	if err != nil {
		log.Fatalf("Error updating IDF scores: %v\n", err)
	}

	fmt.Printf("Updated IDF scores for %d skills.\n", res.RowsAffected())

	var min, max, avg float64
	err = conn.QueryRow(ctx, `SELECT MIN(idf_score), MAX(idf_score), AVG(idf_score) FROM skill WHERE idf_score != 1.0`).Scan(&min, &max, &avg)
	if err != nil {
		log.Printf("Could not compute sanity check metrics: %v\n", err)
	} else {
		fmt.Printf("Sanity check - Min: %.4f, Max: %.4f, Avg: %.4f\n", min, max, avg)
	}
}
