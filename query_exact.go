package main

import (
	"context"
	"fmt"
	"log"
	"github.com/jackc/pgx/v5"
)

func main() {
	conn, err := pgx.Connect(context.Background(), "postgres://postgres:password@localhost:5432/stackrecon")
	if err != nil { log.Fatal(err) }
	defer conn.Close(context.Background())

	query := `
		SELECT jp.id
		FROM job_posting jp
		LEFT JOIN job_posting_skill jps 
			ON jp.id = jps.posting_id 
			AND jps.skill_id = ANY($2::int[])
			AND ($7::int = 0 OR jps.skill_id != $7)
		WHERE jp.company_id = $1 AND jp.active = true
		  AND ($7::int = 0 OR EXISTS (
				SELECT 1 FROM job_posting_skill 
				WHERE posting_id = jp.id AND skill_id = $7
		  ))
		  AND ($3::float = 0.0 OR COALESCE(jp.min_years_required, jp.min_years_inferred) IS NULL OR COALESCE(jp.min_years_required, jp.min_years_inferred) <= $3::float)
		  AND (cardinality($4::text[]) = 0 OR jp.work_type = ANY($4::text[]))
		  AND (cardinality($5::text[]) = 0 OR jp.location_country = ANY($5::text[]))
		  AND (cardinality($6::text[]) = 0 OR jp.location_city = ANY($6::text[]))
	`

	var userSkillIDs []int = []int{15}
	var minYears float64 = 0.0
	var workTypes []string = []string{}
	var countries []string = []string{}
	var cities []string = []string{}
	var pivotSkillID int = 15

	rows, err := conn.Query(context.Background(), query, 231, userSkillIDs, minYears, workTypes, countries, cities, pivotSkillID)
	if err != nil { log.Fatal(err) }
	
	count := 0
	for rows.Next() { count++ }
	fmt.Printf("Jobs: %d\n", count)
}
