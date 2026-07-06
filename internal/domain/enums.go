package domain

type RoleName string

// Role values must exactly match roles.role_name in database/schema/schema.sql,
// which constrains the column to STUDENT, FACULTY, COORDINATOR, ADMIN.
// There is no separate HOD or SUPER_ADMIN role at the database level: HOD-only
// and superadmin-only endpoints are gated behind RoleAdmin.
const (
	RoleStudent     RoleName = "STUDENT"
	RoleFaculty     RoleName = "FACULTY"
	RoleCoordinator RoleName = "COORDINATOR"
	RoleAdmin       RoleName = "ADMIN"
)

// AllRoles lists every role that must exist in the roles table.
var AllRoles = []RoleName{RoleStudent, RoleFaculty, RoleCoordinator, RoleAdmin}

type InternshipStatus string

const (
	InternshipActive    InternshipStatus = "active"
	InternshipCompleted InternshipStatus = "completed"
	InternshipCancelled InternshipStatus = "cancelled"
)

type AssignmentStatus string

const (
	AssignmentPending  AssignmentStatus = "pending"
	AssignmentApproved AssignmentStatus = "approved"
	AssignmentRejected AssignmentStatus = "rejected"
)

type ReportStatus string

const (
	ReportStatusDraft     ReportStatus = "draft"
	ReportStatusSubmitted ReportStatus = "submitted"
	ReportStatusReviewed  ReportStatus = "reviewed"
)

type FeedbackSource string

const (
	FeedbackSourceFaculty  FeedbackSource = "faculty"
	FeedbackSourceIndustry FeedbackSource = "industry"
)

type OutboxStatus string

const (
	OutboxPending OutboxStatus = "pending"
	OutboxSent    OutboxStatus = "sent"
	OutboxFailed  OutboxStatus = "failed"
)

// ExamType distinguishes the two evaluation checkpoints an internship gets.
// A given internship has at most one EvaluationSchedule and one
// EvaluationScore per ExamType (see internal/domain/evaluation.go).
type ExamType string

const (
	ExamTypeISE ExamType = "ISE" // In-Semester Exam
	ExamTypeESE ExamType = "ESE" // End-Semester Exam
)

// AllExamTypes lists every valid ExamType value (mirrors the CHECK
// constraint on evaluation_schedules.exam_type / evaluation_scores.exam_type
// in database/schema/schema.sql).
var AllExamTypes = []ExamType{ExamTypeISE, ExamTypeESE}
