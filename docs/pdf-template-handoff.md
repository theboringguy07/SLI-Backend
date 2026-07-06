# PDF Template Handoff

The backend currently generates a simple marksheet PDF in code. To replace it
with your college/client template, provide one of these:

1. A final `.docx` marksheet template.
2. A final `.html` + CSS template.
3. A PDF visual sample plus an editable source file.

Prefer `.docx` if the college already has an official marksheet format. Prefer
HTML/CSS if the template will be maintained by developers.

## Required Placeholder Fields

The template should clearly mark where these values go:

```text
student_name
student_email
department_name
company_name
role_title
faculty_mentor_name
exam_type
evaluation_date
venue
report_quality
oral_presentation
work_quality
understanding
periodic_interaction
total_marks
remarks
generated_at
```

If the final schema uses flexible `evaluation_marks`, provide a marks table
placeholder instead:

```text
marks_rows[]:
  criteria_name
  max_marks
  marks_obtained
  weightage
```

## Design Requirements

- Provide the exact page size: usually A4 portrait.
- Include official header/footer, logo, signatures, and seal placement.
- Mention whether signatures are text-only, scanned images, or uploaded files.
- Mention whether the PDF needs a QR code or verification ID.
- Provide a sample filled PDF if possible; it makes layout verification much easier.

## Suggested Template Syntax

For DOCX templates, use double braces:

```text
{{student_name}}
{{company_name}}
{{total_marks}}
```

For a marks table:

```text
{{#marks_rows}}
{{criteria_name}} | {{marks_obtained}} / {{max_marks}}
{{/marks_rows}}
```

## What Backend Will Do

The backend should:

- Load the template from a configured path.
- Merge the evaluation data into the template.
- Render a PDF.
- Store the generated PDF as a `documents` row with `document_type = 'marksheet'`.
- Never silently overwrite locked marks; corrections should generate a new audited PDF.
