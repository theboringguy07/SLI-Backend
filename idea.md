# SLI Backend Server Architecture

SLI Backend Server is a production-ready Go API for managing 16-week student internship reporting, mentor feedback, evaluation scheduling, locked evaluation marks, and generated marksheet PDFs.

## Goals

- Build a scalable backend API using Go, `net/http`, Chi, and GORM.
- Keep the architecture simple enough for another developer to implement.
- Use clear role-based access for Student, Faculty Mentor, Industry Mentor, Internship Coordinator, HOD, and Super Admin.
- Store all submissions, feedback, marks, credentials, and timestamps reliably.
- Store actor identity, role, OAuth subject, and audit metadata. Do not store Google passwords or raw OAuth access credentials.
- Send email reminders and secure industry mentor review links from the backend.
- Generate downloadable marksheet PDFs after evaluation scores are submitted.

## High-Level Architecture

```
                                        +----------------------+
                                        |   Google Workspace   |
                                        |   OAuth / OIDC       |
                                        +----------+-----------+
                                                   |
                                                   v
+------------------+      HTTPS       +------------+-------------+
| Web / Mobile UI  +-----------------> | Go Backend Server       |
| Student/Faculty  |                  | net/http + Chi Router   |
| Coordinator/HOD  |                  +------------+-------------+
+------------------+                               |
                                                   |
                                +------------------+------------------+
                                |                                     |
                                v                                     v
                    +-----------+-----------+             +-----------+-----------+
                    | HTTP Middleware       |             | Public Token Routes   |
                    | Request ID            |             | Industry Mentor Link  |
                    | Recoverer             |             | 24-hour access only   |
                    | Logger                |             +-----------+-----------+
                    | Auth                  |                         |
                    | RBAC                  |                         v
                    | Rate Limit            |             +-----------+-----------+
                    +-----------+-----------+             | Report Feedback       |
                                |                         | Service               |
                                v                         +-----------+-----------+
                    +-----------+-----------+                         |
                    | Route Handlers        |                         |
                    | Thin HTTP layer       |                         |
                    +-----------+-----------+                         |
                                |                                     |
                                v                                     |
                    +-----------+-----------+                         |
                    | Application Services  |<------------------------+
                    | Auth                  |
                    | Internship            |
                    | Weekly Reports        |
                    | Feedback              |
                    | Evaluation            |
                    | Marksheet PDF         |
                    | Notifications         |
                    +-----------+-----------+
                                |
                                v
                    +-----------+-----------+
                    | Repositories          |
                    | GORM data access      |
                    +-----------+-----------+
                                |
                                v
                    +-----------+-----------+
                    | PostgreSQL Database   |
                    | relational source of  |
                    | truth                 |
                    +-----------+-----------+
                                |
              +-----------------+-----------------+
              |                                   |
              v                                   v
  +-----------+-----------+           +-----------+-----------+
  | Object Storage        |           | Audit / Outbox Tables |
  | Marksheet PDFs        |           | reliable side effects |
  +-----------------------+           +-----------------------+

External services:

+----------------------+     +----------------------+     +----------------------+
| SMTP / Email Provider|     | Observability Stack  |     | Backup / Migration   |
| reminders and links  |     | logs, metrics, traces|     | database safety      |
+----------------------+     +----------------------+     +----------------------+
```

## Internal Go Architecture

```
cmd/server/main.go
        |
        v
internal/config
        |
        v
internal/server
        |
        +--> internal/http/router.go
        |           |
        |           +--> Chi route groups
        |           +--> Middleware chain
        |
        +--> internal/handlers
        |           |
        |           +--> auth_handler.go
        |           +--> student_handler.go
        |           +--> faculty_handler.go
        |           +--> coordinator_handler.go
        |           +--> industry_handler.go
        |           +--> hod_handler.go
        |           +--> admin_handler.go
        |
        +--> internal/services
        |           |
        |           +--> auth_service.go
        |           +--> internship_service.go
        |           +--> report_service.go
        |           +--> feedback_service.go
        |           +--> evaluation_service.go
        |           +--> marksheet_service.go
        |           +--> notification_service.go
        |
        +--> internal/repositories
        |           |
        |           +--> GORM-backed repository interfaces
        |
        +--> internal/domain
        |           |
        |           +--> User, Internship, WeeklyReport
        |           +--> Feedback, Evaluation, Marksheet
        |
        +--> internal/jobs
        |           |
        |           +--> reminder_worker.go
        |           +--> outbox_dispatcher.go
        |           +--> token_cleanup_worker.go
        |
        +--> internal/platform
                    |
                    +--> db
                    +--> mailer
                    +--> pdf
                    +--> logger
                    +--> auth
                    +--> errors
```

## Request Lifecycle

```
Client Request
      |
      v
+-----+------+
| Chi Router |
+-----+------+
      |
      v
+-----+---------------------------------------------------------------+
| Middleware: request_id -> recoverer -> logger -> auth -> RBAC       |
+-----+---------------------------------------------------------------+
      |
      v
+-----+------+
| Handler    |  Parse JSON, validate input, call service, encode JSON.
+-----+------+
      |
      v
+-----+------+
| Service    |  Business rules, authorization details, transactions.
+-----+------+
      |
      v
+-----+------+
| Repository |  GORM queries only. No HTTP logic.
+-----+------+
      |
      v
+-----+------+
| Database   |
+------------+
```

## Weekly Report Flow

```
Student
  |
  | POST /api/student/reports/week/{week}
  v
Go API
  |
  | Validate:
  | - authenticated student
  | - week is between 1 and 16
  | - student has active internship
  | - report is within submission/edit rules
  v
Report Service
  |
  | Transaction:
  | - create/update weekly_reports
  | - store submitted_at / edited_at
  | - store created_by / updated_by
  | - enqueue faculty notification
  | - generate industry mentor review token
  | - enqueue industry mentor email
  v
PostgreSQL + Outbox
  |
  v
Email Worker
  |
  +--> Faculty Mentor: report submitted notification
  |
  +--> Industry Mentor: 24-hour review URL
```

## Industry Mentor Feedback Flow

```
Industry Mentor Email
        |
        | Secure URL:
        | /industry/reports/{token}
        v
+-------+--------+
| Public Token   |
| Chi Route      |
+-------+--------+
        |
        v
+-------+---------------------------------------------------+
| Validate token hash, expiry, scope, and used/disabled flag |
+-------+---------------------------------------------------+
        |
        v
+-------+--------+       POST feedback       +----------------------+
| View Report    +-------------------------> | Feedback Service     |
+----------------+                           +----------+-----------+
                                                        |
                                                        v
                                             +----------+-----------+
                                             | report_feedback      |
                                             | actor=industry_link  |
                                             | exact submitted_at   |
                                             +----------------------+
```

## Evaluation And Marksheet Flow

```
Faculty Mentor
      |
      | Set dates:
      | POST /api/faculty/students/{studentID}/evaluations/schedule
      v
Evaluation Service
      |
      +--> Save in-semester and end-semester dates
      +--> Enqueue student email notification


Faculty Mentor
      |
      | Submit final marks:
      | POST /api/faculty/students/{studentID}/evaluations
      v
Evaluation Service
      |
      | Validate:
      | - faculty owns approved student assignment
      | - all required marks are present
      | - marks are not already locked
      v
Database Transaction
      |
      +--> Insert evaluation_scores
      +--> Mark score as locked
      +--> Insert audit log
      +--> Enqueue PDF generation
      v
PDF Worker
      |
      +--> Generate marksheet PDF
      +--> Store PDF metadata
      +--> Faculty can download PDF
```

## Suggested API Route Groups

```
/api/auth
  GET    /google/login
  GET    /google/callback
  POST   /logout
  GET    /me

/api/student
  GET    /dashboard
  GET    /internship
  GET    /reports
  POST   /reports/week/{week}
  PUT    /reports/week/{week}
  GET    /feedback
  GET    /evaluations
  GET    /marksheet

/api/faculty
  GET    /students
  POST   /students/{studentID}/approve
  GET    /students/{studentID}/reports
  POST   /reports/{reportID}/feedback
  POST   /students/{studentID}/evaluations/schedule
  POST   /students/{studentID}/evaluations
  GET    /students/{studentID}/marksheet

/api/coordinator
  GET    /students
  POST   /students
  POST   /students/{studentID}/assign-faculty
  GET    /mentors
  GET    /overview

/api/hod
  GET    /stats
  GET    /reports/completion
  GET    /evaluations/progress
  GET    /mentors/load

/api/admin
  GET    /users
  POST   /users/{userID}/roles
  POST   /internships/{internshipID}/repair
  GET    /audit-logs

/industry
  GET    /reports/{token}
  POST   /reports/{token}/feedback

/healthz
/readyz
```

## Core Data Model

```
+-----------------------+       +-----------------------+
| users                 |       | roles                 |
| id                    |       | id                    |
| google_sub            |       | name                  |
| email                 |       +-----------------------+
| name                  |                  ^
| domain                |                  |
| created_at            |       +----------+------------+
| updated_at            |       | user_roles            |
+-----------+-----------+       | user_id               |
            |                   | role_id               |
            |                   +-----------------------+
            |
            v
+-----------+-----------+       +-----------------------+
| internships           |       | mentor_assignments    |
| id                    |<------+ id                    |
| student_id            |       | internship_id         |
| company_name          |       | faculty_mentor_id     |
| industry_mentor_email |       | approved_at           |
| start_date            |       | approved_by           |
| end_date              |       | status                |
| status                |       +-----------------------+
+-----------+-----------+
            |
            v
+-----------+-----------+       +-----------------------+
| weekly_reports        |<------+ report_feedback       |
| id                    |       | id                    |
| internship_id         |       | report_id             |
| week_number           |       | reviewer_type         |
| content               |       | reviewer_user_id      |
| submitted_at          |       | industry_email        |
| edited_at             |       | feedback_text         |
| created_by            |       | submitted_at          |
| updated_by            |       | created_at            |
+-----------+-----------+       +-----------------------+
            |
            v
+-----------+-----------+       +-----------------------+
| industry_access_tokens|       | evaluation_schedules  |
| id                    |       | id                    |
| report_id             |       | internship_id         |
| token_hash            |       | in_semester_at        |
| expires_at            |       | end_semester_at       |
| used_at               |       | set_by                |
| created_at            |       | created_at            |
+-----------------------+       +-----------------------+

+-----------------------+       +-----------------------+
| evaluation_scores     |       | marksheets            |
| id                    |       | id                    |
| internship_id         |       | evaluation_score_id   |
| report_quality        |       | file_key              |
| oral_presentation     |       | generated_at          |
| work_quality          |       | generated_by          |
| understanding         |       +-----------------------+
| periodic_interaction  |
| remarks               |       +-----------------------+
| locked_at             |       | audit_logs            |
| submitted_by          |       | id                    |
| submitted_at          |       | actor_user_id         |
+-----------------------+       | action                |
                                | resource_type         |
+-----------------------+       | resource_id           |
| email_outbox          |       | metadata_json         |
| id                    |       | created_at            |
| recipient             |       +-----------------------+
| subject               |
| body                  |
| status                |
| attempts              |
| next_attempt_at       |
+-----------------------+
```

## Recommended Backend Rules

- Use Chi only for routing and middleware composition.
- Use the standard Go `net/http` request and response model.
- Keep handlers thin. Business rules belong in services.
- Keep GORM usage inside repositories.
- Use database transactions for report submission, feedback writes, evaluation lock, audit log writes, and outbox enqueueing.
- Use `context.Context` across handlers, services, repositories, mailer, and PDF generator.
- Use structured logging with request ID and actor ID.
- Return consistent JSON errors.
- Never leak internal errors directly to clients.
- Never store Google account passwords. Persist only the Google OAuth subject, verified email, domain, role, and session metadata.
- Use migrations for every schema change.
- Use environment variables for configuration.
- Use health checks for uptime and readiness checks for database connectivity.

## Error Response Shape

```json
{
  "error": {
    "code": "REPORT_ALREADY_LOCKED",
    "message": "This report can no longer be edited.",
    "request_id": "req_01HZ..."
  }
}
```

## Production Checklist

```
+--------------------------+--------------------------------------------------+
| Area                     | Requirement                                      |
+--------------------------+--------------------------------------------------+
| Auth                     | Google OAuth/OIDC with allowed domain check.     |
| Authorization            | RBAC plus resource ownership checks.             |
| Database                 | PostgreSQL, GORM, migrations, backups.           |
| Logging                  | Structured logs with request ID.                 |
| Errors                   | Typed app errors mapped to HTTP status codes.    |
| Email                    | Outbox pattern with retries.                     |
| PDF                      | Async generation and durable file storage.       |
| Security                 | Hashed tokens, expiry, rate limits, CORS.        |
| Observability            | Logs, metrics, traces, health checks.            |
| Tests                    | Unit, integration, auth, service, handler tests. |
| Deployment               | Docker image, env config, migration command.     |
+--------------------------+--------------------------------------------------+
```

## Suggested Implementation Phases

```
Phase 1: Project foundation
  - Go module
  - Chi router
  - config loader
  - logger
  - health routes
  - error response package

Phase 2: Auth and users
  - Google OAuth/OIDC
  - allowed domain validation
  - session or JWT flow
  - users and roles schema

Phase 3: Internship mapping
  - student enrollment
  - coordinator assignment
  - faculty approval
  - resource ownership rules

Phase 4: Weekly reports
  - 16-week report model
  - submit and edit flow
  - faculty feedback
  - timestamp and actor audit fields

Phase 5: Industry mentor links
  - secure token generation
  - hashed token storage
  - 24-hour expiry
  - public report view and feedback route

Phase 6: Notifications
  - email outbox
  - reminder worker
  - report submitted emails
  - evaluation date emails

Phase 7: Evaluation and marksheet
  - schedule dates
  - submit locked marks
  - audit log
  - PDF generation and download

Phase 8: Dashboards and admin
  - coordinator overview
  - HOD statistics
  - Super Admin repair tools
  - audit log viewer

Phase 9: Production hardening
  - integration tests
  - rate limits
  - metrics
  - backups
  - deployment pipeline
```
