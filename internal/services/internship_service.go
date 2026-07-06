package services

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/platform/errors"
	"github.com/sli/backend/internal/repositories"
)

type InternshipService interface {
	EnrollStudent(ctx context.Context, coordinatorID uuid.UUID, req EnrollStudentRequest) (*domain.Internship, error)
	AssignFacultyMentor(ctx context.Context, internshipID uuid.UUID, facultyID uuid.UUID) (*domain.MentorAssignment, error)
	ApproveStudent(ctx context.Context, assignmentID uuid.UUID, facultyID uuid.UUID) error
	GetStudentInternship(ctx context.Context, studentID uuid.UUID) (*domain.Internship, error)
	GetInternshipByID(ctx context.Context, internshipID uuid.UUID) (*domain.Internship, error)
	ListStudentsForFaculty(ctx context.Context, facultyID uuid.UUID) ([]domain.MentorAssignment, error)
	ListAllInternships(ctx context.Context, offset, limit int) ([]domain.Internship, int64, error)
}

type EnrollStudentRequest struct {
	StudentID           uuid.UUID `json:"student_id"`
	CompanyName         string    `json:"company_name"`
	CompanyAddress      string    `json:"company_address"`
	RoleTitle           string    `json:"role_title"`
	IndustryMentorName  string    `json:"industry_mentor_name"`
	IndustryMentorEmail string    `json:"industry_mentor_email"`
	AcademicYear        string    `json:"academic_year"`
	StartDate           string    `json:"start_date"`
	EndDate             string    `json:"end_date"`
}

type internshipService struct {
	internshipRepo repositories.InternshipRepository
	assignmentRepo repositories.MentorAssignmentRepository
	userRepo       repositories.UserRepository
}

func NewInternshipService(
	internshipRepo repositories.InternshipRepository,
	assignmentRepo repositories.MentorAssignmentRepository,
	userRepo repositories.UserRepository,
) InternshipService {
	return &internshipService{
		internshipRepo: internshipRepo,
		assignmentRepo: assignmentRepo,
		userRepo:       userRepo,
	}
}

// ... (Implementation details will be added in next step to keep file manageable)

func (s *internshipService) EnrollStudent(ctx context.Context, coordinatorID uuid.UUID, req EnrollStudentRequest) (*domain.Internship, error) {
	// All internship details are filled by the coordinator, so require the text fields.
	if strings.TrimSpace(req.CompanyName) == "" {
		return nil, errors.New(errors.CodeValidationFailed, "company_name is required")
	}
	if strings.TrimSpace(req.RoleTitle) == "" {
		return nil, errors.New(errors.CodeValidationFailed, "role_title is required")
	}
	if strings.TrimSpace(req.IndustryMentorName) == "" {
		return nil, errors.New(errors.CodeValidationFailed, "industry_mentor_name is required")
	}
	if strings.TrimSpace(req.IndustryMentorEmail) == "" {
		return nil, errors.New(errors.CodeValidationFailed, "industry_mentor_email is required")
	}
	if strings.TrimSpace(req.AcademicYear) == "" {
		return nil, errors.New(errors.CodeValidationFailed, "academic_year is required")
	}

	// Parse dates
	layout := "2006-01-02"
	startDate, err := time.Parse(layout, req.StartDate)
	if err != nil {
		return nil, errors.New(errors.CodeValidationFailed, "invalid start_date format, expected YYYY-MM-DD")
	}
	endDate, err := time.Parse(layout, req.EndDate)
	if err != nil {
		return nil, errors.New(errors.CodeValidationFailed, "invalid end_date format, expected YYYY-MM-DD")
	}
	if endDate.Before(startDate) {
		return nil, errors.New(errors.CodeValidationFailed, "end_date must be after start_date")
	}

	// Check if student exists
	student, err := s.userRepo.FindByID(ctx, req.StudentID)
	if err != nil {
		return nil, errors.NewWithErr(errors.CodeValidationFailed, "student not found", err)
	}

	// Check if student has student role
	if student.Role.Name != domain.RoleStudent {
		return nil, errors.New(errors.CodeValidationFailed, "user is not a student")
	}

	internship := &domain.Internship{
		StudentID:           req.StudentID,
		CompanyName:         req.CompanyName,
		CompanyAddress:      req.CompanyAddress,
		RoleTitle:           req.RoleTitle,
		IndustryMentorName:  req.IndustryMentorName,
		IndustryMentorEmail: req.IndustryMentorEmail,
		AcademicYear:        req.AcademicYear,
		StartDate:           startDate,
		EndDate:             endDate,
		Status:              domain.InternshipActive,
		CreatedBy:           &coordinatorID,
		UpdatedBy:           &coordinatorID,
	}

	if err := s.internshipRepo.Create(ctx, internship); err != nil {
		// Unique constraint on student_id will cause an error here if they already have one
		return nil, errors.NewWithErr(errors.CodeDuplicateReport, "student already has an active internship", err) // Reusing duplicate code
	}

	return internship, nil
}

func (s *internshipService) AssignFacultyMentor(ctx context.Context, internshipID uuid.UUID, facultyID uuid.UUID) (*domain.MentorAssignment, error) {
	// Check if faculty exists and has faculty role
	faculty, err := s.userRepo.FindByID(ctx, facultyID)
	if err != nil {
		return nil, errors.NewWithErr(errors.CodeValidationFailed, "faculty not found", err)
	}

	if faculty.Role.Name != domain.RoleFaculty {
		return nil, errors.New(errors.CodeValidationFailed, "user is not a faculty mentor")
	}

	assignment := &domain.MentorAssignment{
		InternshipID:    internshipID,
		FacultyMentorID: facultyID,
		Status:          domain.AssignmentPending,
	}

	if err := s.assignmentRepo.Create(ctx, assignment); err != nil {
		return nil, errors.NewWithErr(errors.CodeInternalServer, "failed to assign mentor", err)
	}

	return assignment, nil
}

func (s *internshipService) ApproveStudent(ctx context.Context, assignmentID uuid.UUID, facultyID uuid.UUID) error {
	err := s.assignmentRepo.Approve(ctx, assignmentID, facultyID)
	if err != nil {
		if err == repositories.ErrAssignmentNotFound {
			return errors.New(errors.CodeNotFound, "assignment not found or does not belong to you")
		}
		return errors.NewWithErr(errors.CodeInternalServer, "failed to approve student", err)
	}
	return nil
}

func (s *internshipService) GetStudentInternship(ctx context.Context, studentID uuid.UUID) (*domain.Internship, error) {
	internship, err := s.internshipRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		if err == repositories.ErrInternshipNotFound {
			return nil, errors.New(errors.CodeNotFound, "internship not found")
		}
		return nil, errors.NewWithErr(errors.CodeInternalServer, "failed to fetch internship", err)
	}
	return internship, nil
}

func (s *internshipService) GetInternshipByID(ctx context.Context, internshipID uuid.UUID) (*domain.Internship, error) {
	internship, err := s.internshipRepo.FindByID(ctx, internshipID)
	if err != nil {
		if err == repositories.ErrInternshipNotFound {
			return nil, errors.New(errors.CodeNotFound, "internship not found")
		}
		return nil, errors.NewWithErr(errors.CodeInternalServer, "failed to fetch internship", err)
	}
	return internship, nil
}

func (s *internshipService) ListStudentsForFaculty(ctx context.Context, facultyID uuid.UUID) ([]domain.MentorAssignment, error) {
	assignments, err := s.assignmentRepo.ListByFacultyID(ctx, facultyID)
	if err != nil {
		return nil, errors.NewWithErr(errors.CodeInternalServer, "failed to list assignments", err)
	}
	return assignments, nil
}

func (s *internshipService) ListAllInternships(ctx context.Context, offset, limit int) ([]domain.Internship, int64, error) {
	internships, count, err := s.internshipRepo.ListAll(ctx, offset, limit)
	if err != nil {
		return nil, 0, errors.NewWithErr(errors.CodeInternalServer, "failed to list internships", err)
	}
	return internships, count, nil
}
