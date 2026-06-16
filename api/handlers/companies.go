package handlers

import (
	"log"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

type RequirementLevels struct {
	Required  int `json:"required"`
	Preferred int `json:"preferred"`
	Mentioned int `json:"mentioned"`
}

type StackSkill struct {
	SkillID           int               `json:"skill_id"`
	CanonicalName     string            `json:"canonical_name"`
	SignalStrength    float64           `json:"signal_strength"`
	IDFScore          float64           `json:"idf_score"`
	Active            bool              `json:"active"`
	JDCount           int               `json:"jd_count"`
	RequirementLevels RequirementLevels `json:"requirement_levels"`
}

type CompanyStackResponse struct {
	CompanyID     int                     `json:"company_id"`
	Name          string                  `json:"name"`
	Website       *string                 `json:"website"`
	TotalJDCount  int                     `json:"total_jd_count"`
	ActiveJDCount int                     `json:"active_jd_count"`
	Stack         map[string][]StackSkill `json:"stack"`
	Overlap       *Overlap                `json:"overlap,omitempty"`
}

type Overlap struct {
	Matched []int `json:"matched"`
	Gap     []int `json:"gap"`
}

func (h *Handler) CompanyStack(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	companyID, _ := strconv.Atoi(idStr)

	skillIDsParam := r.URL.Query().Get("skill_ids")
	var userSkillIDs []int
	if skillIDsParam != "" {
		for _, s := range strings.Split(skillIDsParam, ",") {
			sid, _ := strconv.Atoi(s)
			if sid > 0 {
				userSkillIDs = append(userSkillIDs, sid)
			}
		}
	}

	var name string
	var website *string
	err := h.db.QueryRow(r.Context(), "SELECT name, website FROM company WHERE id = $1", companyID).Scan(&name, &website)
	if err != nil {
		http.Error(w, "Company not found", http.StatusNotFound)
		return
	}

	var totalJDCount, activeJDCount int
	err = h.db.QueryRow(r.Context(), `
		SELECT 
			COUNT(DISTINCT id) as total_jd_count,
			COUNT(DISTINCT id) FILTER (WHERE active = true AND last_seen_at >= NOW() - INTERVAL '30 days') as active_jd_count
		FROM job_posting 
		WHERE company_id = $1
	`, companyID).Scan(&totalJDCount, &activeJDCount)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	query := `
		WITH req_levels AS (
			SELECT 
				jps.skill_id,
				COUNT(*) FILTER (WHERE jps.requirement_level = 'required') as req_count,
				COUNT(*) FILTER (WHERE jps.requirement_level = 'preferred') as pref_count,
				COUNT(*) FILTER (WHERE jps.requirement_level = 'mentioned') as ment_count
			FROM job_posting_skill jps
			JOIN job_posting jp ON jps.posting_id = jp.id
			WHERE jp.company_id = $1
			GROUP BY jps.skill_id
		)
		SELECT 
			css.skill_id,
			s.canonical_name,
			s.idf_score,
			COALESCE(sc.name, 'Other') as category,
			css.active,
			css.jd_count,
			COALESCE(css.jd_count::float / NULLIF($2::int, 0), 0) as signal_strength,
			COALESCE(rl.req_count, 0) as req_count,
			COALESCE(rl.pref_count, 0) as pref_count,
			COALESCE(rl.ment_count, 0) as ment_count
		FROM company_skill_signal css
		JOIN skill s ON css.skill_id = s.id
		LEFT JOIN skill_category sc ON s.category_id = sc.id
		LEFT JOIN req_levels rl ON css.skill_id = rl.skill_id
		WHERE css.company_id = $1
	`

	rows, err := h.db.Query(r.Context(), query, companyID, activeJDCount)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	res := CompanyStackResponse{
		CompanyID:     companyID,
		Name:          name,
		Website:       website,
		TotalJDCount:  totalJDCount,
		ActiveJDCount: activeJDCount,
		Stack:         make(map[string][]StackSkill),
	}

	var allSkills []int

	for rows.Next() {
		var sk StackSkill
		var cat string
		if err := rows.Scan(&sk.SkillID, &sk.CanonicalName, &sk.IDFScore, &cat, &sk.Active, &sk.JDCount, &sk.SignalStrength, &sk.RequirementLevels.Required, &sk.RequirementLevels.Preferred, &sk.RequirementLevels.Mentioned); err != nil {
			continue
		}
		res.Stack[cat] = append(res.Stack[cat], sk)
		allSkills = append(allSkills, sk.SkillID)
	}

	if len(userSkillIDs) > 0 {
		res.Overlap = &Overlap{
			Matched: []int{},
			Gap:     []int{},
		}
		
		userSkillMap := make(map[int]bool)
		for _, uid := range userSkillIDs {
			userSkillMap[uid] = true
		}

		for _, sid := range allSkills {
			if userSkillMap[sid] {
				res.Overlap.Matched = append(res.Overlap.Matched, sid)
			} else {
				res.Overlap.Gap = append(res.Overlap.Gap, sid)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

type PostingResponse struct {
	ID                string   `json:"id"`
	Title             string   `json:"title"`
	ApplyURL          string   `json:"apply_url"`
	MatchedSkills     []string `json:"matched_skills"`
	MissingSkills     []string `json:"missing_skills"`
	PostedAt          string   `json:"posted_at"`
	MinYearsRequired  *int     `json:"min_years_required"`
	TotalSkillsInJob  int      `json:"total_skills_in_job"`
	MatchRatio        float64  `json:"match_ratio"`
}

type CompanyPostingsResponse struct {
	Postings []PostingResponse `json:"postings"`
}

func (h *Handler) CompanyPostings(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	companyID, _ := strconv.Atoi(idStr)

	skillIDsParam := r.URL.Query().Get("skill_ids")
	var userSkillIDs []int
	if skillIDsParam != "" {
		for _, s := range strings.Split(skillIDsParam, ",") {
			sid, _ := strconv.Atoi(s)
			if sid > 0 {
				userSkillIDs = append(userSkillIDs, sid)
			}
		}
	}

	minYearsStr := r.URL.Query().Get("min_years")
	var minYears float64
	if minYearsStr != "" {
		minYears, _ = strconv.ParseFloat(minYearsStr, 64)
	}

	workTypes := r.URL.Query()["work_types"]
	if workTypes == nil {
		workTypes = []string{}
	}
	countries := r.URL.Query()["countries"]
	if countries == nil {
		countries = []string{}
	}
	cities := r.URL.Query()["cities"]
	if cities == nil {
		cities = []string{}
	}

	pivotSkillIDStr := r.URL.Query().Get("pivot_skill_id")
	var pivotSkillID int
	if pivotSkillIDStr != "" {
		pivotSkillID, _ = strconv.Atoi(pivotSkillIDStr)
	}

	query := `
		SELECT 
			jp.id, 
			jp.raw_title, 
			coalesce(jp.apply_url, ''),
			coalesce(to_char(jp.posted_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), ''),
			coalesce(
				jsonb_agg(s.canonical_name) FILTER (WHERE s.canonical_name IS NOT NULL),
				'[]'::jsonb
			) as matched_skills,
			coalesce(
				(SELECT jsonb_agg(s2.canonical_name) 
				 FROM job_posting_skill jps2 
				 JOIN skill s2 ON jps2.skill_id = s2.id 
				 WHERE jps2.posting_id = jp.id 
				 AND NOT (jps2.skill_id = ANY($2::int[]))
				 AND ($7::int = 0 OR jps2.skill_id != $7)
				), '[]'::jsonb
			) as missing_skills,
			COALESCE(jp.min_years_required, jp.min_years_inferred) as min_years_required,
			(SELECT count(*) FROM job_posting_skill WHERE posting_id = jp.id) as total_skills_in_job,
			CASE 
				WHEN (SELECT count(*) FROM job_posting_skill WHERE posting_id = jp.id) = 0 THEN 0
				WHEN (SELECT count(*) FROM job_posting_skill WHERE posting_id = jp.id AND ($7::int = 0 OR skill_id != $7)) = 0 THEN 1.0
				ELSE count(jps.skill_id)::float / 
					 (SELECT count(*) FROM job_posting_skill 
							 WHERE posting_id = jp.id 
							 AND ($7::int = 0 OR skill_id != $7))
			END as match_ratio
		FROM job_posting jp
		LEFT JOIN job_posting_skill jps 
			ON jp.id = jps.posting_id 
			AND jps.skill_id = ANY($2::int[])
			AND ($7::int = 0 OR jps.skill_id != $7)
		LEFT JOIN skill s ON jps.skill_id = s.id
		WHERE jp.company_id = $1 AND jp.active = true
		  AND ($7::int = 0 OR EXISTS (
				SELECT 1 FROM job_posting_skill 
				WHERE posting_id = jp.id AND skill_id = $7
		  ))
		  AND ($3::float = 0.0 OR COALESCE(jp.min_years_required, jp.min_years_inferred) IS NULL OR COALESCE(jp.min_years_required, jp.min_years_inferred) <= $3::float)
		  AND (cardinality($4::text[]) = 0 OR jp.work_type = ANY($4::text[]))
		  AND (cardinality($5::text[]) = 0 OR jp.location_country = ANY($5::text[]))
		  AND (cardinality($6::text[]) = 0 OR jp.location_city = ANY($6::text[]))
		GROUP BY jp.id
		ORDER BY match_ratio DESC, jp.posted_at DESC
		LIMIT 20
	`

	rows, err := h.db.Query(r.Context(), query, companyID, userSkillIDs, minYears, workTypes, countries, cities, pivotSkillID)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var res CompanyPostingsResponse
	res.Postings = []PostingResponse{}

	for rows.Next() {
		var p PostingResponse
		var matchedSkillsJSON []byte
		var missingSkillsJSON []byte
		if err := rows.Scan(&p.ID, &p.Title, &p.ApplyURL, &p.PostedAt, &matchedSkillsJSON, &missingSkillsJSON, &p.MinYearsRequired, &p.TotalSkillsInJob, &p.MatchRatio); err != nil {
			continue
		}
		json.Unmarshal(matchedSkillsJSON, &p.MatchedSkills)
		if p.MatchedSkills == nil {
			p.MatchedSkills = []string{}
		}
		json.Unmarshal(missingSkillsJSON, &p.MissingSkills)
		if p.MissingSkills == nil {
			p.MissingSkills = []string{}
		}
		res.Postings = append(res.Postings, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

type SearchCompanyResponse struct {
	ID           int            `json:"id"`
	Name         string         `json:"name"`
	Website      *string        `json:"website"`
	TotalJDCount int            `json:"total_jd_count"`
	TopSkills    []TopSkillInfo `json:"top_skills"`
}

type TopSkillInfo struct {
	Name           string  `json:"name"`
	SignalStrength float64 `json:"signal_strength"`
	IDFScore       float64 `json:"idf_score"`
}

func (h *Handler) SearchCompanies(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	
	searchParam := ""
	if q != "" {
		searchParam = "%" + q + "%"
	}

	query := `
		WITH company_jds AS (
			SELECT company_id, SUM(jd_count) as total_jd_count
			FROM company_skill_signal
			GROUP BY company_id
		),
		top_skills AS (
			SELECT 
				css.company_id,
				s.canonical_name,
				s.idf_score,
				CASE 
					WHEN cj.total_jd_count = 0 THEN 0
					ELSE css.jd_count::float / cj.total_jd_count
				END as signal_strength
			FROM company_skill_signal css
			JOIN skill s ON css.skill_id = s.id
			JOIN company_jds cj ON css.company_id = cj.company_id
			WHERE css.active = true
		)
		SELECT 
			c.id, 
			c.name,
			c.website,
			COALESCE(cj.total_jd_count, 0) as total_jd_count,
			COALESCE(
				(SELECT jsonb_agg(
					jsonb_build_object('name', ts.canonical_name, 'signal_strength', ts.signal_strength, 'idf_score', ts.idf_score)
				) FROM (
					SELECT canonical_name, signal_strength, idf_score 
					FROM top_skills 
					WHERE company_id = c.id 
					ORDER BY (signal_strength * idf_score) DESC 
					LIMIT 5
				) ts), '[]'::jsonb
			) as top_skills_json
		FROM company c
		LEFT JOIN company_jds cj ON c.id = cj.company_id
		WHERE $1::text = '' OR c.name ILIKE $1
		ORDER BY COALESCE(cj.total_jd_count, 0) DESC
		LIMIT CASE WHEN $1::text = '' THEN 12 ELSE 10 END
	`

	rows, err := h.db.Query(r.Context(), query, searchParam)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []SearchCompanyResponse
	for rows.Next() {
		var res SearchCompanyResponse
		var tsJSON []byte
		if err := rows.Scan(&res.ID, &res.Name, &res.Website, &res.TotalJDCount, &tsJSON); err != nil {
			continue
		}
		json.Unmarshal(tsJSON, &res.TopSkills)
		if res.TopSkills == nil {
			res.TopSkills = []TopSkillInfo{}
		}
		results = append(results, res)
	}
	if results == nil {
		results = []SearchCompanyResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

type CompanyFitRequest struct {
	SkillIDs []int `json:"skill_ids"`
}

type MissingSkill struct {
	Name           string  `json:"name"`
	SignalStrength float64 `json:"signal_strength"`
	IDFScore       float64 `json:"idf_score"`
}

type CompanyFitResponse struct {
	MatchScore               float64        `json:"match_score"`
	MatchedSkills            []string       `json:"matched_skills"`
	MissingHighSignalSkills  []MissingSkill `json:"missing_high_signal_skills"`
}

func (h *Handler) CompanyFit(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	companyID, _ := strconv.Atoi(idStr)

	var req CompanyFitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.SkillIDs) == 0 {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(CompanyFitResponse{
			MatchedSkills: []string{},
			MissingHighSignalSkills: []MissingSkill{},
		})
		return
	}

	query := `
		WITH user_skills AS (
			SELECT unnest($2::int[]) as skill_id
		),
		company_totals AS (
			SELECT COUNT(DISTINCT id) as active_jd_count
			FROM job_posting
			WHERE company_id = $1 AND active = true AND last_seen_at >= NOW() - INTERVAL '30 days'
		),
		company_signals AS (
			SELECT 
				css.skill_id,
				s.canonical_name,
				s.idf_score,
				CASE 
					WHEN ct.active_jd_count = 0 THEN 0
					ELSE css.jd_count::float / ct.active_jd_count
				END as signal_strength,
				CASE WHEN us.skill_id IS NOT NULL THEN true ELSE false END as is_matched
			FROM company_skill_signal css
			CROSS JOIN company_totals ct
			JOIN skill s ON s.id = css.skill_id
			LEFT JOIN user_skills us ON css.skill_id = us.skill_id
			WHERE css.company_id = $1 AND css.active = true
		)
		SELECT 
			COALESCE(
				(SUM(signal_strength * idf_score) FILTER (WHERE is_matched = true)) /
				NULLIF((
					SELECT SUM(signal_strength * idf_score) FROM company_signals
				), 0),
				0
			) as match_score,
			COALESCE(
				jsonb_agg(canonical_name) FILTER (WHERE is_matched = true),
				'[]'::jsonb
			) as matched_skills,
			COALESCE(
				(SELECT jsonb_agg(
					jsonb_build_object('name', canonical_name, 'signal_strength', signal_strength, 'idf_score', idf_score)
				) FROM (
					SELECT canonical_name, signal_strength, idf_score 
					FROM company_signals
					WHERE is_matched = false
					ORDER BY (signal_strength * idf_score) DESC 
					LIMIT 5
				) ms), '[]'::jsonb
			) as missing_skills
		FROM company_signals
	`
	
	var res CompanyFitResponse
	var matchedJSON, missingJSON []byte
	err := h.db.QueryRow(r.Context(), query, companyID, req.SkillIDs).Scan(&res.MatchScore, &matchedJSON, &missingJSON)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	json.Unmarshal(matchedJSON, &res.MatchedSkills)
	if res.MatchedSkills == nil {
		res.MatchedSkills = []string{}
	}
	json.Unmarshal(missingJSON, &res.MissingHighSignalSkills)
	if res.MissingHighSignalSkills == nil {
		res.MissingHighSignalSkills = []MissingSkill{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(res)
}

func (h *Handler) CompanySimilar(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	companyID, _ := strconv.Atoi(idStr)

	query := `
		WITH company_jds AS (
			SELECT company_id, SUM(jd_count) as total_jd_count
			FROM company_skill_signal
			GROUP BY company_id
		),
		all_signals AS (
			SELECT 
				css.company_id,
				css.skill_id,
				s.canonical_name,
				s.idf_score,
				CASE 
					WHEN cj.total_jd_count = 0 THEN 0
					ELSE css.jd_count::float / cj.total_jd_count
				END as signal_strength
			FROM company_skill_signal css
			JOIN skill s ON css.skill_id = s.id
			JOIN company_jds cj ON css.company_id = cj.company_id
			WHERE css.active = true
		),
		target_company AS (
			SELECT skill_id, (signal_strength * idf_score) as val
			FROM all_signals
			WHERE company_id = $1
		),
		other_companies AS (
			SELECT company_id, skill_id, (signal_strength * idf_score) as val
			FROM all_signals
			WHERE company_id != $1
		),
		target_mag AS (
			SELECT SQRT(SUM(val * val)) as mag
			FROM target_company
		),
		other_mags AS (
			SELECT company_id, SQRT(SUM(val * val)) as mag
			FROM other_companies
			GROUP BY company_id
		),
		similarities AS (
			SELECT 
				o.company_id,
				SUM(t.val * o.val) / (MAX(tm.mag) * MAX(om.mag)) as cosine_sim
			FROM target_company t
			JOIN other_companies o ON t.skill_id = o.skill_id
			CROSS JOIN target_mag tm
			JOIN other_mags om ON o.company_id = om.company_id
			WHERE tm.mag > 0 AND om.mag > 0
			GROUP BY o.company_id
		)
		SELECT 
			c.id, 
			c.name,
			c.website,
			COALESCE(cj.total_jd_count, 0) as total_jd_count,
			COALESCE(
				(SELECT jsonb_agg(
					jsonb_build_object('name', ts.canonical_name, 'signal_strength', ts.signal_strength, 'idf_score', ts.idf_score)
				) FROM (
					SELECT canonical_name, signal_strength, idf_score 
					FROM all_signals 
					WHERE company_id = c.id 
					ORDER BY (signal_strength * idf_score) DESC 
					LIMIT 5
				) ts), '[]'::jsonb
			) as top_skills_json
		FROM similarities sim
		JOIN company c ON sim.company_id = c.id
		LEFT JOIN company_jds cj ON c.id = cj.company_id
		ORDER BY sim.cosine_sim DESC
		LIMIT 8
	`

	rows, err := h.db.Query(r.Context(), query, companyID)
	if err != nil {
		log.Printf("Internal server error: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []SearchCompanyResponse
	for rows.Next() {
		var res SearchCompanyResponse
		var tsJSON []byte
		if err := rows.Scan(&res.ID, &res.Name, &res.Website, &res.TotalJDCount, &tsJSON); err != nil {
			continue
		}
		json.Unmarshal(tsJSON, &res.TopSkills)
		if res.TopSkills == nil {
			res.TopSkills = []TopSkillInfo{}
		}
		results = append(results, res)
	}
	if results == nil {
		results = []SearchCompanyResponse{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
