ALTER TABLE job_posting DROP COLUMN IF EXISTS work_type;
ALTER TABLE job_posting DROP COLUMN IF EXISTS location_city;
ALTER TABLE job_posting DROP COLUMN IF EXISTS location_country;
ALTER TABLE job_posting RENAME COLUMN location_raw TO location;
