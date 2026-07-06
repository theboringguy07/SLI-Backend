package domain

import (
	"time"

	"github.com/google/uuid"
)

// ReportType tags a report as one of the three cadences a student submits
// during the 16-week internship. The PeriodNumber (stored in the week_number
// column) is scoped per type: weekly 1-16, fortnightly 1-8, monthly 1-4.
type ReportType string

const (
	ReportTypeWeekly      ReportType = "weekly"
	ReportTypeFortnightly ReportType = "fortnightly"
	ReportTypeMonthly     ReportType = "monthly"
)

// MaxPeriod returns the highest valid period number for a given report type
// over a 16-week internship. Returns 0 for an unknown type.
func (t ReportType) MaxPeriod() int {
	switch t {
	case ReportTypeWeekly:
		return 16
	case ReportTypeFortnightly:
		return 8
	case ReportTypeMonthly:
		return 4
	default:
		return 0
	}
}

// Valid reports whether t is one of the known report types.
func (t ReportType) Valid() bool {
	return t.MaxPeriod() > 0
}

// WeeklyReport maps to the canonical "reports" table (it now holds weekly,
// fortnightly and monthly reports, despite the Go type name).
type WeeklyReport struct {
	ID             uuid.UUID    `json:"id"`
	InternshipID   uuid.UUID    `json:"internship_id"`
	ReportType     ReportType   `json:"report_type"`
	WeekNumber     int          `json:"week_number"`
	Content        string       `json:"content"`
	Status         ReportStatus `json:"status"`
	SubmittedAt    time.Time    `json:"submitted_at"`
	EditedAt       time.Time    `json:"edited_at"`
	ApprovedAt     *time.Time   `json:"approved_at"`
	ApprovedBy     *uuid.UUID   `json:"approved_by"`
	ReminderSentAt *time.Time   `json:"reminder_sent_at"`
	CreatedBy      uuid.UUID    `json:"created_by"`
	UpdatedBy      uuid.UUID    `json:"updated_by"`

	Internship *Internship `json:"internship,omitempty"`
}

// ReportFeedback maps to the canonical "feedback" table. Feedback is per-report;
// faculty feedback sets GivenBy, industry feedback sets IndustryEmail.
type ReportFeedback struct {
	ID            uuid.UUID      `json:"id"`
	ReportID      uuid.UUID      `json:"report_id"`
	Source        FeedbackSource `json:"source"`
	GivenBy       *uuid.UUID     `json:"given_by,omitempty"`
	IndustryEmail string         `json:"industry_email,omitempty"`
	Comments      string         `json:"comments"`
	Rating        *int           `json:"rating,omitempty"`
	SubmittedAt   time.Time      `json:"submitted_at"`
	CreatedAt     time.Time      `json:"created_at"`

	Report *WeeklyReport `json:"report,omitempty"`
}
