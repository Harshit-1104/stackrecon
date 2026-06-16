-- SkillCategory
CREATE TABLE skill_category (
  id                SERIAL PRIMARY KEY,
  name              VARCHAR(100) NOT NULL,
  parent_category_id INTEGER REFERENCES skill_category(id),
  UNIQUE(name)
);

-- Skill
CREATE TABLE skill (
  id             SERIAL PRIMARY KEY,
  canonical_name VARCHAR(255) NOT NULL,
  category_id    INTEGER REFERENCES skill_category(id),
  description    TEXT,
  UNIQUE(canonical_name)
);

-- SkillAlias
CREATE TABLE skill_alias (
  id       SERIAL PRIMARY KEY,
  skill_id INTEGER NOT NULL REFERENCES skill(id) ON DELETE CASCADE,
  alias    VARCHAR(255) NOT NULL,
  UNIQUE(alias)
);

-- SkillCoOccurrence
CREATE TABLE skill_co_occurrence (
  skill_a_id         INTEGER NOT NULL REFERENCES skill(id),
  skill_b_id         INTEGER NOT NULL REFERENCES skill(id),
  co_occurrence_count INTEGER NOT NULL DEFAULT 0,
  context            VARCHAR(50) NOT NULL,
  last_computed_at   TIMESTAMPTZ,
  PRIMARY KEY (skill_a_id, skill_b_id, context),
  CHECK (skill_a_id < skill_b_id)
);

-- User
CREATE TABLE "user" (
  id         SERIAL PRIMARY KEY,
  name       VARCHAR(255),
  email      VARCHAR(255) NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE(email)
);

-- UserSkill
CREATE TABLE user_skill (
  user_id             INTEGER NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
  skill_id            INTEGER NOT NULL REFERENCES skill(id),
  proficiency_level   VARCHAR(50) NOT NULL
                      CHECK (proficiency_level IN ('beginner','intermediate','advanced','expert')),
  years_of_experience DECIMAL(4,1),
  PRIMARY KEY (user_id, skill_id)
);

-- Company
CREATE TABLE company (
  id          SERIAL PRIMARY KEY,
  name        VARCHAR(255) NOT NULL,
  description TEXT,
  website     VARCHAR(500),
  UNIQUE(name)
);

-- CompanySource
CREATE TABLE company_source (
  id                SERIAL PRIMARY KEY,
  company_id        INTEGER NOT NULL REFERENCES company(id) ON DELETE CASCADE,
  source_type       VARCHAR(50) NOT NULL,
  source_identifier VARCHAR(500) NOT NULL,
  is_primary        BOOLEAN NOT NULL DEFAULT false,
  last_scraped_at   TIMESTAMPTZ,
  active            BOOLEAN NOT NULL DEFAULT true,
  UNIQUE(company_id, source_type, source_identifier)
);

-- RoleType
CREATE TABLE role_type (
  id   SERIAL PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  UNIQUE(name)
);

-- JobPosting
CREATE TABLE job_posting (
  id               VARCHAR(50) PRIMARY KEY,
  company_id       INTEGER NOT NULL REFERENCES company(id),
  role_type_id     INTEGER REFERENCES role_type(id),
  company_source_id INTEGER NOT NULL REFERENCES company_source(id),
  raw_title        VARCHAR(500),
  content          TEXT,
  apply_url        VARCHAR(1000),
  last_seen_at     TIMESTAMPTZ,
  active           BOOLEAN NOT NULL DEFAULT true,
  posted_at        TIMESTAMPTZ,
  expired_at       TIMESTAMPTZ
);

-- JobPostingSkill
CREATE TABLE job_posting_skill (
  posting_id        VARCHAR(50) NOT NULL REFERENCES job_posting(id) ON DELETE CASCADE,
  skill_id          INTEGER NOT NULL REFERENCES skill(id),
  requirement_level VARCHAR(50) NOT NULL
                    CHECK (requirement_level IN ('required','preferred','mentioned')),
  extracted_by      VARCHAR(50) NOT NULL
                    CHECK (extracted_by IN ('keyword','llm')),
  PRIMARY KEY (posting_id, skill_id)
);

-- CompanySkillSignal
CREATE TABLE company_skill_signal (
  company_id       INTEGER NOT NULL REFERENCES company(id) ON DELETE CASCADE,
  skill_id         INTEGER NOT NULL REFERENCES skill(id),
  active           BOOLEAN NOT NULL DEFAULT false,
  jd_count         INTEGER NOT NULL DEFAULT 0,
  jd_expired_count INTEGER NOT NULL DEFAULT 0,
  github_count     INTEGER NOT NULL DEFAULT 0,
  blog_count       INTEGER NOT NULL DEFAULT 0,
  last_computed_at TIMESTAMPTZ,
  PRIMARY KEY (company_id, skill_id)
);

-- RoleTypeSkillFrequency
CREATE TABLE role_type_skill_frequency (
  role_type_id     INTEGER NOT NULL REFERENCES role_type(id),
  skill_id         INTEGER NOT NULL REFERENCES skill(id),
  demand_frequency DECIMAL(4,3) NOT NULL DEFAULT 0.0,
  sample_size      INTEGER NOT NULL DEFAULT 0,
  last_computed_at TIMESTAMPTZ,
  PRIMARY KEY (role_type_id, skill_id)
);

-- Indexes
CREATE INDEX idx_job_posting_company_id        ON job_posting(company_id);
CREATE INDEX idx_job_posting_active            ON job_posting(active);
CREATE INDEX idx_job_posting_skill_skill_id    ON job_posting_skill(skill_id);
CREATE INDEX idx_company_skill_signal_skill_id ON company_skill_signal(skill_id);
CREATE INDEX idx_company_skill_signal_active   ON company_skill_signal(active);
CREATE INDEX idx_skill_category_id             ON skill(category_id);
CREATE INDEX idx_company_source_company_id     ON company_source(company_id);
CREATE INDEX idx_skill_alias_skill_id          ON skill_alias(skill_id);