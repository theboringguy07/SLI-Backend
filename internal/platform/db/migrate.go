package db

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sli/backend/internal/domain"
)

// roleDescriptions is used to seed the roles table. Keep in sync with
// domain.AllRoles and the CHECK constraint on roles.role_name in
// database/schema/schema.sql.
var roleDescriptions = map[domain.RoleName]string{
	domain.RoleStudent:     "Student enrolled in an internship",
	domain.RoleFaculty:     "Faculty member acting as an internship mentor",
	domain.RoleCoordinator: "Coordinator who enrolls students and assigns faculty mentors",
	domain.RoleAdmin:       "Administrator with full access, including HOD-level statistics",
}

// Migrate does not run any schema migration.
//
// The production schema is managed by versioned SQL files under
// database/schema. Apply those migrations with psql or a migration tool during
// deployment, then start the API against the migrated database.
//
// It does seed the fixed set of roles the application depends on so a freshly
// migrated database is usable without a separate manual seeding step.
func Migrate(pool *pgxpool.Pool) error {
	slog.Info("Skipping schema migration; apply database/schema/schema.sql to the database")
	return seedRoles(pool)
}

// seedRoles ensures every role in domain.AllRoles exists in the roles table.
// It is idempotent (safe to run on every startup, via ON CONFLICT DO NOTHING)
// and never overwrites an existing row's ID, so existing user.role_id
// foreign keys stay valid.
func seedRoles(pool *pgxpool.Pool) error {
	ctx := context.Background()
	for _, name := range domain.AllRoles {
		_, err := pool.Exec(ctx, `
			INSERT INTO roles (role_name, description) VALUES ($1, $2)
			ON CONFLICT (role_name) DO NOTHING`,
			name, roleDescriptions[name],
		)
		if err != nil {
			return err
		}
	}
	slog.Info("Role seeding complete", "roles", domain.AllRoles)
	return nil
}
