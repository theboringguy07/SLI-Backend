package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sli/backend/internal/domain"
	"github.com/sli/backend/internal/http/middleware"
	"github.com/sli/backend/internal/http/response"
	"github.com/sli/backend/internal/platform/auth"
	"github.com/sli/backend/internal/platform/errors"
	"github.com/sli/backend/internal/repositories"
)

type AdminHandler struct {
	auditRepo repositories.AuditRepository
	userRepo  repositories.UserRepository
}

func NewAdminHandler(auditRepo repositories.AuditRepository, userRepo repositories.UserRepository) *AdminHandler {
	return &AdminHandler{
		auditRepo: auditRepo,
		userRepo:  userRepo,
	}
}

func (h *AdminHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}

	logs, count, err := h.auditRepo.ListAll(r.Context(), offset, limit)
	if err != nil {
		response.Error(w, r, errors.NewWithErr(errors.CodeInternalServer, "failed to list audit logs", err))
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"data":  logs,
		"total": count,
	})
}

func (h *AdminHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 50
	}

	users, count, err := h.userRepo.ListUsers(r.Context(), offset, limit)
	if err != nil {
		response.Error(w, r, errors.NewWithErr(errors.CodeInternalServer, "failed to list users", err))
		return
	}

	response.JSON(w, http.StatusOK, map[string]interface{}{
		"data":  users,
		"total": count,
	})
}

// UpdateUserProfile is PATCH /api/admin/users/{userID} (admin-only). It sets
// department on a user - a field Google OAuth never populates, needed so
// marksheets can display a student's department
// (internal/platform/pdf/generator.go). May be omitted to leave it unchanged.
func (h *AdminHandler) UpdateUserProfile(w http.ResponseWriter, r *http.Request) {
	claims, _ := r.Context().Value(middleware.UserClaimsKey).(*auth.JWTClaims)

	userIDStr := chi.URLParam(r, "userID")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		response.Error(w, r, errors.New(errors.CodeBadRequest, "Invalid user ID"))
		return
	}

	var req struct {
		Department *string `json:"department"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.Error(w, r, errors.New(errors.CodeBadRequest, "Invalid JSON payload"))
		return
	}

	updates := map[string]interface{}{}
	if req.Department != nil {
		updates["department"] = *req.Department
	}
	if len(updates) == 0 {
		response.Error(w, r, errors.New(errors.CodeValidationFailed, "provide department"))
		return
	}

	if err := h.userRepo.UpdateProfileFields(r.Context(), userID, updates); err != nil {
		if err == repositories.ErrUserNotFound {
			response.Error(w, r, errors.New(errors.CodeNotFound, "user not found"))
			return
		}
		response.Error(w, r, errors.NewWithErr(errors.CodeInternalServer, "failed to update user profile", err))
		return
	}

	meta, _ := json.Marshal(updates)
	auditLog := &domain.AuditLog{
		ActorUserID:  claims.UserID,
		ActorName:    claims.Email,
		Action:       "update_user_profile",
		ResourceType: "users",
		ResourceID:   userID,
		MetadataJSON: string(meta),
		CreatedAt:    time.Now(),
	}
	_ = h.auditRepo.Create(r.Context(), auditLog)

	response.JSON(w, http.StatusOK, map[string]string{"message": "profile updated"})
}
