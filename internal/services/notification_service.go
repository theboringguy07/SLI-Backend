package services

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/repositories"
)

type NotificationService interface {
	NotifyFacultyReportSubmitted(ctx context.Context, facultyEmail string, studentName string, week int) error
	NotifyIndustryMentorReviewLink(ctx context.Context, mentorEmail string, studentName string, rollNo string, companyName string, rawToken string) error
	NotifyStudentWeeklyReminder(ctx context.Context, studentEmail string, week int) error
	// NotifyMagicLink emails a one-click passwordless sign-in link. The link
	// points at this backend's own /api/auth/magic-link/verify endpoint (see
	// apiBaseURL) rather than the frontend - the backend consumes the token,
	// sets session cookies, and only then redirects to the frontend
	// dashboard, mirroring the Google OAuth callback flow.
	NotifyMagicLink(ctx context.Context, email string, rawToken string, expiryMinutes int) error
	// Evaluation related notifications can be added here
}

type notificationService struct {
	outboxRepo  repositories.OutboxRepository
	frontendURL string
	apiBaseURL  string
}

func NewNotificationService(outboxRepo repositories.OutboxRepository, frontendURL string, apiBaseURL string) NotificationService {
	return &notificationService{
		outboxRepo:  outboxRepo,
		frontendURL: frontendURL,
		apiBaseURL:  apiBaseURL,
	}
}

func (s *notificationService) NotifyFacultyReportSubmitted(ctx context.Context, facultyEmail string, studentName string, week int) error {
	subject := fmt.Sprintf("Report Submitted - Week %d by %s", week, studentName)
	body := fmt.Sprintf("<p>Hello,</p><p>Your student <b>%s</b> has submitted their weekly report for week <b>%d</b>.</p><p>Please log in to review it.</p>", studentName, week)

	email := &domain.EmailOutbox{
		Recipient:    facultyEmail,
		Subject:      subject,
		Body:         body,
		TemplateKey:  "faculty_report_submitted",
		TemplateData: mustTemplateData(map[string]interface{}{"student_name": studentName, "week": week}),
	}

	return s.outboxRepo.Enqueue(ctx, email)
}

func (s *notificationService) NotifyIndustryMentorReviewLink(ctx context.Context, mentorEmail string, studentName string, rollNo string, companyName string, rawToken string) error {
	subject := fmt.Sprintf("Review Request: Weekly Report for %s", studentName)
	link := fmt.Sprintf("%s/industry/reports/%s", s.frontendURL, rawToken)

	body := fmt.Sprintf(`
		<p>Hello,</p>
		<p>Please review the weekly report submitted by your intern:</p>
		<ul>
			<li><b>Student Name:</b> %s</li>
			<li><b>Roll No:</b> %s</li>
			<li><b>Internship:</b> %s</li>
		</ul>
		<p>Click the link below to view the report and submit your feedback. This link is valid for 24 hours.</p>
		<p><a href="%s">%s</a></p>
	`, studentName, rollNo, companyName, link, link)

	email := &domain.EmailOutbox{
		Recipient:    mentorEmail,
		Subject:      subject,
		Body:         body,
		TemplateKey:  "industry_review_link",
		TemplateData: mustTemplateData(map[string]interface{}{"student_name": studentName, "roll_no": rollNo, "company_name": companyName, "review_url": link}),
	}

	return s.outboxRepo.Enqueue(ctx, email)
}

func (s *notificationService) NotifyStudentWeeklyReminder(ctx context.Context, studentEmail string, week int) error {
	subject := fmt.Sprintf("Reminder: Submit Weekly Report - Week %d", week)
	body := fmt.Sprintf("<p>Hello,</p><p>This is a reminder to submit your weekly report for week <b>%d</b>.</p><p>Please log in to the portal to submit it before the deadline.</p>", week)

	email := &domain.EmailOutbox{
		Recipient:    studentEmail,
		Subject:      subject,
		Body:         body,
		TemplateKey:  "student_weekly_reminder",
		TemplateData: mustTemplateData(map[string]interface{}{"week": week}),
	}

	return s.outboxRepo.Enqueue(ctx, email)
}

func (s *notificationService) NotifyMagicLink(ctx context.Context, email string, rawToken string, expiryMinutes int) error {
	link := fmt.Sprintf("%s/api/auth/magic-link/verify?token=%s", s.apiBaseURL, rawToken)
	subject := "Your sign-in link"
	body := fmt.Sprintf(`
		<p>Hello,</p>
		<p>Click the link below to sign in. This link is valid for %d minutes and can only be used once.</p>
		<p><a href="%s">%s</a></p>
		<p>If you didn't request this, you can safely ignore this email.</p>
	`, expiryMinutes, link, link)

	outboxEmail := &domain.EmailOutbox{
		Recipient:    email,
		Subject:      subject,
		Body:         body,
		TemplateKey:  "magic_link_signin",
		TemplateData: mustTemplateData(map[string]interface{}{"expiry_minutes": expiryMinutes}),
	}

	return s.outboxRepo.Enqueue(ctx, outboxEmail)
}

func mustTemplateData(data map[string]interface{}) string {
	encoded, err := json.Marshal(data)
	if err != nil {
		return "{}"
	}
	return string(encoded)
}
