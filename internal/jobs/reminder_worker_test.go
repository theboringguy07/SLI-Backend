package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/repositories"
)

func TestReminderWorkerEnqueuesMissingCurrentWeekReports(t *testing.T) {
	ctx := context.Background()
	studentID := uuid.New()
	internshipID := uuid.New()
	internships := &reminderInternshipRepo{
		internships: []domain.Internship{
			{
				ID:        internshipID,
				StudentID: studentID,
				Status:    domain.InternshipActive,
				StartDate: time.Now().AddDate(0, 0, -7),
				Student:   &domain.User{ID: studentID, Email: "student@somaiya.edu"},
			},
			{
				ID:        uuid.New(),
				StudentID: uuid.New(),
				Status:    domain.InternshipCompleted,
				StartDate: time.Now().AddDate(0, 0, -7),
				Student:   &domain.User{Email: "done@somaiya.edu"},
			},
		},
	}
	reports := &reminderReportRepo{existing: map[string]bool{}}
	notifications := &reminderNotificationService{}
	worker := NewReminderWorker(notifications, internships, reports)

	worker.sendReminders(ctx)

	if notifications.reminders != 1 {
		t.Fatalf("expected one reminder, got %d", notifications.reminders)
	}
	if notifications.lastEmail != "student@somaiya.edu" || notifications.lastWeek != 2 {
		t.Fatalf("unexpected reminder target email=%s week=%d", notifications.lastEmail, notifications.lastWeek)
	}
}

func TestReminderWorkerSkipsAlreadySubmittedReport(t *testing.T) {
	ctx := context.Background()
	internshipID := uuid.New()
	internships := &reminderInternshipRepo{
		internships: []domain.Internship{{
			ID:        internshipID,
			StudentID: uuid.New(),
			Status:    domain.InternshipActive,
			StartDate: time.Now(),
			Student:   &domain.User{Email: "student@somaiya.edu"},
		}},
	}
	reports := &reminderReportRepo{existing: map[string]bool{reminderKey(internshipID, 1): true}}
	notifications := &reminderNotificationService{}

	NewReminderWorker(notifications, internships, reports).sendReminders(ctx)

	if notifications.reminders != 0 {
		t.Fatalf("expected no reminders, got %d", notifications.reminders)
	}
}

type reminderInternshipRepo struct {
	internships []domain.Internship
}

func (r *reminderInternshipRepo) Create(ctx context.Context, internship *domain.Internship) error {
	r.internships = append(r.internships, *internship)
	return nil
}
func (r *reminderInternshipRepo) FindByStudentID(ctx context.Context, studentID uuid.UUID) (*domain.Internship, error) {
	return nil, repositories.ErrInternshipNotFound
}
func (r *reminderInternshipRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Internship, error) {
	return nil, repositories.ErrInternshipNotFound
}
func (r *reminderInternshipRepo) ListAll(ctx context.Context, offset, limit int) ([]domain.Internship, int64, error) {
	return r.internships, int64(len(r.internships)), nil
}
func (r *reminderInternshipRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.InternshipStatus) error {
	return nil
}

type reminderReportRepo struct {
	existing map[string]bool
}

func (r *reminderReportRepo) Create(ctx context.Context, report *domain.WeeklyReport) error {
	return nil
}
func (r *reminderReportRepo) Update(ctx context.Context, report *domain.WeeklyReport) error {
	return nil
}
func (r *reminderReportRepo) FindByInternshipTypeAndPeriod(ctx context.Context, internshipID uuid.UUID, reportType domain.ReportType, period int) (*domain.WeeklyReport, error) {
	if r.existing[reminderKey(internshipID, period)] {
		return &domain.WeeklyReport{InternshipID: internshipID, ReportType: reportType, WeekNumber: period}, nil
	}
	return nil, repositories.ErrReportNotFound
}
func (r *reminderReportRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.WeeklyReport, error) {
	return nil, repositories.ErrReportNotFound
}
func (r *reminderReportRepo) ListByInternship(ctx context.Context, internshipID uuid.UUID) ([]domain.WeeklyReport, error) {
	return nil, nil
}
func (r *reminderReportRepo) ListAll(ctx context.Context, offset, limit int) ([]domain.WeeklyReport, int64, error) {
	return nil, 0, nil
}
func (r *reminderReportRepo) RunInTransaction(ctx context.Context, fn func(txRepo repositories.ReportRepository) error) error {
	return fn(r)
}

func reminderKey(id uuid.UUID, week int) string {
	return id.String() + ":" + string(rune(week))
}

type reminderNotificationService struct {
	reminders int
	lastEmail string
	lastWeek  int
}

func (s *reminderNotificationService) NotifyFacultyReportSubmitted(ctx context.Context, facultyEmail string, studentName string, week int) error {
	return nil
}
func (s *reminderNotificationService) NotifyIndustryMentorReviewLink(ctx context.Context, mentorEmail string, studentName string, rollNo string, companyName string, rawToken string) error {
	return nil
}
func (s *reminderNotificationService) NotifyStudentWeeklyReminder(ctx context.Context, studentEmail string, week int) error {
	s.reminders++
	s.lastEmail = studentEmail
	s.lastWeek = week
	return nil
}
func (s *reminderNotificationService) NotifyMagicLink(ctx context.Context, email string, rawToken string, expiryMinutes int) error {
	return nil
}
