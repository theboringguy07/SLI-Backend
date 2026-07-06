package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sli/backend/internal/domain"
	apperrors "github.com/sli/backend/internal/platform/errors"
	"github.com/sli/backend/internal/repositories"
)

func TestInternshipServiceEnrollAssignAndApprove(t *testing.T) {
	ctx := context.Background()
	studentID := uuid.New()
	facultyID := uuid.New()
	internships := newFakeInternshipRepo()
	assignments := newFakeAssignmentRepo()
	users := &fakeUserRepo{users: map[uuid.UUID]*domain.User{
		studentID: {ID: studentID, Email: "student@somaiya.edu", Role: domain.Role{Name: domain.RoleStudent}},
		facultyID: {ID: facultyID, Email: "faculty@somaiya.edu", Role: domain.Role{Name: domain.RoleFaculty}},
	}}
	svc := NewInternshipService(internships, assignments, users)

	internship, err := svc.EnrollStudent(ctx, facultyID, EnrollStudentRequest{
		StudentID:           studentID,
		CompanyName:         "Acme",
		RoleTitle:           "SDE Intern",
		IndustryMentorName:  "Jane Mentor",
		IndustryMentorEmail: "mentor@example.com",
		AcademicYear:        "2026-2027",
		StartDate:           "2026-06-25",
		EndDate:             "2026-10-25",
	})
	if err != nil {
		t.Fatalf("EnrollStudent returned error: %v", err)
	}
	if internship.Status != domain.InternshipActive {
		t.Fatalf("expected active internship, got %s", internship.Status)
	}

	assignment, err := svc.AssignFacultyMentor(ctx, internship.ID, facultyID)
	if err != nil {
		t.Fatalf("AssignFacultyMentor returned error: %v", err)
	}
	if assignment.Status != domain.AssignmentPending {
		t.Fatalf("expected pending assignment, got %s", assignment.Status)
	}

	if err := svc.ApproveStudent(ctx, assignment.ID, facultyID); err != nil {
		t.Fatalf("ApproveStudent returned error: %v", err)
	}
	if assignments.assignments[assignment.ID].Status != domain.AssignmentApproved {
		t.Fatal("expected assignment to be approved")
	}
}

func TestInternshipServiceRejectsInvalidEnrollment(t *testing.T) {
	users := &fakeUserRepo{users: map[uuid.UUID]*domain.User{
		uuid.New(): {ID: uuid.New(), Role: domain.Role{Name: domain.RoleFaculty}},
	}}
	svc := NewInternshipService(newFakeInternshipRepo(), newFakeAssignmentRepo(), users)

	_, err := svc.EnrollStudent(context.Background(), uuid.New(), EnrollStudentRequest{
		StudentID:           uuid.New(),
		StartDate:           "bad-date",
		EndDate:             "2026-10-25",
		CompanyName:         "Acme",
		RoleTitle:           "SDE Intern",
		IndustryMentorName:  "Jane Mentor",
		IndustryMentorEmail: "mentor@example.com",
		AcademicYear:        "2026-2027",
	})
	assertAppCode(t, err, apperrors.CodeValidationFailed)
}

func TestReportServiceSubmitEditAndNotifications(t *testing.T) {
	ctx := context.Background()
	studentID := uuid.New()
	facultyID := uuid.New()
	internshipID := uuid.New()
	reportRepo := newFakeReportRepo()
	internshipRepo := newFakeInternshipRepo()
	internshipRepo.byStudent[studentID] = &domain.Internship{
		ID:                  internshipID,
		StudentID:           studentID,
		Status:              domain.InternshipActive,
		CompanyName:         "Acme",
		IndustryMentorEmail: "mentor@example.com",
		Student:             &domain.User{ID: studentID, Name: "Student", Email: "student@somaiya.edu"},
	}
	assignmentRepo := newFakeAssignmentRepo()
	assignmentRepo.byInternship[internshipID] = &domain.MentorAssignment{ID: uuid.New(), InternshipID: internshipID, FacultyMentorID: facultyID}
	users := &fakeUserRepo{users: map[uuid.UUID]*domain.User{
		facultyID: {ID: facultyID, Email: "faculty@somaiya.edu"},
	}}
	tokens := &fakeTokenService{raw: "raw-token"}
	notifications := &fakeNotificationService{}
	svc := NewReportService(reportRepo, internshipRepo, assignmentRepo, users, tokens, notifications, 24)

	report, err := svc.SubmitReport(ctx, studentID, domain.ReportTypeWeekly, 1, "week 1")
	if err != nil {
		t.Fatalf("SubmitReport returned error: %v", err)
	}
	if report.WeekNumber != 1 || report.CreatedBy != studentID || report.UpdatedBy != studentID {
		t.Fatalf("unexpected report fields: %#v", report)
	}
	if tokens.generatedFor != report.ID {
		t.Fatal("expected industry token to be generated for report")
	}
	if notifications.facultyCount != 1 || notifications.industryCount != 1 {
		t.Fatalf("expected faculty and industry notifications, got faculty=%d industry=%d", notifications.facultyCount, notifications.industryCount)
	}

	updated, err := svc.EditReport(ctx, studentID, domain.ReportTypeWeekly, 1, "updated")
	if err != nil {
		t.Fatalf("EditReport returned error: %v", err)
	}
	if updated.Content != "updated" {
		t.Fatalf("expected updated content, got %q", updated.Content)
	}
}

func TestReportServiceRejectsInvalidDuplicateAndClosedEdit(t *testing.T) {
	ctx := context.Background()
	studentID := uuid.New()
	internshipID := uuid.New()
	reportRepo := newFakeReportRepo()
	internshipRepo := newFakeInternshipRepo()
	internshipRepo.byStudent[studentID] = &domain.Internship{
		ID:        internshipID,
		StudentID: studentID,
		Status:    domain.InternshipActive,
		Student:   &domain.User{ID: studentID, Email: "student@somaiya.edu"},
	}
	svc := NewReportService(reportRepo, internshipRepo, nil, nil, nil, nil, 1)

	_, err := svc.SubmitReport(ctx, studentID, domain.ReportTypeWeekly, 17, "bad")
	assertAppCode(t, err, apperrors.CodeWeekOutOfRange)

	reportRepo.reports[key(internshipID, domain.ReportTypeWeekly, 1)] = &domain.WeeklyReport{
		ID:           uuid.New(),
		InternshipID: internshipID,
		ReportType:   domain.ReportTypeWeekly,
		WeekNumber:   1,
		Content:      "old",
		SubmittedAt:  time.Now().Add(-2 * time.Hour),
	}
	_, err = svc.SubmitReport(ctx, studentID, domain.ReportTypeWeekly, 1, "duplicate")
	assertAppCode(t, err, apperrors.CodeDuplicateReport)

	_, err = svc.EditReport(ctx, studentID, domain.ReportTypeWeekly, 1, "late")
	assertAppCode(t, err, apperrors.CodeEditWindowClosed)
}

func TestFeedbackServiceRequiresAssignedFaculty(t *testing.T) {
	ctx := context.Background()
	reportID := uuid.New()
	internshipID := uuid.New()
	facultyID := uuid.New()
	reportRepo := newFakeReportRepo()
	reportRepo.byID[reportID] = &domain.WeeklyReport{ID: reportID, InternshipID: internshipID}
	assignments := newFakeAssignmentRepo()
	assignments.byInternship[internshipID] = &domain.MentorAssignment{InternshipID: internshipID, FacultyMentorID: facultyID}
	feedbackRepo := &fakeFeedbackRepo{}
	svc := NewFeedbackService(feedbackRepo, reportRepo, assignments)

	feedback, err := svc.SubmitFacultyFeedback(ctx, facultyID, reportID, "good")
	if err != nil {
		t.Fatalf("SubmitFacultyFeedback returned error: %v", err)
	}
	if feedback.Source != domain.FeedbackSourceFaculty || feedback.GivenBy == nil || *feedback.GivenBy != facultyID || feedback.ReportID != reportID {
		t.Fatalf("unexpected feedback: %#v", feedback)
	}

	_, err = svc.SubmitFacultyFeedback(ctx, uuid.New(), reportID, "bad")
	assertAppCode(t, err, apperrors.CodeForbidden)
}

func TestTokenServiceGenerateValidateExpireAndUsed(t *testing.T) {
	ctx := context.Background()
	repo := newFakeTokenRepo()
	svc := NewTokenService(repo)
	reportID := uuid.New()

	raw, err := svc.GenerateToken(ctx, reportID)
	if err != nil {
		t.Fatalf("GenerateToken returned error: %v", err)
	}
	if raw == "" {
		t.Fatal("expected raw token")
	}
	hash := sha256.Sum256([]byte(raw))
	stored := repo.byHash[hex.EncodeToString(hash[:])]
	if stored == nil || stored.TokenHash == raw {
		t.Fatal("expected hashed token storage")
	}

	token, err := svc.ValidateToken(ctx, raw)
	if err != nil {
		t.Fatalf("ValidateToken returned error: %v", err)
	}
	if token.ReportID != reportID {
		t.Fatalf("expected report id %s, got %s", reportID, token.ReportID)
	}

	usedAt := time.Now()
	token.UsedAt = &usedAt
	_, err = svc.ValidateToken(ctx, raw)
	assertAppCode(t, err, apperrors.CodeTokenExpired)

	token.UsedAt = nil
	token.ExpiresAt = time.Now().Add(-time.Hour)
	_, err = svc.ValidateToken(ctx, raw)
	assertAppCode(t, err, apperrors.CodeTokenExpired)
}

func TestNotificationServiceEnqueuesEmails(t *testing.T) {
	outbox := &fakeOutboxRepo{}
	svc := NewNotificationService(outbox, "http://frontend", "http://api")
	ctx := context.Background()

	if err := svc.NotifyFacultyReportSubmitted(ctx, "faculty@somaiya.edu", "Student", 3); err != nil {
		t.Fatalf("NotifyFacultyReportSubmitted returned error: %v", err)
	}
	if err := svc.NotifyIndustryMentorReviewLink(ctx, "mentor@example.com", "Student", "ROLL", "Acme", "raw"); err != nil {
		t.Fatalf("NotifyIndustryMentorReviewLink returned error: %v", err)
	}
	if err := svc.NotifyStudentWeeklyReminder(ctx, "student@somaiya.edu", 4); err != nil {
		t.Fatalf("NotifyStudentWeeklyReminder returned error: %v", err)
	}
	if len(outbox.emails) != 3 {
		t.Fatalf("expected 3 emails, got %d", len(outbox.emails))
	}
}

func TestEvaluationServiceScheduleMarksAndCorrection(t *testing.T) {
	ctx := context.Background()
	internshipID := uuid.New()
	facultyID := uuid.New()
	adminID := uuid.New()
	assignments := newFakeAssignmentRepo()
	assignments.byInternship[internshipID] = &domain.MentorAssignment{InternshipID: internshipID, FacultyMentorID: facultyID}
	evals := newFakeEvaluationRepo()
	audit := &fakeAuditRepo{}
	marksheets := &fakeMarksheetService{}
	svc := NewEvaluationService(evals, newFakeInternshipRepo(), assignments, audit, marksheets)

	schedule := &domain.EvaluationSchedule{InternshipID: internshipID, Venue: "Hall"}
	if err := svc.SetSchedule(ctx, facultyID, schedule); err != nil {
		t.Fatalf("SetSchedule returned error: %v", err)
	}
	if schedule.SetBy != facultyID {
		t.Fatal("expected schedule SetBy to be faculty")
	}

	score := &domain.EvaluationScore{
		InternshipID:        internshipID,
		ReportQuality:       18,
		OralPresentation:    27,
		WorkQuality:         14,
		Understanding:       13,
		PeriodicInteraction: 19,
	}
	if err := svc.SubmitMarks(ctx, facultyID, score); err != nil {
		t.Fatalf("SubmitMarks returned error: %v", err)
	}
	if score.LockedAt == nil || score.SubmittedBy != facultyID {
		t.Fatal("expected score to be locked and submitted by faculty")
	}
	if len(audit.logs) != 1 || marksheets.generatedFor != internshipID {
		t.Fatalf("expected audit and marksheet generation, audit=%d generated=%s", len(audit.logs), marksheets.generatedFor)
	}

	err := svc.SubmitMarks(ctx, facultyID, &domain.EvaluationScore{InternshipID: internshipID})
	assertAppCode(t, err, apperrors.CodeReportAlreadyLocked)

	originalQuality := evals.scores[internshipID].ReportQuality
	if err := svc.CorrectMarks(ctx, adminID, "Admin", internshipID, domain.ExamTypeISE, &domain.EvaluationScore{ReportQuality: 20, Remarks: "corrected"}, "reason"); err != nil {
		t.Fatalf("CorrectMarks returned error: %v", err)
	}
	if evals.scores[internshipID].ReportQuality != originalQuality {
		t.Fatal("correction must not mutate locked original score")
	}
	if len(evals.corrections) != 1 || len(audit.logs) != 2 {
		t.Fatalf("expected correction and audit log, corrections=%d audit=%d", len(evals.corrections), len(audit.logs))
	}
}

func assertAppCode(t *testing.T, err error, code apperrors.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected %s error, got nil", code)
	}
	var appErr *apperrors.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T: %v", err, err)
	}
	if appErr.Code != code {
		t.Fatalf("expected code %s, got %s", code, appErr.Code)
	}
}

type fakeUserRepo struct {
	users map[uuid.UUID]*domain.User
}

func (r *fakeUserRepo) FindByGoogleSub(ctx context.Context, sub string) (*domain.User, error) {
	for _, user := range r.users {
		if user.GoogleSub != nil && *user.GoogleSub == sub {
			return user, nil
		}
	}
	return nil, repositories.ErrUserNotFound
}
func (r *fakeUserRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, repositories.ErrUserNotFound
	}
	return user, nil
}
func (r *fakeUserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	for _, user := range r.users {
		if user.Email == email {
			return user, nil
		}
	}
	return nil, repositories.ErrUserNotFound
}
func (r *fakeUserRepo) CreateUser(ctx context.Context, user *domain.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	r.users[user.ID] = user
	return nil
}
func (r *fakeUserRepo) UpdateUser(ctx context.Context, user *domain.User) error {
	r.users[user.ID] = user
	return nil
}
func (r *fakeUserRepo) FindRoleByName(ctx context.Context, roleName domain.RoleName) (*domain.Role, error) {
	return &domain.Role{Name: roleName}, nil
}
func (r *fakeUserRepo) SetRole(ctx context.Context, userID uuid.UUID, roleName domain.RoleName) error {
	r.users[userID].Role = domain.Role{Name: roleName}
	return nil
}
func (r *fakeUserRepo) LinkGoogleSub(ctx context.Context, userID uuid.UUID, sub string) error {
	r.users[userID].GoogleSub = &sub
	return nil
}
func (r *fakeUserRepo) ListUsers(ctx context.Context, offset, limit int) ([]domain.User, int64, error) {
	users := make([]domain.User, 0, len(r.users))
	for _, user := range r.users {
		users = append(users, *user)
	}
	return users, int64(len(users)), nil
}
func (r *fakeUserRepo) ListByRole(ctx context.Context, roleName domain.RoleName) ([]domain.User, error) {
	var users []domain.User
	for _, user := range r.users {
		if user.Role.Name == roleName {
			users = append(users, *user)
		}
	}
	return users, nil
}
func (r *fakeUserRepo) UpdateProfileFields(ctx context.Context, userID uuid.UUID, updates map[string]interface{}) error {
	user, ok := r.users[userID]
	if !ok {
		return repositories.ErrUserNotFound
	}
	if v, ok := updates["department"]; ok {
		if s, ok := v.(string); ok {
			user.Department = s
		}
	}
	return nil
}

type fakeInternshipRepo struct {
	byID      map[uuid.UUID]*domain.Internship
	byStudent map[uuid.UUID]*domain.Internship
}

func newFakeInternshipRepo() *fakeInternshipRepo {
	return &fakeInternshipRepo{byID: map[uuid.UUID]*domain.Internship{}, byStudent: map[uuid.UUID]*domain.Internship{}}
}
func (r *fakeInternshipRepo) Create(ctx context.Context, internship *domain.Internship) error {
	if internship.ID == uuid.Nil {
		internship.ID = uuid.New()
	}
	r.byID[internship.ID] = internship
	r.byStudent[internship.StudentID] = internship
	return nil
}
func (r *fakeInternshipRepo) FindByStudentID(ctx context.Context, studentID uuid.UUID) (*domain.Internship, error) {
	internship, ok := r.byStudent[studentID]
	if !ok {
		return nil, repositories.ErrInternshipNotFound
	}
	return internship, nil
}
func (r *fakeInternshipRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Internship, error) {
	internship, ok := r.byID[id]
	if !ok {
		return nil, repositories.ErrInternshipNotFound
	}
	return internship, nil
}
func (r *fakeInternshipRepo) ListAll(ctx context.Context, offset, limit int) ([]domain.Internship, int64, error) {
	internships := make([]domain.Internship, 0, len(r.byID))
	for _, internship := range r.byID {
		internships = append(internships, *internship)
	}
	return internships, int64(len(internships)), nil
}
func (r *fakeInternshipRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.InternshipStatus) error {
	r.byID[id].Status = status
	return nil
}

type fakeAssignmentRepo struct {
	assignments  map[uuid.UUID]*domain.MentorAssignment
	byInternship map[uuid.UUID]*domain.MentorAssignment
}

func newFakeAssignmentRepo() *fakeAssignmentRepo {
	return &fakeAssignmentRepo{assignments: map[uuid.UUID]*domain.MentorAssignment{}, byInternship: map[uuid.UUID]*domain.MentorAssignment{}}
}
func (r *fakeAssignmentRepo) Create(ctx context.Context, assignment *domain.MentorAssignment) error {
	if assignment.ID == uuid.Nil {
		assignment.ID = uuid.New()
	}
	r.assignments[assignment.ID] = assignment
	r.byInternship[assignment.InternshipID] = assignment
	return nil
}
func (r *fakeAssignmentRepo) FindByInternshipID(ctx context.Context, internshipID uuid.UUID) (*domain.MentorAssignment, error) {
	assignment, ok := r.byInternship[internshipID]
	if !ok {
		return nil, repositories.ErrAssignmentNotFound
	}
	return assignment, nil
}
func (r *fakeAssignmentRepo) ListByFacultyID(ctx context.Context, facultyID uuid.UUID) ([]domain.MentorAssignment, error) {
	var out []domain.MentorAssignment
	for _, assignment := range r.assignments {
		if assignment.FacultyMentorID == facultyID {
			out = append(out, *assignment)
		}
	}
	return out, nil
}
func (r *fakeAssignmentRepo) Approve(ctx context.Context, assignmentID uuid.UUID, facultyID uuid.UUID) error {
	assignment, ok := r.assignments[assignmentID]
	if !ok || assignment.FacultyMentorID != facultyID {
		return repositories.ErrAssignmentNotFound
	}
	now := time.Now()
	assignment.Status = domain.AssignmentApproved
	assignment.ApprovedAt = &now
	assignment.ApprovedBy = &facultyID
	return nil
}

type fakeReportRepo struct {
	reports map[string]*domain.WeeklyReport
	byID    map[uuid.UUID]*domain.WeeklyReport
}

func newFakeReportRepo() *fakeReportRepo {
	return &fakeReportRepo{reports: map[string]*domain.WeeklyReport{}, byID: map[uuid.UUID]*domain.WeeklyReport{}}
}
func (r *fakeReportRepo) Create(ctx context.Context, report *domain.WeeklyReport) error {
	if report.ID == uuid.Nil {
		report.ID = uuid.New()
	}
	r.reports[key(report.InternshipID, report.ReportType, report.WeekNumber)] = report
	r.byID[report.ID] = report
	return nil
}
func (r *fakeReportRepo) Update(ctx context.Context, report *domain.WeeklyReport) error {
	r.reports[key(report.InternshipID, report.ReportType, report.WeekNumber)] = report
	r.byID[report.ID] = report
	return nil
}
func (r *fakeReportRepo) FindByInternshipTypeAndPeriod(ctx context.Context, internshipID uuid.UUID, reportType domain.ReportType, period int) (*domain.WeeklyReport, error) {
	report, ok := r.reports[key(internshipID, reportType, period)]
	if !ok {
		return nil, repositories.ErrReportNotFound
	}
	return report, nil
}
func (r *fakeReportRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.WeeklyReport, error) {
	report, ok := r.byID[id]
	if !ok {
		return nil, repositories.ErrReportNotFound
	}
	return report, nil
}
func (r *fakeReportRepo) ListByInternship(ctx context.Context, internshipID uuid.UUID) ([]domain.WeeklyReport, error) {
	var reports []domain.WeeklyReport
	for _, report := range r.reports {
		if report.InternshipID == internshipID {
			reports = append(reports, *report)
		}
	}
	return reports, nil
}
func (r *fakeReportRepo) RunInTransaction(ctx context.Context, fn func(txRepo repositories.ReportRepository) error) error {
	return fn(r)
}

func key(internshipID uuid.UUID, reportType domain.ReportType, period int) string {
	return internshipID.String() + ":" + string(reportType) + ":" + string(rune(period))
}

type fakeFeedbackRepo struct {
	feedbacks []domain.ReportFeedback
}

func (r *fakeFeedbackRepo) Create(ctx context.Context, feedback *domain.ReportFeedback) error {
	if feedback.ID == uuid.Nil {
		feedback.ID = uuid.New()
	}
	r.feedbacks = append(r.feedbacks, *feedback)
	return nil
}
func (r *fakeFeedbackRepo) ListByReport(ctx context.Context, reportID uuid.UUID) ([]domain.ReportFeedback, error) {
	var out []domain.ReportFeedback
	for _, feedback := range r.feedbacks {
		if feedback.ReportID == reportID {
			out = append(out, feedback)
		}
	}
	return out, nil
}

type fakeTokenRepo struct {
	byHash map[string]*domain.IndustryAccessToken
}

func newFakeTokenRepo() *fakeTokenRepo {
	return &fakeTokenRepo{byHash: map[string]*domain.IndustryAccessToken{}}
}
func (r *fakeTokenRepo) Create(ctx context.Context, token *domain.IndustryAccessToken) error {
	if token.ID == uuid.Nil {
		token.ID = uuid.New()
	}
	r.byHash[token.TokenHash] = token
	return nil
}
func (r *fakeTokenRepo) FindByHash(ctx context.Context, hash string) (*domain.IndustryAccessToken, error) {
	token, ok := r.byHash[hash]
	if !ok {
		return nil, repositories.ErrTokenNotFound
	}
	return token, nil
}
func (r *fakeTokenRepo) MarkUsed(ctx context.Context, id uuid.UUID) error {
	for _, token := range r.byHash {
		if token.ID == id {
			now := time.Now()
			token.UsedAt = &now
		}
	}
	return nil
}
func (r *fakeTokenRepo) DeleteExpired(ctx context.Context) error {
	for hash, token := range r.byHash {
		if time.Now().After(token.ExpiresAt) {
			delete(r.byHash, hash)
		}
	}
	return nil
}

type fakeTokenService struct {
	raw          string
	generatedFor uuid.UUID
}

func (s *fakeTokenService) GenerateToken(ctx context.Context, reportID uuid.UUID) (string, error) {
	s.generatedFor = reportID
	return s.raw, nil
}
func (s *fakeTokenService) ValidateToken(ctx context.Context, rawToken string) (*domain.IndustryAccessToken, error) {
	return nil, nil
}
func (s *fakeTokenService) MarkTokenUsed(ctx context.Context, id uuid.UUID) error { return nil }
func (s *fakeTokenService) CleanupExpiredTokens(ctx context.Context) error        { return nil }

type fakeNotificationService struct {
	facultyCount   int
	industryCount  int
	reminderCount  int
	magicLinkCount int
}

func (s *fakeNotificationService) NotifyFacultyReportSubmitted(ctx context.Context, facultyEmail string, studentName string, week int) error {
	s.facultyCount++
	return nil
}
func (s *fakeNotificationService) NotifyIndustryMentorReviewLink(ctx context.Context, mentorEmail string, studentName string, rollNo string, companyName string, rawToken string) error {
	s.industryCount++
	return nil
}
func (s *fakeNotificationService) NotifyStudentWeeklyReminder(ctx context.Context, studentEmail string, week int) error {
	s.reminderCount++
	return nil
}
func (s *fakeNotificationService) NotifyMagicLink(ctx context.Context, email string, rawToken string, expiryMinutes int) error {
	s.magicLinkCount++
	return nil
}

type fakeOutboxRepo struct {
	emails []domain.EmailOutbox
}

func (r *fakeOutboxRepo) Enqueue(ctx context.Context, email *domain.EmailOutbox) error {
	r.emails = append(r.emails, *email)
	return nil
}
func (r *fakeOutboxRepo) FetchPending(ctx context.Context, limit int) ([]domain.EmailOutbox, error) {
	return r.emails, nil
}
func (r *fakeOutboxRepo) MarkSent(ctx context.Context, id uuid.UUID) error { return nil }
func (r *fakeOutboxRepo) MarkFailed(ctx context.Context, id uuid.UUID, nextAttemptAt time.Time) error {
	return nil
}

type fakeEvaluationRepo struct {
	schedules   map[uuid.UUID]*domain.EvaluationSchedule
	scores      map[uuid.UUID]*domain.EvaluationScore
	corrections []domain.EvaluationCorrection
}

func newFakeEvaluationRepo() *fakeEvaluationRepo {
	return &fakeEvaluationRepo{schedules: map[uuid.UUID]*domain.EvaluationSchedule{}, scores: map[uuid.UUID]*domain.EvaluationScore{}}
}
func (r *fakeEvaluationRepo) SetSchedule(ctx context.Context, schedule *domain.EvaluationSchedule) error {
	if schedule.ID == uuid.Nil {
		schedule.ID = uuid.New()
	}
	r.schedules[schedule.InternshipID] = schedule
	return nil
}
func (r *fakeEvaluationRepo) GetSchedule(ctx context.Context, internshipID uuid.UUID, examType domain.ExamType) (*domain.EvaluationSchedule, error) {
	schedule, ok := r.schedules[internshipID]
	if !ok {
		return nil, repositories.ErrEvaluationNotFound
	}
	return schedule, nil
}
func (r *fakeEvaluationRepo) SubmitScores(ctx context.Context, score *domain.EvaluationScore) error {
	if score.ID == uuid.Nil {
		score.ID = uuid.New()
	}
	r.scores[score.InternshipID] = score
	return nil
}
func (r *fakeEvaluationRepo) GetScore(ctx context.Context, internshipID uuid.UUID, examType domain.ExamType) (*domain.EvaluationScore, error) {
	score, ok := r.scores[internshipID]
	if !ok {
		return nil, repositories.ErrEvaluationNotFound
	}
	return score, nil
}
func (r *fakeEvaluationRepo) CreateCorrection(ctx context.Context, correction *domain.EvaluationCorrection) error {
	if correction.ID == uuid.Nil {
		correction.ID = uuid.New()
	}
	r.corrections = append(r.corrections, *correction)
	return nil
}
func (r *fakeEvaluationRepo) RunInTransaction(ctx context.Context, fn func(txRepo repositories.EvaluationRepository) error) error {
	return fn(r)
}

type fakeAuditRepo struct {
	logs []domain.AuditLog
}

func (r *fakeAuditRepo) Create(ctx context.Context, log *domain.AuditLog) error {
	r.logs = append(r.logs, *log)
	return nil
}
func (r *fakeAuditRepo) ListAll(ctx context.Context, offset, limit int) ([]domain.AuditLog, int64, error) {
	return r.logs, int64(len(r.logs)), nil
}

type fakeMarksheetService struct {
	generatedFor uuid.UUID
}

func (s *fakeMarksheetService) GenerateMarksheet(ctx context.Context, internshipID uuid.UUID, examType domain.ExamType) error {
	s.generatedFor = internshipID
	return nil
}
func (s *fakeMarksheetService) GetMarksheetContent(ctx context.Context, internshipID uuid.UUID, examType domain.ExamType) ([]byte, error) {
	return []byte("%PDF"), nil
}
