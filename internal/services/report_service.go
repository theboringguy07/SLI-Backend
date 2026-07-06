package services

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/platform/errors"
	"github.com/sli/backend/internal/repositories"
)

type ReportService interface {
	SubmitReport(ctx context.Context, studentID uuid.UUID, reportType domain.ReportType, period int, content string) (*domain.WeeklyReport, error)
	EditReport(ctx context.Context, studentID uuid.UUID, reportType domain.ReportType, period int, content string) (*domain.WeeklyReport, error)
	GetReports(ctx context.Context, studentID uuid.UUID) ([]domain.WeeklyReport, error)
	GetReportByPeriod(ctx context.Context, studentID uuid.UUID, reportType domain.ReportType, period int) (*domain.WeeklyReport, error)
	// GetReportsForInternship is the faculty-facing equivalent of GetReports -
	// it lists every report for one internship, but only for the faculty
	// mentor actually assigned to it (checked via assignmentRepo, the same
	// way SubmitMarks/SetSchedule check assignment in evaluation_service.go).
	GetReportsForInternship(ctx context.Context, facultyID uuid.UUID, internshipID uuid.UUID) ([]domain.WeeklyReport, error)
	// ListAllReports is the coordinator/admin-facing view across every
	// internship's reports.
	ListAllReports(ctx context.Context, offset, limit int) ([]domain.WeeklyReport, int64, error)
}

type reportService struct {
	reportRepo          repositories.ReportRepository
	internshipRepo      repositories.InternshipRepository
	assignmentRepo      repositories.MentorAssignmentRepository
	userRepo            repositories.UserRepository
	tokenService        TokenService
	notificationService NotificationService
	editWindowHrs       int
}

func NewReportService(
	reportRepo repositories.ReportRepository,
	internshipRepo repositories.InternshipRepository,
	assignmentRepo repositories.MentorAssignmentRepository,
	userRepo repositories.UserRepository,
	tokenService TokenService,
	notificationService NotificationService,
	editWindowHrs int,
) ReportService {
	return &reportService{
		reportRepo:          reportRepo,
		internshipRepo:      internshipRepo,
		assignmentRepo:      assignmentRepo,
		userRepo:            userRepo,
		tokenService:        tokenService,
		notificationService: notificationService,
		editWindowHrs:       editWindowHrs,
	}
}

func (s *reportService) SubmitReport(ctx context.Context, studentID uuid.UUID, reportType domain.ReportType, period int, content string) (*domain.WeeklyReport, error) {
	if !reportType.Valid() {
		return nil, errors.New(errors.CodeInvalidReportType, "report type must be one of weekly, fortnightly or monthly")
	}
	if period < 1 || period > reportType.MaxPeriod() {
		return nil, errors.New(errors.CodeWeekOutOfRange, fmt.Sprintf("%s period must be between 1 and %d", reportType, reportType.MaxPeriod()))
	}

	internship, err := s.internshipRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		if err == repositories.ErrInternshipNotFound {
			return nil, errors.New(errors.CodeNotFound, "active internship not found")
		}
		return nil, errors.NewWithErr(errors.CodeInternalServer, "failed to check internship", err)
	}

	if internship.Status != domain.InternshipActive {
		return nil, errors.New(errors.CodeForbidden, "internship is not active")
	}

	// Check if already exists
	existing, err := s.reportRepo.FindByInternshipTypeAndPeriod(ctx, internship.ID, reportType, period)
	if err == nil && existing != nil {
		return nil, errors.New(errors.CodeDuplicateReport, "report for this period already submitted")
	} else if err != nil && err != repositories.ErrReportNotFound {
		return nil, errors.NewWithErr(errors.CodeInternalServer, "database error checking reports", err)
	}

	now := time.Now()
	report := &domain.WeeklyReport{
		InternshipID: internship.ID,
		ReportType:   reportType,
		WeekNumber:   period,
		Content:      content,
		Status:       domain.ReportStatusSubmitted,
		SubmittedAt:  now,
		EditedAt:     now,
		CreatedBy:    studentID,
		UpdatedBy:    studentID,
	}

	err = s.reportRepo.RunInTransaction(ctx, func(txRepo repositories.ReportRepository) error {
		if err := txRepo.Create(ctx, report); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, errors.NewWithErr(errors.CodeInternalServer, "failed to submit report", err)
	}

	if s.assignmentRepo != nil && s.userRepo != nil && s.notificationService != nil {
		assignment, err := s.assignmentRepo.FindByInternshipID(ctx, internship.ID)
		if err == nil {
			faculty, err := s.userRepo.FindByID(ctx, assignment.FacultyMentorID)
			if err == nil {
				_ = s.notificationService.NotifyFacultyReportSubmitted(ctx, faculty.Email, internship.Student.Name, period)
			}
		}
	}

	// All report types (weekly, fortnightly, monthly) trigger an industry-mentor
	// review link so the mentor can leave feedback on each report the student submits.
	if s.tokenService != nil && s.notificationService != nil && internship.IndustryMentorEmail != "" {
		rawToken, err := s.tokenService.GenerateToken(ctx, report.ID)
		if err != nil {
			return nil, err
		}
		if err := s.notificationService.NotifyIndustryMentorReviewLink(ctx, internship.IndustryMentorEmail, internship.Student.Name, internship.Student.Email, internship.CompanyName, rawToken); err != nil {
			return nil, errors.NewWithErr(errors.CodeInternalServer, "failed to enqueue industry mentor review email", err)
		}
	}

	return report, nil
}

func (s *reportService) EditReport(ctx context.Context, studentID uuid.UUID, reportType domain.ReportType, period int, content string) (*domain.WeeklyReport, error) {
	if !reportType.Valid() {
		return nil, errors.New(errors.CodeInvalidReportType, "report type must be one of weekly, fortnightly or monthly")
	}

	internship, err := s.internshipRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, errors.New(errors.CodeNotFound, "active internship not found")
	}

	report, err := s.reportRepo.FindByInternshipTypeAndPeriod(ctx, internship.ID, reportType, period)
	if err != nil {
		if err == repositories.ErrReportNotFound {
			return nil, errors.New(errors.CodeNotFound, "report not found")
		}
		return nil, errors.NewWithErr(errors.CodeInternalServer, "database error", err)
	}

	// Check edit window (e.g. 24 hours after submission)
	if time.Since(report.SubmittedAt).Hours() > float64(s.editWindowHrs) {
		return nil, errors.New(errors.CodeEditWindowClosed, "report edit window has closed")
	}

	report.Content = content
	report.EditedAt = time.Now()
	report.UpdatedBy = studentID

	if err := s.reportRepo.Update(ctx, report); err != nil {
		return nil, errors.NewWithErr(errors.CodeInternalServer, "failed to update report", err)
	}

	return report, nil
}

func (s *reportService) GetReports(ctx context.Context, studentID uuid.UUID) ([]domain.WeeklyReport, error) {
	internship, err := s.internshipRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, errors.New(errors.CodeNotFound, "active internship not found")
	}

	return s.reportRepo.ListByInternship(ctx, internship.ID)
}

func (s *reportService) GetReportsForInternship(ctx context.Context, facultyID uuid.UUID, internshipID uuid.UUID) ([]domain.WeeklyReport, error) {
	assignment, err := s.assignmentRepo.FindByInternshipID(ctx, internshipID)
	if err != nil {
		return nil, errors.New(errors.CodeNotFound, "internship assignment not found")
	}
	if assignment.FacultyMentorID != facultyID {
		return nil, errors.New(errors.CodeForbidden, "you are not the assigned mentor for this internship")
	}

	return s.reportRepo.ListByInternship(ctx, internshipID)
}

func (s *reportService) ListAllReports(ctx context.Context, offset, limit int) ([]domain.WeeklyReport, int64, error) {
	reports, count, err := s.reportRepo.ListAll(ctx, offset, limit)
	if err != nil {
		return nil, 0, errors.NewWithErr(errors.CodeInternalServer, "failed to list reports", err)
	}
	return reports, count, nil
}

func (s *reportService) GetReportByPeriod(ctx context.Context, studentID uuid.UUID, reportType domain.ReportType, period int) (*domain.WeeklyReport, error) {
	if !reportType.Valid() {
		return nil, errors.New(errors.CodeInvalidReportType, "report type must be one of weekly, fortnightly or monthly")
	}

	internship, err := s.internshipRepo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, errors.New(errors.CodeNotFound, "active internship not found")
	}

	report, err := s.reportRepo.FindByInternshipTypeAndPeriod(ctx, internship.ID, reportType, period)
	if err != nil {
		if err == repositories.ErrReportNotFound {
			return nil, errors.New(errors.CodeNotFound, "report not found")
		}
		return nil, errors.NewWithErr(errors.CodeInternalServer, "database error", err)
	}
	return report, nil
}
