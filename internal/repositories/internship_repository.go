package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sli/backend/internal/domain"
)

var (
	ErrInternshipNotFound = errors.New("internship not found")
	ErrAssignmentNotFound = errors.New("mentor assignment not found")
)

type InternshipRepository interface {
	Create(ctx context.Context, internship *domain.Internship) error
	FindByStudentID(ctx context.Context, studentID uuid.UUID) (*domain.Internship, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Internship, error)
	ListAll(ctx context.Context, offset, limit int) ([]domain.Internship, int64, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.InternshipStatus) error
}

type MentorAssignmentRepository interface {
	Create(ctx context.Context, assignment *domain.MentorAssignment) error
	FindByInternshipID(ctx context.Context, internshipID uuid.UUID) (*domain.MentorAssignment, error)
	ListByFacultyID(ctx context.Context, facultyID uuid.UUID) ([]domain.MentorAssignment, error)
	Approve(ctx context.Context, assignmentID uuid.UUID, facultyID uuid.UUID) error
}

type internshipRepository struct {
	db dbtx
}

func NewInternshipRepository(db dbtx) InternshipRepository {
	return &internshipRepository{db: db}
}

// internshipSelectCols joins in the student row (but not the student's role
// - nothing reads Internship.Student.Role, matching the previous
// Preload("Student") which likewise didn't cascade into Student.Role).
const internshipSelectCols = `
	i.id, i.student_id, i.company_name, i.company_address, i.role_title, i.industry_mentor_name,
	i.industry_mentor_email, i.academic_year, i.start_date, i.end_date, i.status, i.created_by, i.updated_by,
	i.created_at, i.updated_at,
	s.id, s.google_sub, s.email, s.display_name, s.role_id, s.department, s.created_at, s.last_login_at`

const internshipSelectFrom = `FROM internships i JOIN users s ON s.id = i.student_id`

func scanInternship(row pgx.Row) (*domain.Internship, error) {
	var in domain.Internship
	var student domain.User
	var department *string
	err := row.Scan(
		&in.ID, &in.StudentID, &in.CompanyName, &in.CompanyAddress, &in.RoleTitle, &in.IndustryMentorName,
		&in.IndustryMentorEmail, &in.AcademicYear, &in.StartDate, &in.EndDate, &in.Status, &in.CreatedBy, &in.UpdatedBy,
		&in.CreatedAt, &in.UpdatedAt,
		&student.ID, &student.GoogleSub, &student.Email, &student.Name, &student.RoleID, &department, &student.CreatedAt, &student.LastLoginAt,
	)
	if err != nil {
		return nil, err
	}
	if department != nil {
		student.Department = *department
	}
	in.Student = &student
	return &in, nil
}

func (r *internshipRepository) Create(ctx context.Context, internship *domain.Internship) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO internships (
			student_id, company_name, company_address, role_title, industry_mentor_name,
			industry_mentor_email, academic_year, start_date, end_date, status, created_by, updated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING id, created_at, updated_at`,
		internship.StudentID, internship.CompanyName, internship.CompanyAddress, internship.RoleTitle,
		internship.IndustryMentorName, internship.IndustryMentorEmail, internship.AcademicYear,
		internship.StartDate, internship.EndDate, internship.Status, internship.CreatedBy, internship.UpdatedBy,
	).Scan(&internship.ID, &internship.CreatedAt, &internship.UpdatedAt)
}

func (r *internshipRepository) FindByStudentID(ctx context.Context, studentID uuid.UUID) (*domain.Internship, error) {
	row := r.db.QueryRow(ctx, `SELECT `+internshipSelectCols+` `+internshipSelectFrom+` WHERE i.student_id = $1`, studentID)
	internship, err := scanInternship(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInternshipNotFound
		}
		return nil, err
	}
	return internship, nil
}

func (r *internshipRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Internship, error) {
	row := r.db.QueryRow(ctx, `SELECT `+internshipSelectCols+` `+internshipSelectFrom+` WHERE i.id = $1`, id)
	internship, err := scanInternship(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrInternshipNotFound
		}
		return nil, err
	}
	return internship, nil
}

func (r *internshipRepository) ListAll(ctx context.Context, offset, limit int) ([]domain.Internship, int64, error) {
	var count int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM internships`).Scan(&count); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `SELECT `+internshipSelectCols+` `+internshipSelectFrom+`
		ORDER BY i.created_at DESC OFFSET $1 LIMIT $2`, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	internships := []domain.Internship{}
	for rows.Next() {
		internship, err := scanInternship(rows)
		if err != nil {
			return nil, 0, err
		}
		internships = append(internships, *internship)
	}
	return internships, count, rows.Err()
}

func (r *internshipRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.InternshipStatus) error {
	_, err := r.db.Exec(ctx, `UPDATE internships SET status = $1 WHERE id = $2`, status, id)
	return err
}

type mentorAssignmentRepository struct {
	db dbtx
}

func NewMentorAssignmentRepository(db dbtx) MentorAssignmentRepository {
	return &mentorAssignmentRepository{db: db}
}

func scanAssignmentWithInternship(row pgx.Row) (*domain.MentorAssignment, error) {
	var a domain.MentorAssignment
	var in domain.Internship
	var student domain.User
	var department *string
	err := row.Scan(
		&a.ID, &a.InternshipID, &a.FacultyMentorID, &a.ApprovedAt, &a.ApprovedBy, &a.Status,
		&in.ID, &in.StudentID, &in.CompanyName, &in.CompanyAddress, &in.RoleTitle, &in.IndustryMentorName,
		&in.IndustryMentorEmail, &in.AcademicYear, &in.StartDate, &in.EndDate, &in.Status, &in.CreatedBy, &in.UpdatedBy,
		&in.CreatedAt, &in.UpdatedAt,
		&student.ID, &student.GoogleSub, &student.Email, &student.Name, &student.RoleID, &department, &student.CreatedAt, &student.LastLoginAt,
	)
	if err != nil {
		return nil, err
	}
	if department != nil {
		student.Department = *department
	}
	in.Student = &student
	a.Internship = &in
	return &a, nil
}

const assignmentWithInternshipCols = `
	ma.id, ma.internship_id, ma.faculty_mentor_id, ma.approved_at, ma.approved_by, ma.status,
` + internshipSelectCols

const assignmentWithInternshipFrom = `
	FROM mentor_assignments ma
	JOIN internships i ON i.id = ma.internship_id
	JOIN users s ON s.id = i.student_id`

func (r *mentorAssignmentRepository) Create(ctx context.Context, assignment *domain.MentorAssignment) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO mentor_assignments (internship_id, faculty_mentor_id, status)
		VALUES ($1, $2, $3)
		RETURNING id`,
		assignment.InternshipID, assignment.FacultyMentorID, assignment.Status,
	).Scan(&assignment.ID)
}

// FindByInternshipID also preloads FacultyMentor in the original GORM code,
// but nothing reads assignment.FacultyMentor after the query (only
// FacultyMentorID) - the join is dropped here for that reason. If a future
// caller needs the faculty user, reload it via UserRepository.FindByID.
func (r *mentorAssignmentRepository) FindByInternshipID(ctx context.Context, internshipID uuid.UUID) (*domain.MentorAssignment, error) {
	row := r.db.QueryRow(ctx, `
		SELECT id, internship_id, faculty_mentor_id, approved_at, approved_by, status
		FROM mentor_assignments WHERE internship_id = $1`, internshipID)

	var a domain.MentorAssignment
	err := row.Scan(&a.ID, &a.InternshipID, &a.FacultyMentorID, &a.ApprovedAt, &a.ApprovedBy, &a.Status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAssignmentNotFound
		}
		return nil, err
	}
	return &a, nil
}

func (r *mentorAssignmentRepository) ListByFacultyID(ctx context.Context, facultyID uuid.UUID) ([]domain.MentorAssignment, error) {
	rows, err := r.db.Query(ctx, `SELECT `+assignmentWithInternshipCols+assignmentWithInternshipFrom+`
		WHERE ma.faculty_mentor_id = $1`, facultyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	assignments := []domain.MentorAssignment{}
	for rows.Next() {
		a, err := scanAssignmentWithInternship(rows)
		if err != nil {
			return nil, err
		}
		assignments = append(assignments, *a)
	}
	return assignments, rows.Err()
}

func (r *mentorAssignmentRepository) Approve(ctx context.Context, assignmentID uuid.UUID, facultyID uuid.UUID) error {
	now := time.Now()
	tag, err := r.db.Exec(ctx, `
		UPDATE mentor_assignments SET status = $1, approved_at = $2, approved_by = $3
		WHERE id = $4 AND faculty_mentor_id = $5`,
		domain.AssignmentApproved, now, facultyID, assignmentID, facultyID,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrAssignmentNotFound
	}
	return nil
}
