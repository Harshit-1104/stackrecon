package handlers

import (
	"log"
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strings"
)

type MatchRequest struct {
	Skills []struct {
		SkillID int `json:"skill_id"`
	} `json:"skills"`
	ActiveOnly           bool     `json:"active_only"`
	TotalYearsExperience float64  `json:"total_years_experience"`
	WorkTypes            []string `json:"work_types"`
	Countries            []string `json:"countries"`
	Cities               []string `json:"cities"`
}

type TopSkill struct {
	Name   string `json:"name"`
	IsRare bool   `json:"is_rare"`
}

type MatchResponseItem struct {
	CompanyID        int        `json:"company_id"`
	Name             string     `json:"name"`
	Domain           string     `json:"domain"`
	CompanyScore     float64    `json:"company_score"`
	JDScore          float64    `json:"jd_score"`
	RankingScore     float64    `json:"ranking_score"`
	Rail             int        `json:"rail"`
	QualifiedJDCount int        `json:"qualified_jd_count"`
	TopMatchedSkills []TopSkill `json:"top_matched_skills"`
}

type MatchResponse struct {
	Results []MatchResponseItem `json:"results"`
	Total   int                 `json:"total"`
}

// internal struct for parsing db row
type matchRow struct {
	CompanyID           int
	Name                string
	Website             *string
	CompanyScore        float64
	JDScore             float64
	QualifiedJDCount    int
	MatchedSkillsInfo   string // json string of []struct{SkillID int, SignalStrength float64, IDFScore float64}
	HasLocationPresence bool
}

func extractDomain(website string) string {
	domain := strings.TrimSpace(website)
	domain = strings.TrimPrefix(domain, "http://")
	domain = strings.TrimPrefix(domain, "https://")
	domain = strings.TrimPrefix(domain, "www.")
	domain = strings.TrimRight(domain, "/")

	return domain
}

func (h *Handler) MatchCompanies(w http.ResponseWriter, r *http.Request) {
	var req MatchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Skills) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MatchResponse{Results: []MatchResponseItem{}, Total: 0})
		return
	}

	skillsJSON, _ := json.Marshal(req.Skills)

	query := `
		WITH user_skills AS (
			SELECT 
				CAST(value->>'skill_id' AS INTEGER) as skill_id,
				value->>'proficiency' as proficiency
			FROM jsonb_array_elements($1::jsonb)
		),
		company_qualified_jds AS (
			SELECT 
				jp.company_id, 
				COUNT(DISTINCT jp.id) as qualified_jd_count
			FROM job_posting jp
			JOIN job_posting_skill jps ON jp.id = jps.posting_id
			JOIN user_skills us ON jps.skill_id = us.skill_id
			WHERE jp.active = true 
			  AND ($3::float = 0.0 OR COALESCE(jp.min_years_required, jp.min_years_inferred) IS NULL OR COALESCE(jp.min_years_required, jp.min_years_inferred) <= $3::float)
			  AND (cardinality($4::text[]) = 0 OR jp.work_type = ANY($4::text[]))
			  AND (cardinality($5::text[]) = 0 OR jp.location_country = ANY($5::text[]))
			  AND (cardinality($6::text[]) = 0 OR jp.location_city = ANY($6::text[]))
			GROUP BY jp.company_id
		),
		company_location_presence AS (
			SELECT DISTINCT jp.company_id
			FROM job_posting jp
			WHERE jp.active = true
			  AND (cardinality($4::text[]) = 0 OR jp.work_type = ANY($4::text[]))
			  AND (cardinality($5::text[]) = 0 OR jp.location_country = ANY($5::text[]))
			  AND (cardinality($6::text[]) = 0 OR jp.location_city = ANY($6::text[]))
		),
		total_skills AS (
			SELECT COUNT(*) as cnt FROM user_skills
		),
		company_totals AS (
			SELECT company_id, SUM(jd_count) as total_jd_count
			FROM company_skill_signal
			GROUP BY company_id
		),
		company_signals AS (
			SELECT 
				css.company_id,
				css.skill_id,
				css.active,
				css.jd_count,
				sk.idf_score,
				CASE 
					WHEN ct.total_jd_count = 0 THEN 0
					ELSE css.jd_count::float / ct.total_jd_count
				END as signal_strength
			FROM company_skill_signal css
			JOIN company_totals ct ON css.company_id = ct.company_id
			JOIN skill sk ON sk.id = css.skill_id
		),
		company_scores AS (
			SELECT 
				cs.company_id,
				(SUM(cs.signal_strength * cs.idf_score) /
				 NULLIF((
					 SELECT SUM(sk2.idf_score)
					 FROM user_skills us2
					 JOIN skill sk2 ON sk2.id = us2.skill_id
				 ), 0)
				) as company_score,
				((COUNT(DISTINCT cs.skill_id) FILTER (WHERE us.skill_id IS NOT NULL AND cs.active = true))::float / (SELECT cnt FROM total_skills)) as jd_score,
				jsonb_agg(
					jsonb_build_object(
						'skill_id', cs.skill_id,
						'signal_strength', cs.signal_strength,
						'idf_score', cs.idf_score
					)
				) FILTER (WHERE us.skill_id IS NOT NULL) as matched_skills_info
			FROM company_signals cs
			JOIN user_skills us ON cs.skill_id = us.skill_id
			WHERE (cs.active = true OR $2 = false)
			GROUP BY cs.company_id
		)
		SELECT 
			c.id, 
			c.name,
			c.website,
			coalesce(cs.company_score, 0) as company_score,
			coalesce(cs.jd_score, 0) as jd_score,
			coalesce(cqj.qualified_jd_count, 0) as qualified_jd_count,
			coalesce(cs.matched_skills_info, '[]'::jsonb) as matched_skills_info,
			CASE WHEN clp.company_id IS NOT NULL THEN true ELSE false END as has_location_presence
		FROM company c
		JOIN company_scores cs ON c.id = cs.company_id
		LEFT JOIN company_qualified_jds cqj ON c.id = cqj.company_id
		LEFT JOIN company_location_presence clp ON c.id = clp.company_id
		WHERE cs.company_score > 0
	`

	workTypes := req.WorkTypes
	if workTypes == nil {
		workTypes = []string{}
	}
	countries := req.Countries
	if countries == nil {
		countries = []string{}
	}
	cities := req.Cities
	if cities == nil {
		cities = []string{}
	}

	rows, err := h.db.Query(r.Context(), query, skillsJSON, req.ActiveOnly, req.TotalYearsExperience, workTypes, countries, cities)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var dbRows []matchRow

	for rows.Next() {
		var row matchRow
		err := rows.Scan(
			&row.CompanyID,
			&row.Name,
			&row.Website,
			&row.CompanyScore,
			&row.JDScore,
			&row.QualifiedJDCount,
			&row.MatchedSkillsInfo,
			&row.HasLocationPresence,
		)
		if err != nil {
			continue
		}
		dbRows = append(dbRows, row)
	}

	if len(dbRows) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MatchResponse{Results: []MatchResponseItem{}, Total: 0})
		return
	}

	// Get skill names map to populate TopMatchedSkills
	skillMap := make(map[int]string)
	sRows, _ := h.db.Query(r.Context(), "SELECT id, canonical_name FROM skill")
	if sRows != nil {
		for sRows.Next() {
			var id int
			var name string
			sRows.Scan(&id, &name)
			skillMap[id] = name
		}
		sRows.Close()
	}

	var res MatchResponse
	res.Results = []MatchResponseItem{}
	
	filtersActive := len(req.WorkTypes) > 0 || len(req.Countries) > 0 || len(req.Cities) > 0

	for _, r := range dbRows {
		rail := 0
		if r.QualifiedJDCount > 0 {
			rail = 1
		} else if !filtersActive || r.HasLocationPresence {
			rail = 2
		}

		if rail == 0 {
			continue
		}

		domain := ""
		if r.Website != nil {
			domain = extractDomain(*r.Website)
		}

		// parse matched skills info
		type skillInfo struct {
			SkillID        int     `json:"skill_id"`
			SignalStrength float64 `json:"signal_strength"`
			IDFScore       float64 `json:"idf_score"`
		}
		var sInfos []skillInfo
		json.Unmarshal([]byte(r.MatchedSkillsInfo), &sInfos)

		sort.Slice(sInfos, func(i, j int) bool {
			scoreI := sInfos[i].SignalStrength * sInfos[i].IDFScore
			scoreJ := sInfos[j].SignalStrength * sInfos[j].IDFScore
			return scoreI > scoreJ
		})

		var topSkills []TopSkill
		for i, si := range sInfos {
			if i >= 4 {
				break
			}
			if name, ok := skillMap[si.SkillID]; ok {
				isRare := si.IDFScore > 2.1
				topSkills = append(topSkills, TopSkill{Name: name, IsRare: isRare})
			}
		}

		if topSkills == nil {
			topSkills = []TopSkill{}
		}

		rankingScore := r.CompanyScore * math.Log(1.0 + float64(r.QualifiedJDCount))

		res.Results = append(res.Results, MatchResponseItem{
			CompanyID:        r.CompanyID,
			Name:             r.Name,
			Domain:           domain,
			CompanyScore:     r.CompanyScore,
			JDScore:          r.JDScore,
			RankingScore:     rankingScore,
			Rail:             rail,
			QualifiedJDCount: r.QualifiedJDCount,
			TopMatchedSkills: topSkills,
		})
	}

	// sort by rail (1 -> 2 -> 3), then by ranking score
	sort.Slice(res.Results, func(i, j int) bool {
		if res.Results[i].Rail != res.Results[j].Rail {
			return res.Results[i].Rail < res.Results[j].Rail
		}
		return res.Results[i].RankingScore > res.Results[j].RankingScore
	})

	res.Total = len(res.Results)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}
