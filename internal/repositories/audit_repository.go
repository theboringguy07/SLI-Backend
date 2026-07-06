package repositories

import (
	"context"

	"github.com/sli/backend/internal/domain"
)

type AuditRepository interface {
	Create(ctx context.Context, log *domain.AuditLog) error
	ListAll(ctx context.Context, offset, limit int) ([]domain.AuditLog, int64, error)
}

type auditRepository struct {
	db dbtx
}

func NewAuditRepository(db dbtx) AuditRepository {
	return &auditRepository{db: db}
}

func (r *auditRepository) Create(ctx context.Context, log *domain.AuditLog) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO audit_logs (actor_user_id, actor_name, action, resource_type, resource_id, metadata_json, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		log.ActorUserID, log.ActorName, log.Action, log.ResourceType, log.ResourceID, log.MetadataJSON, log.CreatedAt,
	).Scan(&log.ID)
}

func (r *auditRepository) ListAll(ctx context.Context, offset, limit int) ([]domain.AuditLog, int64, error) {
	var count int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM audit_logs`).Scan(&count); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `
		SELECT id, actor_user_id, actor_name, action, resource_type, resource_id, metadata_json, created_at
		FROM audit_logs ORDER BY created_at DESC OFFSET $1 LIMIT $2`, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	logs := []domain.AuditLog{}
	for rows.Next() {
		var l domain.AuditLog
		if err := rows.Scan(&l.ID, &l.ActorUserID, &l.ActorName, &l.Action, &l.ResourceType, &l.ResourceID, &l.MetadataJSON, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		logs = append(logs, l)
	}
	return logs, count, rows.Err()
}
