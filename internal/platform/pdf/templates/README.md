# Marksheet PDF template

Drop your real files in here, replacing the starter ones:

- `marksheet.html` — the template itself. Uses Go's `html/template` syntax
  (`{{.FieldName}}`, not Mustache `{{field_name}}`) — see the field list below.
- Your two logo files (SVG or PNG) — name them anything, then reference them
  from `marksheet.html` with a plain relative path, e.g. `<img src="logo-left.svg">`.
  Whatever files you put in this directory get embedded into the Go binary
  (`//go:embed templates` in internal/platform/pdf/generator.go) and copied
  alongside the filled-in HTML at render time, so relative paths just work.

**You must rebuild the Go binary after changing anything in this folder** —
`go:embed` embeds these files at compile time, not at runtime.

## Available template fields

These map to `pdf.MarksheetData` in `internal/platform/pdf/generator.go`:

```
{{.StudentName}}              internship.Student.Name
{{.StudentEmail}}             internship.Student.Email
{{.DepartmentName}}           internship.Student.Department (users.department)
{{.CompanyName}}              internship.CompanyName
{{.RoleTitle}}                internship.RoleTitle
{{.FacultyMentorName}}        user who submitted the score
{{.ExamType}}                 "ISE" or "ESE" (evaluation_scores.exam_type)
{{.EvaluationDate}}           evaluation_schedules.end_semester_at, formatted "2 January 2006"
{{.Venue}}                    evaluation_schedules.venue
{{.ReportQuality}}            out of 20
{{.OralPresentation}}         out of 30
{{.WorkQuality}}              out of 15
{{.Understanding}}            out of 15
{{.PeriodicInteraction}}      out of 20
{{.ReportQualityBucket}}      "excellent"|"good"|"average"|"satisfactory" - which
{{.OralPresentationBucket}}   column gets the bullet point, computed from the
{{.WorkQualityBucket}}        score's percentage of its own max (>=90% excellent,
{{.UnderstandingBucket}}      70-89% good, 50-69% average, <50% satisfactory).
{{.PeriodicInteractionBucket}}
{{.TotalMarks}}               sum of the five scores above, out of 100
{{.Remarks}}                  evaluation_scores.remarks
{{.GeneratedAt}}              time of PDF generation, formatted "2 January 2006, 3:04 PM"
```

`department` is populated via `PATCH /api/admin/users/{userID}` (admin-only)
after a student's first login, since Google OAuth doesn't provide it.
`exam_type` is set per-schedule/per-score when faculty create the schedule or
submit marks (`ISE` or `ESE`).

## Rendering

`marksheet.html` is filled with real data via `html/template`, written to a
temp directory together with your logo files, then rendered by a real,
headless Chrome instance (`chromedp`) and printed to PDF - so full CSS
(flexbox, grid, `@page` rules, web fonts if you inline them) is supported,
same as what you'd see in Chrome's print preview. Use `@page { size: A4; }`
and `@media print` rules to control page size/margins.
