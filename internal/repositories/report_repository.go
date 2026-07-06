package repositories

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sli/backend/internal/domain"
)

var (
	ErrReportNotFound = errors.New("weekly report not found")
)

type ReportRepository interface {
	Create(ctx context.Context, report *domain.WeeklyReport) error
	Update(ctx context.Context, report *domain.WeeklyReport) error
	FindByInternshipTypeAndPeriod(ctx context.Context, internshipID uuid.UUID, reportType domain.ReportType, period int) (*domain.WeeklyReport, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.WeeklyReport, error)
	ListByInternship(ctx context.Context, internshipID uuid.UUID) ([]domain.WeeklyReport, error)
	// ListAll is the coordinator-facing view across every internship's
	// reports (with Internship + Internship.Student joined in, so the UI can
	// show who each report belongs to without N+1 lookups).
	ListAll(ctx context.Context, offset, limit int) ([]domain.WeeklyReport, int64, error)
	// ListAllByDepartment is the department-scoped equivalent of ListAll -
	// a coordinator should only see reports from students in their own
	// department (see ReportService.ListAllReportsForUser).
	ListAllByDepartment(ctx context.Context, department string, offset, limit int) ([]domain.WeeklyReport, int64, error)
	RunInTransaction(ctx context.Context, fn func(txRepo ReportRepository) error) error
}

type FeedbackRepository interface {
	Create(ctx context.Context, feedback *domain.ReportFeedback) error
	ListByReport(ctx context.Context, reportID uuid.UUID) ([]domain.ReportFeedback, error)
}

type reportRepository struct {
	db dbtx
}

func NewReportRepository(db dbtx) ReportRepository {
	return &reportRepository{db: db}
}

const reportSelectCols = `
	id, internship_id, report_type, week_number, content, status, submitted_at, edited_at,
	approved_at, approved_by, reminder_sent_at, created_by, updated_by`

func scanReport(row pgx.Row) (*domain.WeeklyReport, error) {
	var rpt domain.WeeklyReport
	err := row.Scan(
		&rpt.ID, &rpt.InternshipID, &rpt.ReportType, &rpt.WeekNumber, &rpt.Content, &rpt.Status,
		&rpt.SubmittedAt, &rpt.EditedAt, &rpt.ApprovedAt, &rpt.ApprovedBy, &rpt.ReminderSentAt,
		&rpt.CreatedBy, &rpt.UpdatedBy,
	)
	if err != nil {
		return nil, err
	}
	return &rpt, nil
}

func (r *reportRepository) Create(ctx context.Context, report *domain.WeeklyReport) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO reports (
			internship_id, report_type, week_number, content, status, submitted_at, edited_at, created_by, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`,
		report.InternshipID, report.ReportType, report.WeekNumber, report.Content, report.Status,
		report.SubmittedAt, report.EditedAt, report.CreatedBy, report.UpdatedBy,
	).Scan(&report.ID)
}

func (r *reportRepository) Update(ctx context.Context, report *domain.WeeklyReport) error {
	_, err := r.db.Exec(ctx, `
		UPDATE reports SET
			content = $1, status = $2, edited_at = $3, approved_at = $4, approved_by = $5,
			reminder_sent_at = $6, updated_by = $7
		WHERE id = $8`,
		report.Content, report.Status, report.EditedAt, report.ApprovedAt, report.ApprovedBy,
		report.ReminderSentAt, report.UpdatedBy, report.ID,
	)
	return err
}

func (r *reportRepository) FindByInternshipTypeAndPeriod(ctx context.Context, internshipID uuid.UUID, reportType domain.ReportType, period int) (*domain.WeeklyReport, error) {
	row := r.db.QueryRow(ctx, `SELECT `+reportSelectCols+` FROM reports
		WHERE internship_id = $1 AND report_type = $2 AND week_number = $3`, internshipID, reportType, period)
	report, err := scanReport(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReportNotFound
		}
		return nil, err
	}
	return report, nil
}

func (r *reportRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.WeeklyReport, error) {
	row := r.db.QueryRow(ctx, `SELECT `+reportSelectCols+` FROM reports WHERE id = $1`, id)
	report, err := scanReport(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrReportNotFound
		}
		return nil, err
	}
	return report, nil
}

func (r *reportRepository) ListByInternship(ctx context.Context, internshipID uuid.UUID) ([]domain.WeeklyReport, error) {
	rows, err := r.db.Query(ctx, `SELECT `+reportSelectCols+` FROM reports
		WHERE internship_id = $1 ORDER BY week_number ASC`, internshipID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reports := []domain.WeeklyReport{}
	for rows.Next() {
		report, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		reports = append(reports, *report)
	}
	return reports, rows.Err()
}

func (r *reportRepository) ListAll(ctx context.Context, offset, limit int) ([]domain.WeeklyReport, int64, error) {
	var count int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM reports`).Scan(&count); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT `+reportCols("rp")+`, `+internshipSelectCols+`
		FROM reports rp
		JOIN internships i ON i.id = rp.internship_id
		JOIN users s ON s.id = i.student_id
		ORDER BY rp.submitted_at DESC
		OFFSET $1 LIMIT $2`, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	reports := []domain.WeeklyReport{}
	for rows.Next() {
		var rpt domain.WeeklyReport
		var in domain.Internship
		var student domain.User
		var department *string
		err := rows.Scan(
			&rpt.ID, &rpt.InternshipID, &rpt.ReportType, &rpt.WeekNumber, &rpt.Content, &rpt.Status,
			&rpt.SubmittedAt, &rpt.EditedAt, &rpt.ApprovedAt, &rpt.ApprovedBy, &rpt.ReminderSentAt,
			&rpt.CreatedBy, &rpt.UpdatedBy,
			&in.ID, &in.StudentID, &in.CompanyName, &in.CompanyAddress, &in.RoleTitle, &in.IndustryMentorName,
			&in.IndustryMentorEmail, &in.AcademicYear, &in.StartDate, &in.EndDate, &in.Status, &in.CreatedBy, &in.UpdatedBy,
			&in.CreatedAt, &in.UpdatedAt,
			&student.ID, &student.GoogleSub, &student.Email, &student.Name, &student.RoleID, &department, &student.CreatedAt, &student.LastLoginAt,
		)
		if err != nil {
			return nil, 0, err
		}
		if department != nil {
			student.Department = *department
		}
		in.Student = &student
		rpt.Internship = &in
		reports = append(reports, rpt)
	}
	return reports, count, rows.Err()
}

func (r *reportRepository) ListAllByDepartment(ctx context.Context, department string, offset, limit int) ([]domain.WeeklyReport, int64, error) {
	var count int64
	if err := r.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM reports rp
		JOIN internships i ON i.id = rp.internship_id
		JOIN users s ON s.id = i.student_id
		WHERE s.department = $1`, department).Scan(&count); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT `+reportCols("rp")+`, `+internshipSelectCols+`
		FROM reports rp
		JOIN internships i ON i.id = rp.internship_id
		JOIN users s ON s.id = i.student_id
		WHERE s.department = $1
		ORDER BY rp.submitted_at DESC
		OFFSET $2 LIMIT $3`, department, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	reports := []domain.WeeklyReport{}
	for rows.Next() {
		var rpt domain.WeeklyReport
		var in domain.Internship
		var student domain.User
		var dept *string
		err := rows.Scan(
			&rpt.ID, &rpt.InternshipID, &rpt.ReportType, &rpt.WeekNumber, &rpt.Content, &rpt.Status,
			&rpt.SubmittedAt, &rpt.EditedAt, &rpt.ApprovedAt, &rpt.ApprovedBy, &rpt.ReminderSentAt,
			&rpt.CreatedBy, &rpt.UpdatedBy,
			&in.ID, &in.StudentID, &in.CompanyName, &in.CompanyAddress, &in.RoleTitle, &in.IndustryMentorName,
			&in.IndustryMentorEmail, &in.AcademicYear, &in.StartDate, &in.EndDate, &in.Status, &in.CreatedBy, &in.UpdatedBy,
			&in.CreatedAt, &in.UpdatedAt,
			&student.ID, &student.GoogleSub, &student.Email, &student.Name, &student.RoleID, &dept, &student.CreatedAt, &student.LastLoginAt,
		)
		if err != nil {
			return nil, 0, err
		}
		if dept != nil {
			student.Department = *dept
		}
		in.Student = &student
		rpt.Internship = &in
		reports = append(reports, rpt)
	}
	return reports, count, rows.Err()
}

// reportCols prefixes reportSelectCols' bare column names with a table
// alias, since ListAll's query joins reports against internships/users and
// needs to disambiguate.
func reportCols(alias string) string {
	cols := []string{
		"id", "internship_id", "report_type", "week_number", "content", "status", "submitted_at", "edited_at",
		"approved_at", "approved_by", "reminder_sent_at", "created_by", "updated_by",
	}
	out := ""
	for i, c := range cols {
		if i > 0 {
			out += ", "
		}
		out += alias + "." + c
	}
	return out
}

func (r *reportRepository) RunInTransaction(ctx context.Context, fn func(txRepo ReportRepository) error) error {
	// r.db satisfies txBeginner whether it's the pool (starts a real
	// transaction) or a pgx.Tx from an outer RunInTransaction (Begin on a
	// Tx opens a SAVEPOINT - Postgres nests transactions that way, and
	// pgx.Tx.Commit/Rollback on it becomes RELEASE/ROLLBACK TO SAVEPOINT).
	// Either way this "just works"; the only repo that wouldn't satisfy
	// txBeginner at all is one built directly on a raw non-transactional
	// connection, which doesn't happen in this codebase.
	beginner, ok := r.db.(txBeginner)
	if !ok {
		return fn(r)
	}

	tx, err := beginner.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op if already committed

	txRepo := NewReportRepository(tx)
	if err := fn(txRepo); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

type feedbackRepository struct {
	db dbtx
}

func NewFeedbackRepository(db dbtx) FeedbackRepository {
	return &feedbackRepository{db: db}
}

func (r *feedbackRepository) Create(ctx context.Context, feedback *domain.ReportFeedback) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO feedback (report_id, source, given_by, industry_email, comments, rating, submitted_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at`,
		feedback.ReportID, feedback.Source, feedback.GivenBy, feedback.IndustryEmail, feedback.Comments,
		feedback.Rating, feedback.SubmittedAt,
	).Scan(&feedback.ID, &feedback.CreatedAt)
}

func (r *feedbackRepository) ListByReport(ctx context.Context, reportID uuid.UUID) ([]domain.ReportFeedback, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, report_id, source, given_by, industry_email, comments, rating, submitted_at, created_at
		FROM feedback WHERE report_id = $1 ORDER BY submitted_at ASC`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	feedbacks := []domain.ReportFeedback{}
	for rows.Next() {
		var f domain.ReportFeedback
		if err := rows.Scan(&f.ID, &f.ReportID, &f.Source, &f.GivenBy, &f.IndustryEmail, &f.Comments, &f.Rating, &f.SubmittedAt, &f.CreatedAt); err != nil {
			return nil, err
		}
		feedbacks = append(feedbacks, f)
	}
	return feedbacks, rows.Err()
}
