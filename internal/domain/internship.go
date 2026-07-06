package domain

import (
	"time"

	"github.com/google/uuid"
)

type Internship struct {
	ID                  uuid.UUID        `json:"id"`
	StudentID           uuid.UUID        `json:"student_id"`
	CompanyName         string           `json:"company_name"`
	CompanyAddress      string           `json:"company_address"`
	RoleTitle           string           `json:"role_title"`
	IndustryMentorName  string           `json:"industry_mentor_name"`
	IndustryMentorEmail string           `json:"industry_mentor_email"`
	AcademicYear        string           `json:"academic_year"`
	StartDate           time.Time        `json:"start_date"`
	EndDate             time.Time        `json:"end_date"`
	Status              InternshipStatus `json:"status"`
	CreatedBy           *uuid.UUID       `json:"created_by"`
	UpdatedBy           *uuid.UUID       `json:"updated_by"`
	CreatedAt           time.Time        `json:"created_at"`
	UpdatedAt           time.Time        `json:"updated_at"`

	Student *User `json:"student,omitempty"`
}

type MentorAssignment struct {
	ID              uuid.UUID        `json:"id"`
	InternshipID    uuid.UUID        `json:"internship_id"`
	FacultyMentorID uuid.UUID        `json:"faculty_mentor_id"`
	ApprovedAt      *time.Time       `json:"approved_at"`
	ApprovedBy      *uuid.UUID       `json:"approved_by"`
	Status          AssignmentStatus `json:"status"`

	Internship    *Internship `json:"internship,omitempty"`
	FacultyMentor *User       `json:"faculty_mentor,omitempty"`
}
