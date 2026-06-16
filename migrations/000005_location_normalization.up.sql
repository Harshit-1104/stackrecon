ALTER TABLE job_posting RENAME COLUMN location TO location_raw;
ALTER TABLE job_posting ADD COLUMN IF NOT EXISTS location_country TEXT NULL;
ALTER TABLE job_posting ADD COLUMN IF NOT EXISTS location_city TEXT NULL;
ALTER TABLE job_posting ADD COLUMN IF NOT EXISTS work_type TEXT NULL;
