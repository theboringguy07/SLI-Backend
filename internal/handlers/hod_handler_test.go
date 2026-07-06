package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/repositories"
)

func TestHODStatisticsUsesInternshipData(t *testing.T) {
	repo := &hodInternshipRepo{internships: []domain.Internship{
		{ID: uuid.New(), Status: domain.InternshipActive, CompanyName: "Acme"},
		{ID: uuid.New(), Status: domain.InternshipCompleted, CompanyName: "Acme"},
		{ID: uuid.New(), Status: domain.InternshipActive, CompanyName: "Globex"},
	}}
	handler := NewHODHandler(repo)

	rr := httptest.NewRecorder()
	handler.GetStatistics(rr, httptest.NewRequest(http.MethodGet, "/api/hod/statistics", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var body map[string]float64
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["total_internships"] != 3 || body["active_internships"] != 2 || body["completed_internships"] != 1 {
		t.Fatalf("unexpected stats: %#v", body)
	}
}

func TestHODOverviewGroupsByCompany(t *testing.T) {
	repo := &hodInternshipRepo{internships: []domain.Internship{
		{ID: uuid.New(), Status: domain.InternshipActive, CompanyName: "Acme"},
		{ID: uuid.New(), Status: domain.InternshipActive, CompanyName: "Acme"},
		{ID: uuid.New(), Status: domain.InternshipActive, CompanyName: "Globex"},
	}}
	handler := NewHODHandler(repo)

	rr := httptest.NewRecorder()
	handler.GetOverview(rr, httptest.NewRequest(http.MethodGet, "/api/hod/overview", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	var rows []map[string]interface{}
	if err := json.NewDecoder(rr.Body).Decode(&rows); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	counts := map[string]float64{}
	for _, row := range rows {
		counts[row["company"].(string)] = row["students"].(float64)
	}
	if counts["Acme"] != 2 || counts["Globex"] != 1 {
		t.Fatalf("unexpected overview: %#v", rows)
	}
}

type hodInternshipRepo struct {
	internships []domain.Internship
}

func (r *hodInternshipRepo) Create(ctx context.Context, internship *domain.Internship) error {
	r.internships = append(r.internships, *internship)
	return nil
}
func (r *hodInternshipRepo) FindByStudentID(ctx context.Context, studentID uuid.UUID) (*domain.Internship, error) {
	return nil, repositories.ErrInternshipNotFound
}
func (r *hodInternshipRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.Internship, error) {
	return nil, repositories.ErrInternshipNotFound
}
func (r *hodInternshipRepo) ListAll(ctx context.Context, offset, limit int) ([]domain.Internship, int64, error) {
	return r.internships, int64(len(r.internships)), nil
}
func (r *hodInternshipRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.InternshipStatus) error {
	return nil
}
