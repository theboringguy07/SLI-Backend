# SLI Backend Server Agents

This file defines the human actors, system agents, permissions, and non-negotiable access rules for the SLI Backend Server.

## Human Actors

```
+--------------------------+--------------------------------------------------+
| Actor                    | Purpose                                          |
+--------------------------+--------------------------------------------------+
| Student                  | Submits weekly internship reports for 16 weeks.  |
| Faculty Mentor           | Reviews assigned students and awards evaluation. |
| Industry Mentor          | Reviews a report through a 24-hour secure link.  |
| Internship Coordinator   | Enrolls students and maps faculty mentors.       |
| HOD                      | Views department-level statistics and overview.  |
| Super Admin              | Fixes configuration, accounts, and mappings.     |
+--------------------------+--------------------------------------------------+
```

## Permission Matrix

```
+------------------------+---------+---------+----------+-------------+-----+-------------+
| Capability             | Student | Faculty | Industry | Coordinator | HOD | Super Admin |
+------------------------+---------+---------+----------+-------------+-----+-------------+
| Login with college mail| Yes     | Yes     | No       | Yes         | Yes | Yes         |
| View own profile       | Yes     | Yes     | Limited  | Yes         | Yes | Yes         |
| Submit weekly report   | Yes     | No      | No       | No          | No  | Override    |
| Edit weekly report     | Yes     | No      | No       | No          | No  | Override    |
| Review weekly report   | No      | Yes     | Link     | View        | No  | Override    |
| Assign faculty mentor  | No      | No      | No       | Yes         | No  | Yes         |
| Approve assigned student| No     | Yes     | No       | View        | No  | Override    |
| Set evaluation dates   | No      | Yes     | No       | View        | View| Override    |
| Enter evaluation marks | No      | Yes     | No       | View        | View| Override    |
| Edit locked marks      | No      | No      | No       | No          | No  | Restricted  |
| Download marksheet PDF | Own     | Yes     | No       | View        | View| Yes         |
| View statistics        | No      | Own     | No       | All mapped  | Yes | Yes         |
| Fix misconfiguration   | No      | No      | No       | Limited     | No  | Yes         |
+------------------------+---------+---------+----------+-------------+-----+-------------+
```

## Actor Rules

### Student

- Authenticates with the approved college Google Workspace domain, for example `@somaiya.edu`.
- Can view only their own internship, mentor mappings, submissions, feedback, evaluation dates, and marksheets.
- Can submit one weekly report per internship week, from week `1` to week `16`.
- Can edit a report only while the configured edit window is open.
- Every submission and edit must store `submitted_at`, `edited_at`, `created_by`, and `updated_by`.
- Receives email reminders for pending weekly reports and scheduled evaluations.

### Faculty Mentor

- Authenticates with the approved college Google Workspace domain.
- Can see only students assigned to them and approved by them.
- Can review student weekly reports and provide feedback.
- Can set in-semester and end-semester evaluation dates for their assigned students.
- Can enter marks for report quality, oral presentation, work quality, understanding of work, periodic interaction, and remarks.
- Once evaluation marks are submitted, they become locked and cannot be edited through normal faculty flows.
- Can download computer-generated marksheet PDFs for assigned students.

### Industry Mentor

- Does not need a full platform account for normal weekly review.
- Receives a secure, single-purpose URL by email after the student submits a weekly report.
- The URL must expire after 24 hours.
- The URL must be scoped to exactly one weekly report and one student.
- Can view the report and submit feedback only while the token is valid.
- Cannot browse other reports, students, statistics, or system data.

### Internship Coordinator

- Authenticates with the approved college Google Workspace domain.
- Can enroll students into the internship cycle.
- Can assign students to faculty mentors.
- Can view all enrolled students and their assigned mentors.
- Can see mentor approval status, report completion status, feedback status, and evaluation status.
- Should not be able to edit locked faculty evaluation marks.

### HOD

- Authenticates with the approved college Google Workspace domain.
- Can view statistics and department-level overview.
- Can inspect aggregate status such as submission completion, pending reviews, mentor load, and evaluation progress.
- Should avoid direct operational edits except where explicitly approved by policy.

### Super Admin

- Authenticates with the approved college Google Workspace domain.
- Can fix account, role, assignment, domain, and configuration mistakes.
- Can override mappings and repair bad data through audited admin-only endpoints.
- Any destructive or sensitive action must be written to an audit log.
- Should not casually edit locked marks. If a correction path is required, it must create an audited correction record instead of silently mutating the original score.

## System Agents

```
+--------------------------+--------------------------------------------------+
| System Agent             | Responsibility                                   |
+--------------------------+--------------------------------------------------+
| Auth Middleware          | Validates session/JWT and college email domain.  |
| RBAC Middleware          | Enforces role and resource ownership checks.     |
| Reminder Worker          | Sends weekly submission reminder emails.         |
| Notification Worker      | Sends evaluation date and feedback notifications.|
| Industry Link Worker     | Creates and expires 24-hour mentor URLs.         |
| PDF Worker               | Generates downloadable marksheet PDFs.           |
| Audit Logger             | Records sensitive writes and admin overrides.    |
| Outbox Dispatcher        | Reliably sends queued emails/events.             |
+--------------------------+--------------------------------------------------+
```

## Backend Invariants

- All protected routes must pass authentication and authorization middleware.
- Resource ownership checks must happen on the server, never only in the frontend.
- Weekly reports are always scoped by `student_id`, `internship_id`, and `week_number`.
- Week numbers must be constrained to `1..16`.
- Industry mentor tokens must be hashed in the database, expire after 24 hours, and be single-scope.
- Evaluation scores are immutable after final submission.
- Corrections to locked marks must be modeled as audited corrections, not silent updates.
- Email sending should use an outbox table so failed sends can be retried safely.
- All important writes must store actor identity and timestamp.
- Server logs must include request ID, actor ID when available, route, status code, duration, and error code.

