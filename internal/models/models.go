package models

type Location struct {
	Name string `json:"name"`
}

type Department struct {
	Name string `json:"name"`
}

type GreenhouseJob struct {
	ID             int64        `json:"id"`
	Title          string       `json:"title"`
	CompanyName    string       `json:"company_name"`
	Location       Location     `json:"location"`
	AbsoluteURL    string       `json:"absolute_url"`
	FirstPublished string       `json:"first_published"`
	UpdatedAt      string       `json:"updated_at"`
	Departments    []Department `json:"departments"`
	Content        string       `json:"content"`
	CleanedContent string       `json:"cleaned_content"`
}

type Skill struct {
	Name             string `json:"name"`
	RequirementLevel string `json:"requirement_level"`
}

type GeminiResponse struct {
	IsTech           bool    `json:"is_tech"`
	MinYearsRequired *int    `json:"min_years_required"`
	Skills           []Skill `json:"skills"`
}

type ExtractedJob struct {
	PostingID        string   `json:"posting_id"`
	CompanyName      string   `json:"company_name"`
	RawTitle         string   `json:"raw_title"`
	ApplyURL         string   `json:"apply_url"`
	Location         string   `json:"location"`
	PostedAt         string   `json:"posted_at"`
	LastSeenAt       string   `json:"last_seen_at"`
	Departments      []string `json:"departments"`
	CleanedContent   string   `json:"cleaned_content"`
	Skills           []Skill  `json:"skills"`
	MinYearsRequired *int     `json:"min_years_required"`
	ExtractionStatus string   `json:"extraction_status"`
}

type FailedJobLog struct {
	PostingID   string `json:"posting_id"`
	CompanyName string `json:"company_name"`
	Reason      string `json:"reason"`
}

type RunLog struct {
	RunAt                   string         `json:"run_at"`
	TotalJobsAttempted      int            `json:"total_jobs_attempted"`
	TotalJobsSucceeded      int            `json:"total_jobs_succeeded"`
	TotalJobsSkippedNonTech int            `json:"total_jobs_skipped_non_tech"`
	TotalJobsFailed         int            `json:"total_jobs_failed"`
	TotalSkillsExtracted    int            `json:"total_skills_extracted"`
	FailedJobs              []FailedJobLog `json:"failed_jobs"`
}
