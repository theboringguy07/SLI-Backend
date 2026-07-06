package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/repositories"
	"github.com/sli/backend/internal/services"
)

type ReminderWorker struct {
	notificationService services.NotificationService
	internshipRepo      repositories.InternshipRepository
	reportRepo          repositories.ReportRepository
}

func NewReminderWorker(notificationService services.NotificationService, internshipRepo repositories.InternshipRepository, reportRepo repositories.ReportRepository) *ReminderWorker {
	return &ReminderWorker{
		notificationService: notificationService,
		internshipRepo:      internshipRepo,
		reportRepo:          reportRepo,
	}
}

func (w *ReminderWorker) Start(ctx context.Context) {
	slog.Info("Starting Reminder Worker")
	// Run once a day to check for missing reports
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping Reminder Worker")
			return
		case <-ticker.C:
			w.sendReminders(ctx)
		}
	}
}

func (w *ReminderWorker) sendReminders(ctx context.Context) {
	internships, _, err := w.internshipRepo.ListAll(ctx, 0, 10000)
	if err != nil {
		slog.Error("Failed to fetch internships for reminders", "err", err)
		return
	}

	sent := 0
	for _, internship := range internships {
		if internship.Status != domain.InternshipActive || internship.Student == nil {
			continue
		}

		week := int(time.Since(internship.StartDate).Hours()/24/7) + 1
		if week < 1 || week > 16 {
			continue
		}

		if _, err := w.reportRepo.FindByInternshipTypeAndPeriod(ctx, internship.ID, domain.ReportTypeWeekly, week); err == nil {
			continue
		} else if err != repositories.ErrReportNotFound {
			slog.Error("Failed to check weekly report before reminder", "internship_id", internship.ID, "week", week, "err", err)
			continue
		}

		if err := w.notificationService.NotifyStudentWeeklyReminder(ctx, internship.Student.Email, week); err != nil {
			slog.Error("Failed to enqueue weekly reminder", "student_id", internship.StudentID, "week", week, "err", err)
			continue
		}
		sent++
	}

	slog.Info("Reminder worker tick completed", "reminders_enqueued", sent)
}
