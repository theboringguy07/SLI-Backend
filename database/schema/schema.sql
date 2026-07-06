-- =============================================================================
-- SLI Backend — canonical schema (source of truth)
--
-- Generated to match internal/domain/*.go exactly (field names and
-- nullability). The Go app talks to Postgres directly through pgx - no ORM,
-- no auto-migration (see internal/repositories/*.go and
-- internal/platform/db/migrate.go) - apply this file directly to a fresh
-- database, then start the API. The API additionally seeds the fixed rows in
-- `roles` on every startup (see migrate.go); this file intentionally does not
-- duplicate that seed data.
--
-- A few columns use native Postgres ENUM types (report_type, report_status,
-- feedback_source) because the corresponding repository queries bind them as
-- such. Every other Go "enum" (a plain Go string type - role names,
-- internship/assignment/outbox/exam-type status) is a TEXT column with a
-- CHECK constraint instead, which is simpler to evolve without an
-- ALTER TYPE ... ADD VALUE migration.
--
-- Tables with no corresponding Go domain struct or repository
-- (college_students, college_faculty, evaluation_marks from an earlier
-- design) have been removed.
-- =============================================================================

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

CREATE OR REPLACE FUNCTION fn_set_updated_at()
RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;

-- ---------------------------------------------------------------------------
-- Native enum types. Only for columns where the Go struct explicitly sets
-- gorm:"type:<name>" (internal/domain/report.go, internal/domain/enums.go).
-- ---------------------------------------------------------------------------
DO $$ BEGIN
    CREATE TYPE report_type AS ENUM ('weekly', 'fortnightly', 'monthly');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE report_status AS ENUM ('draft', 'submitted', 'reviewed');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
    CREATE TYPE feedback_source AS ENUM ('faculty', 'industry');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ---------------------------------------------------------------------------
-- Identity / RBAC (internal/domain/user.go, internal/domain/enums.go)
--
-- One role per user (users.role_id), matching domain.User.RoleID/Role - not
-- a many-to-many. Role values are exactly domain.AllRoles: STUDENT, FACULTY,
-- COORDINATOR, ADMIN. There is no separate HOD or SUPER_ADMIN role; HOD- and
-- superadmin-only endpoints are gated behind ADMIN in internal/http/router.go.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS roles (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role_name   TEXT NOT NULL UNIQUE CHECK (role_name IN ('STUDENT', 'FACULTY', 'COORDINATOR', 'ADMIN')),
    description TEXT NOT NULL
);
-- Rows are seeded automatically at app startup (internal/platform/db/migrate.go).

CREATE TABLE IF NOT EXISTS users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    -- Nullable: a user who has only ever signed in via magic link has no
    -- Google account linked yet. Postgres UNIQUE allows multiple NULLs, so
    -- this doesn't collide across magic-link-only users. Signing in with
    -- Google later links it to the existing row by email (see
    -- AuthService.HandleOAuthCallback / UserRepository.LinkGoogleSub)
    -- instead of creating a second user.
    google_sub    TEXT UNIQUE,
    email         TEXT NOT NULL UNIQUE,
    display_name  TEXT NOT NULL,
    role_id       UUID NOT NULL REFERENCES roles (id) ON DELETE RESTRICT ON UPDATE CASCADE,
    -- Not set during login (neither Google nor magic link has a notion of
    -- it) - populated later via PATCH /api/admin/users/{userID}. Exists so
    -- the marksheet PDF can display a student's department
    -- (internal/platform/pdf/generator.go).
    department    TEXT,
    last_login_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_users_email_lower CHECK (email = lower(email))
);

CREATE INDEX IF NOT EXISTS idx_users_role_id ON users (role_id);

-- ---------------------------------------------------------------------------
-- Magic-link sign-in tokens (internal/domain/magic_link.go). Same hashed-
-- token pattern as industry_access_tokens below: only the SHA-256 hash is
-- stored, never the raw token that goes out in the email.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS magic_link_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email      TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_mlt_email ON magic_link_tokens (email);
CREATE INDEX IF NOT EXISTS idx_mlt_expires_at ON magic_link_tokens (expires_at);

CREATE TABLE IF NOT EXISTS revoked_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    jti        TEXT NOT NULL UNIQUE,
    user_id    UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE ON UPDATE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_revoked_tokens_user_id ON revoked_tokens (user_id);
CREATE INDEX IF NOT EXISTS idx_revoked_tokens_expires_at ON revoked_tokens (expires_at);

-- ---------------------------------------------------------------------------
-- Internships + faculty mentor assignment (internal/domain/internship.go)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS internships (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    student_id            UUID NOT NULL UNIQUE REFERENCES users (id) ON DELETE RESTRICT ON UPDATE CASCADE,
    company_name          TEXT NOT NULL,
    company_address       TEXT,
    role_title            TEXT,
    industry_mentor_name  TEXT,
    industry_mentor_email TEXT NOT NULL,
    academic_year         TEXT,
    start_date            DATE NOT NULL,
    end_date              DATE NOT NULL,
    status                TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed', 'cancelled')),
    created_by            UUID REFERENCES users (id) ON DELETE SET NULL ON UPDATE CASCADE,
    updated_by            UUID REFERENCES users (id) ON DELETE SET NULL ON UPDATE CASCADE,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_internships_dates CHECK (end_date > start_date)
);
-- NOTE: student_id is a plain UNIQUE constraint here because
-- domain.Internship.StudentID is tagged gorm:"uniqueIndex" (one row per
-- student, full stop) - it does not vary by status. If the intent is really
-- "one *active* internship at a time, but a student can re-enroll after
-- completing one", relax this to a partial unique index
-- (`... WHERE status = 'active'`) AND drop/relax the Go tag to match, since
-- right now the Go tag itself forbids ever creating a second row for the
-- same student even after the first is completed or cancelled.

CREATE OR REPLACE TRIGGER trg_internships_updated_at
    BEFORE UPDATE ON internships FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

CREATE INDEX IF NOT EXISTS idx_internships_status ON internships (status);

CREATE TABLE IF NOT EXISTS mentor_assignments (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    internship_id     UUID NOT NULL REFERENCES internships (id) ON DELETE CASCADE ON UPDATE CASCADE,
    faculty_mentor_id UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT ON UPDATE CASCADE,
    approved_at       TIMESTAMPTZ,
    approved_by       UUID REFERENCES users (id) ON DELETE SET NULL ON UPDATE CASCADE,
    status            TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected'))
);

CREATE INDEX IF NOT EXISTS idx_ma_internship_id ON mentor_assignments (internship_id);
CREATE INDEX IF NOT EXISTS idx_ma_faculty_mentor_id ON mentor_assignments (faculty_mentor_id);

-- ---------------------------------------------------------------------------
-- Reports: weekly / fortnightly / monthly (internal/domain/report.go's
-- WeeklyReport struct - despite the Go type name, this table holds all three
-- report cadences, distinguished by report_type).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS reports (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    internship_id    UUID NOT NULL REFERENCES internships (id) ON DELETE CASCADE ON UPDATE CASCADE,
    report_type      report_type NOT NULL DEFAULT 'weekly',
    week_number      INTEGER NOT NULL,
    content          TEXT NOT NULL,
    status           report_status NOT NULL DEFAULT 'submitted',
    submitted_at     TIMESTAMPTZ NOT NULL,
    edited_at        TIMESTAMPTZ NOT NULL,
    approved_at      TIMESTAMPTZ,
    approved_by      UUID REFERENCES users (id) ON DELETE SET NULL ON UPDATE CASCADE,
    reminder_sent_at TIMESTAMPTZ,
    created_by       UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT ON UPDATE CASCADE,
    updated_by       UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT ON UPDATE CASCADE,
    CONSTRAINT uq_reports_internship_type_period UNIQUE (internship_id, report_type, week_number),
    -- Per-type period ranges match domain.ReportType.MaxPeriod(): weekly 1-16,
    -- fortnightly 1-8, monthly 1-4.
    CONSTRAINT chk_reports_period CHECK (
        (report_type = 'weekly'      AND week_number BETWEEN 1 AND 16) OR
        (report_type = 'fortnightly' AND week_number BETWEEN 1 AND 8)  OR
        (report_type = 'monthly'     AND week_number BETWEEN 1 AND 4)
    )
);

CREATE INDEX IF NOT EXISTS idx_reports_status ON reports (status);

-- ---------------------------------------------------------------------------
-- Feedback (internal/domain/report.go's ReportFeedback struct maps here) +
-- per-report industry review tokens (internal/domain/token.go).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS feedback (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id      UUID NOT NULL REFERENCES reports (id) ON DELETE CASCADE ON UPDATE CASCADE,
    source         feedback_source NOT NULL,
    given_by       UUID REFERENCES users (id) ON DELETE RESTRICT ON UPDATE CASCADE,
    industry_email TEXT,
    comments       TEXT NOT NULL,
    rating         INTEGER,
    submitted_at   TIMESTAMPTZ NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_feedback_rating CHECK (rating IS NULL OR rating BETWEEN 1 AND 5)
);

CREATE INDEX IF NOT EXISTS idx_feedback_report_id ON feedback (report_id);

CREATE TABLE IF NOT EXISTS industry_access_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id  UUID NOT NULL REFERENCES reports (id) ON DELETE CASCADE ON UPDATE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_iat_report_id ON industry_access_tokens (report_id);
CREATE INDEX IF NOT EXISTS idx_iat_expires_at ON industry_access_tokens (expires_at);

-- ---------------------------------------------------------------------------
-- Evaluations: scheduling, fixed-rubric scoring, corrections
-- (internal/domain/evaluation.go). Two separate concepts, two separate
-- tables - EvaluationSchedule (venue/dates) and EvaluationScore (the
-- fixed 5-criterion rubric that sums to 100: 20+30+15+15+20). Each is now
-- scoped per (internship_id, exam_type): an internship gets at most one
-- schedule and one score per exam type (ISE, ESE).
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS evaluation_schedules (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    internship_id   UUID NOT NULL REFERENCES internships (id) ON DELETE CASCADE ON UPDATE CASCADE,
    exam_type       TEXT NOT NULL CHECK (exam_type IN ('ISE', 'ESE')),
    in_semester_at  DATE NOT NULL,
    end_semester_at DATE NOT NULL,
    venue           VARCHAR(255) NOT NULL DEFAULT '',
    set_by          UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT ON UPDATE CASCADE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_es_internship_examtype UNIQUE (internship_id, exam_type)
);

CREATE OR REPLACE TRIGGER trg_evaluation_schedules_updated_at
    BEFORE UPDATE ON evaluation_schedules FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

CREATE INDEX IF NOT EXISTS idx_es_internship_id ON evaluation_schedules (internship_id);

CREATE TABLE IF NOT EXISTS evaluation_scores (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    internship_id        UUID NOT NULL REFERENCES internships (id) ON DELETE CASCADE ON UPDATE CASCADE,
    exam_type            TEXT NOT NULL CHECK (exam_type IN ('ISE', 'ESE')),
    report_quality       INTEGER NOT NULL DEFAULT 0 CHECK (report_quality BETWEEN 0 AND 20),
    oral_presentation    INTEGER NOT NULL DEFAULT 0 CHECK (oral_presentation BETWEEN 0 AND 30),
    work_quality         INTEGER NOT NULL DEFAULT 0 CHECK (work_quality BETWEEN 0 AND 15),
    understanding        INTEGER NOT NULL DEFAULT 0 CHECK (understanding BETWEEN 0 AND 15),
    periodic_interaction INTEGER NOT NULL DEFAULT 0 CHECK (periodic_interaction BETWEEN 0 AND 20),
    remarks              TEXT,
    locked_at            TIMESTAMPTZ,
    submitted_by         UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT ON UPDATE CASCADE,
    submitted_at         TIMESTAMPTZ NOT NULL,
    CONSTRAINT uq_ev_internship_examtype UNIQUE (internship_id, exam_type)
);

CREATE TABLE IF NOT EXISTS evaluation_corrections (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evaluation_score_id UUID NOT NULL REFERENCES evaluation_scores (id) ON DELETE RESTRICT ON UPDATE CASCADE,
    old_scores_json     JSONB NOT NULL,
    new_scores_json     JSONB NOT NULL,
    reason              TEXT NOT NULL,
    corrected_by        UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT ON UPDATE CASCADE,
    corrected_at        TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_ec_evaluation_score_id ON evaluation_corrections (evaluation_score_id);

-- ---------------------------------------------------------------------------
-- Marksheets: generated PDF per evaluation score (internal/domain/marksheet.go)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS marksheets (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    evaluation_score_id UUID NOT NULL UNIQUE REFERENCES evaluation_scores (id) ON DELETE CASCADE ON UPDATE CASCADE,
    file_key            TEXT NOT NULL,
    generated_at        TIMESTAMPTZ NOT NULL,
    generated_by        UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT ON UPDATE CASCADE
);

-- ---------------------------------------------------------------------------
-- Email outbox (internal/domain/outbox.go maps this to "email_delivery_requests",
-- PK column "email_request_id"). Rows are created by NotificationService and
-- drained by internal/jobs/outbox_dispatcher.go, which sends each one over
-- SMTP (internal/platform/mailer) and retries on failure with backoff.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS email_delivery_requests (
    email_request_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    recipient        TEXT NOT NULL,
    subject          TEXT NOT NULL,
    body_html        TEXT NOT NULL,
    template_key     TEXT NOT NULL DEFAULT 'general',
    template_data    JSONB NOT NULL DEFAULT '{}'::jsonb,
    status           TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'sent', 'failed')),
    attempts         INTEGER NOT NULL DEFAULT 0,
    next_attempt_at  TIMESTAMPTZ NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE OR REPLACE TRIGGER trg_email_delivery_requests_updated_at
    BEFORE UPDATE ON email_delivery_requests FOR EACH ROW EXECUTE FUNCTION fn_set_updated_at();

CREATE INDEX IF NOT EXISTS idx_edr_pending ON email_delivery_requests (next_attempt_at ASC) WHERE status = 'pending';

-- ---------------------------------------------------------------------------
-- Audit log (internal/domain/audit.go). resource_id is intentionally not a
-- foreign key: it is polymorphic (resource_type names which table it refers
-- to - "users", "internships", etc.), matching AuditLog having no
-- association pointer for it.
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS audit_logs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id UUID NOT NULL REFERENCES users (id) ON DELETE RESTRICT ON UPDATE CASCADE,
    actor_name    TEXT NOT NULL,
    action        TEXT NOT NULL,
    resource_type TEXT NOT NULL,
    resource_id   UUID,
    metadata_json JSONB,
    created_at    TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_al_actor_user_id ON audit_logs (actor_user_id);
CREATE INDEX IF NOT EXISTS idx_al_resource_id ON audit_logs (resource_id);
CREATE INDEX IF NOT EXISTS idx_al_created_at ON audit_logs (created_at DESC);

COMMIT;
