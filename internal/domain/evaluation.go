package domain

import (
	"time"

	"github.com/google/uuid"
)

// EvaluationSchedule is scoped per (InternshipID, ExamType): an internship
// has at most one schedule for its ISE and one for its ESE.
type EvaluationSchedule struct {
	ID            uuid.UUID `json:"id"`
	InternshipID  uuid.UUID `json:"internship_id"`
	ExamType      ExamType  `json:"exam_type"`
	InSemesterAt  time.Time `json:"in_semester_at"`
	EndSemesterAt time.Time `json:"end_semester_at"`
	Venue         string    `json:"venue"`
	SetBy         uuid.UUID `json:"set_by"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	Internship *Internship `json:"internship,omitempty"`
}

// EvaluationScore is likewise scoped per (InternshipID, ExamType): an
// internship gets one locked score for its ISE and a separate one for its
// ESE.
type EvaluationScore struct {
	ID                  uuid.UUID  `json:"id"`
	InternshipID        uuid.UUID  `json:"internship_id"`
	ExamType            ExamType   `json:"exam_type"`
	ReportQuality       int        `json:"report_quality"`
	OralPresentation    int        `json:"oral_presentation"`
	WorkQuality         int        `json:"work_quality"`
	Understanding       int        `json:"understanding"`
	PeriodicInteraction int        `json:"periodic_interaction"`
	Remarks             string     `json:"remarks"`
	LockedAt            *time.Time `json:"locked_at"`
	SubmittedBy         uuid.UUID  `json:"submitted_by"`
	SubmittedAt         time.Time  `json:"submitted_at"`

	Internship *Internship `json:"internship,omitempty"`
}

type EvaluationCorrection struct {
	ID                uuid.UUID `json:"id"`
	EvaluationScoreID uuid.UUID `json:"evaluation_score_id"`
	OldScoresJSON     string    `json:"old_scores_json"`
	NewScoresJSON     string    `json:"new_scores_json"`
	Reason            string    `json:"reason"`
	CorrectedBy       uuid.UUID `json:"corrected_by"`
	CorrectedAt       time.Time `json:"corrected_at"`

	EvaluationScore *EvaluationScore `json:"evaluation_score,omitempty"`
}
