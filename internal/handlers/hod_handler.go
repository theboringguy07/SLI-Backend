package handlers

import (
	"net/http"

	"github.com/sli/backend/internal/http/response"
	"github.com/sli/backend/internal/repositories"
)

type HODHandler struct {
	internshipRepo repositories.InternshipRepository
	// other repos as needed
}

func NewHODHandler(internshipRepo repositories.InternshipRepository) *HODHandler {
	return &HODHandler{
		internshipRepo: internshipRepo,
	}
}

func (h *HODHandler) GetStatistics(w http.ResponseWriter, r *http.Request) {
	internships, count, err := h.internshipRepo.ListAll(r.Context(), 0, 10000)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	active := 0
	completed := 0
	for _, internship := range internships {
		switch internship.Status {
		case "active":
			active++
		case "completed":
			completed++
		}
	}

	stats := map[string]interface{}{
		"total_internships":     count,
		"active_internships":    active,
		"completed_internships": completed,
	}

	response.JSON(w, http.StatusOK, stats)
}

func (h *HODHandler) GetOverview(w http.ResponseWriter, r *http.Request) {
	internships, _, err := h.internshipRepo.ListAll(r.Context(), 0, 10000)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	companyCounts := make(map[string]int)
	for _, internship := range internships {
		companyCounts[internship.CompanyName]++
	}

	overview := make([]map[string]interface{}, 0, len(companyCounts))
	for company, students := range companyCounts {
		overview = append(overview, map[string]interface{}{
			"company":  company,
			"students": students,
		})
	}

	response.JSON(w, http.StatusOK, overview)
}
