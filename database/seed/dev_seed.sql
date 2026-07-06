-- =============================================================================
-- Dev/test seed data — NOT for production.
--
-- Populates a handful of students, faculty, and a coordinator, plus
-- internships, mentor assignments, reports, feedback, and one completed
-- evaluation, so the app has something to look at while testing.
--
-- Your own account (created via Google login) is left untouched — flip its
-- role yourself with:
--   UPDATE users SET role_id = (SELECT id FROM roles WHERE role_name = 'ADMIN')
--   WHERE email = 'you@somaiya.edu';
-- (swap ADMIN/COORDINATOR/FACULTY/STUDENT as needed to test each dashboard)
--
-- Safe to re-run: every insert is guarded so existing rows aren't duplicated.
-- Run with: psql "$DB_DSN" -f database/seed/dev_seed.sql
-- =============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Users: 5 students, 2 faculty, 1 coordinator
-- ---------------------------------------------------------------------------
INSERT INTO users (id, email, display_name, role_id, department)
VALUES
    ('a1111111-1111-1111-1111-111111111111', 'aarav.shah@somaiya.edu',    'Aarav Shah',      (SELECT id FROM roles WHERE role_name = 'STUDENT'),     'Computer Engineering'),
    ('a2222222-2222-2222-2222-222222222222', 'priya.mehta@somaiya.edu',   'Priya Mehta',     (SELECT id FROM roles WHERE role_name = 'STUDENT'),     'Information Technology'),
    ('a3333333-3333-3333-3333-333333333333', 'rohan.iyer@somaiya.edu',    'Rohan Iyer',      (SELECT id FROM roles WHERE role_name = 'STUDENT'),     'Computer Engineering'),
    ('a4444444-4444-4444-4444-444444444444', 'sneha.kulkarni@somaiya.edu','Sneha Kulkarni',  (SELECT id FROM roles WHERE role_name = 'STUDENT'),     'Electronics Engineering'),
    ('a5555555-5555-5555-5555-555555555555', 'karan.desai@somaiya.edu',   'Karan Desai',     (SELECT id FROM roles WHERE role_name = 'STUDENT'),     'Computer Engineering'),
    ('b1111111-1111-1111-1111-111111111111', 'anjali.rao@somaiya.edu',    'Dr. Anjali Rao',  (SELECT id FROM roles WHERE role_name = 'FACULTY'),     'Computer Engineering'),
    ('b2222222-2222-2222-2222-222222222222', 'vikram.nair@somaiya.edu',   'Prof. Vikram Nair', (SELECT id FROM roles WHERE role_name = 'FACULTY'),   'Electronics Engineering'),
    ('c1111111-1111-1111-1111-111111111111', 'meera.joshi@somaiya.edu',   'Meera Joshi',     (SELECT id FROM roles WHERE role_name = 'COORDINATOR'), 'Computer Engineering')
ON CONFLICT (email) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Internships (Karan intentionally has none — a fresh, unenrolled student to
-- test the enrollment flow). Current "semester" runs 2026-06-01 to
-- 2026-09-30; Sneha's is an earlier, already-completed internship.
-- ---------------------------------------------------------------------------
INSERT INTO internships (id, student_id, company_name, company_address, role_title, industry_mentor_name, industry_mentor_email, academic_year, start_date, end_date, status, created_by, updated_by)
VALUES
    ('d1111111-1111-1111-1111-111111111111', 'a1111111-1111-1111-1111-111111111111', 'Tata Consultancy Services', 'Mumbai, Maharashtra', 'Software Engineering Intern', 'Rakesh Sharma', 'rakesh.sharma@tcs.example.com', '2025-26', '2026-06-01', '2026-09-30', 'active',    'c1111111-1111-1111-1111-111111111111', 'c1111111-1111-1111-1111-111111111111'),
    ('d2222222-2222-2222-2222-222222222222', 'a2222222-2222-2222-2222-222222222222', 'Infosys Limited',           'Pune, Maharashtra',   'Data Analyst Intern',        'Sunita Patel',  'sunita.patel@infosys.example.com', '2025-26', '2026-06-01', '2026-09-30', 'active',    'c1111111-1111-1111-1111-111111111111', 'c1111111-1111-1111-1111-111111111111'),
    ('d3333333-3333-3333-3333-333333333333', 'a3333333-3333-3333-3333-333333333333', 'Wipro Technologies',        'Bengaluru, Karnataka','Backend Developer Intern',   'Amit Kulkarni', 'amit.kulkarni@wipro.example.com', '2025-26', '2026-06-01', '2026-09-30', 'active',    'c1111111-1111-1111-1111-111111111111', 'c1111111-1111-1111-1111-111111111111'),
    ('d4444444-4444-4444-4444-444444444444', 'a4444444-4444-4444-4444-444444444444', 'Accenture',                 'Mumbai, Maharashtra', 'QA Engineering Intern',      'Neha Verma',    'neha.verma@accenture.example.com', '2025-26', '2026-01-15', '2026-05-15', 'completed', 'c1111111-1111-1111-1111-111111111111', 'c1111111-1111-1111-1111-111111111111')
ON CONFLICT (student_id) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Mentor assignments: Aarav/Rohan/Sneha approved, Priya still pending
-- (to test the coordinator's approval queue).
-- ---------------------------------------------------------------------------
INSERT INTO mentor_assignments (internship_id, faculty_mentor_id, approved_at, approved_by, status)
SELECT 'd1111111-1111-1111-1111-111111111111', 'b1111111-1111-1111-1111-111111111111', '2026-06-02T10:00:00Z'::timestamptz, 'c1111111-1111-1111-1111-111111111111', 'approved'
WHERE NOT EXISTS (SELECT 1 FROM mentor_assignments WHERE internship_id = 'd1111111-1111-1111-1111-111111111111');

INSERT INTO mentor_assignments (internship_id, faculty_mentor_id, approved_at, approved_by, status)
SELECT 'd2222222-2222-2222-2222-222222222222', 'b1111111-1111-1111-1111-111111111111', NULL, NULL, 'pending'
WHERE NOT EXISTS (SELECT 1 FROM mentor_assignments WHERE internship_id = 'd2222222-2222-2222-2222-222222222222');

INSERT INTO mentor_assignments (internship_id, faculty_mentor_id, approved_at, approved_by, status)
SELECT 'd3333333-3333-3333-3333-333333333333', 'b2222222-2222-2222-2222-222222222222', '2026-06-03T10:00:00Z'::timestamptz, 'c1111111-1111-1111-1111-111111111111', 'approved'
WHERE NOT EXISTS (SELECT 1 FROM mentor_assignments WHERE internship_id = 'd3333333-3333-3333-3333-333333333333');

INSERT INTO mentor_assignments (internship_id, faculty_mentor_id, approved_at, approved_by, status)
SELECT 'd4444444-4444-4444-4444-444444444444', 'b2222222-2222-2222-2222-222222222222', '2026-01-16T10:00:00Z'::timestamptz, 'c1111111-1111-1111-1111-111111111111', 'approved'
WHERE NOT EXISTS (SELECT 1 FROM mentor_assignments WHERE internship_id = 'd4444444-4444-4444-4444-444444444444');

-- ---------------------------------------------------------------------------
-- Weekly reports
-- ---------------------------------------------------------------------------
INSERT INTO reports (id, internship_id, report_type, week_number, content, status, submitted_at, edited_at, approved_at, approved_by, created_by, updated_by)
VALUES
    ('e1111111-1111-1111-1111-111111111111', 'd1111111-1111-1111-1111-111111111111', 'weekly', 1, 'Onboarded with the team, set up dev environment, and completed initial training modules.', 'submitted', '2026-06-08T09:00:00Z', '2026-06-08T09:00:00Z', NULL, NULL, 'a1111111-1111-1111-1111-111111111111', 'a1111111-1111-1111-1111-111111111111'),
    ('e1111111-1111-1111-1111-111111111112', 'd1111111-1111-1111-1111-111111111111', 'weekly', 2, 'Started work on a REST API module; paired with a senior engineer on code review practices.', 'reviewed',  '2026-06-15T09:00:00Z', '2026-06-15T09:00:00Z', '2026-06-16T11:00:00Z', 'b1111111-1111-1111-1111-111111111111', 'a1111111-1111-1111-1111-111111111111', 'a1111111-1111-1111-1111-111111111111'),
    ('e3333333-3333-3333-3333-333333333111', 'd3333333-3333-3333-3333-333333333333', 'weekly', 1, 'Set up local Kubernetes cluster and explored the microservices architecture.', 'submitted', '2026-06-10T09:00:00Z', '2026-06-10T09:00:00Z', NULL, NULL, 'a3333333-3333-3333-3333-333333333333', 'a3333333-3333-3333-3333-333333333333'),
    ('e4444444-4444-4444-4444-444444444111', 'd4444444-4444-4444-4444-444444444444', 'weekly', 1, 'Learned the test automation framework and wrote first set of regression cases.', 'reviewed', '2026-01-25T09:00:00Z', '2026-01-25T09:00:00Z', '2026-01-26T10:00:00Z', 'b2222222-2222-2222-2222-222222222222', 'a4444444-4444-4444-4444-444444444444', 'a4444444-4444-4444-4444-444444444444'),
    ('e4444444-4444-4444-4444-444444444112', 'd4444444-4444-4444-4444-444444444444', 'weekly', 2, 'Automated three critical user flows; fixed two flaky tests in the existing suite.', 'reviewed', '2026-02-05T09:00:00Z', '2026-02-05T09:00:00Z', '2026-02-06T10:00:00Z', 'b2222222-2222-2222-2222-222222222222', 'a4444444-4444-4444-4444-444444444444', 'a4444444-4444-4444-4444-444444444444'),
    ('e4444444-4444-4444-4444-444444444113', 'd4444444-4444-4444-4444-444444444444', 'weekly', 3, 'Presented test coverage report to the QA lead; began work on performance testing.', 'submitted', '2026-02-20T09:00:00Z', '2026-02-20T09:00:00Z', NULL, NULL, 'a4444444-4444-4444-4444-444444444444', 'a4444444-4444-4444-4444-444444444444')
ON CONFLICT (internship_id, report_type, week_number) DO NOTHING;

-- ---------------------------------------------------------------------------
-- Faculty feedback on one report
-- ---------------------------------------------------------------------------
INSERT INTO feedback (report_id, source, given_by, comments, rating, submitted_at)
SELECT 'e1111111-1111-1111-1111-111111111112', 'faculty', 'b1111111-1111-1111-1111-111111111111', 'Good progress on the API module. Keep documenting your design decisions.', 4, '2026-06-16T11:00:00Z'::timestamptz
WHERE NOT EXISTS (SELECT 1 FROM feedback WHERE report_id = 'e1111111-1111-1111-1111-111111111112');

-- ---------------------------------------------------------------------------
-- Evaluation: Sneha's internship is complete with a locked ISE score
-- (exercises the marksheet flow). Rohan has an ISE schedule set but no score
-- yet (exercises the "faculty submits marks" flow).
-- ---------------------------------------------------------------------------
INSERT INTO evaluation_schedules (internship_id, exam_type, in_semester_at, end_semester_at, venue, set_by)
VALUES
    ('d4444444-4444-4444-4444-444444444444', 'ISE', '2026-03-15', '2026-05-10', 'Seminar Hall 2', 'c1111111-1111-1111-1111-111111111111'),
    ('d3333333-3333-3333-3333-333333333333', 'ISE', '2026-08-01', '2026-08-15', 'Seminar Hall 1', 'c1111111-1111-1111-1111-111111111111')
ON CONFLICT (internship_id, exam_type) DO NOTHING;

INSERT INTO evaluation_scores (internship_id, exam_type, report_quality, oral_presentation, work_quality, understanding, periodic_interaction, remarks, locked_at, submitted_by, submitted_at)
VALUES
    ('d4444444-4444-4444-4444-444444444444', 'ISE', 18, 26, 13, 13, 18, 'Consistent performance throughout the internship.', '2026-05-20T12:00:00Z', 'b2222222-2222-2222-2222-222222222222', '2026-05-18T12:00:00Z')
ON CONFLICT (internship_id, exam_type) DO NOTHING;

COMMIT;
