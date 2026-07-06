-- One-time reset for a dev/test Neon database that has an out-of-date schema
-- applied (e.g. a roles table with a "name" column instead of "role_name").
-- Drops everything schema.sql creates - and a few older/candidate names in
-- case an earlier version of schema.sql was applied - then you can re-run
-- schema.sql to recreate everything fresh.
--
-- DESTRUCTIVE: this deletes all data in these tables. Only run this against
-- a database you're OK wiping.

BEGIN;

DROP TABLE IF EXISTS audit_logs CASCADE;
DROP TABLE IF EXISTS email_delivery_requests CASCADE;
DROP TABLE IF EXISTS marksheets CASCADE;
DROP TABLE IF EXISTS evaluation_corrections CASCADE;
DROP TABLE IF EXISTS evaluation_scores CASCADE;
DROP TABLE IF EXISTS evaluation_schedules CASCADE;
DROP TABLE IF EXISTS evaluation_marks CASCADE;      -- older design, if present
DROP TABLE IF EXISTS evaluations CASCADE;           -- older design, if present
DROP TABLE IF EXISTS industry_access_tokens CASCADE;
DROP TABLE IF EXISTS feedback CASCADE;
DROP TABLE IF EXISTS reports CASCADE;
DROP TABLE IF EXISTS mentor_assignments CASCADE;
DROP TABLE IF EXISTS internships CASCADE;
DROP TABLE IF EXISTS revoked_tokens CASCADE;
DROP TABLE IF EXISTS college_students CASCADE;      -- unused, if present
DROP TABLE IF EXISTS college_faculty CASCADE;       -- unused, if present
DROP TABLE IF EXISTS users CASCADE;
DROP TABLE IF EXISTS roles CASCADE;

DROP TYPE IF EXISTS report_type CASCADE;
DROP TYPE IF EXISTS report_status CASCADE;
DROP TYPE IF EXISTS feedback_source CASCADE;
DROP TYPE IF EXISTS role_name CASCADE;              -- older design, if present
DROP TYPE IF EXISTS internship_status CASCADE;      -- older design, if present
DROP TYPE IF EXISTS assignment_status CASCADE;      -- older design, if present
DROP TYPE IF EXISTS exam_type CASCADE;              -- older design, if present
DROP TYPE IF EXISTS email_status CASCADE;           -- older design, if present

DROP FUNCTION IF EXISTS fn_set_updated_at CASCADE;

COMMIT;
