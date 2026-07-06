package repositories

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/sli/backend/internal/domain"
)

var ErrUserNotFound = errors.New("user not found")

type UserRepository interface {
	FindByGoogleSub(ctx context.Context, sub string) (*domain.User, error)
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindRoleByName(ctx context.Context, roleName domain.RoleName) (*domain.Role, error)
	CreateUser(ctx context.Context, user *domain.User) error
	UpdateUser(ctx context.Context, user *domain.User) error
	// LinkGoogleSub attaches a Google account to a user who previously only
	// existed via magic-link signup (see AuthService.HandleOAuthCallback).
	LinkGoogleSub(ctx context.Context, userID uuid.UUID, sub string) error
	SetRole(ctx context.Context, userID uuid.UUID, roleName domain.RoleName) error
	ListUsers(ctx context.Context, offset, limit int) ([]domain.User, int64, error)
	// ListByRole is a narrower, unpaginated listing for use cases like a
	// coordinator picking a faculty mentor - callers who need "everyone with
	// this role", not admin's paginated full user listing.
	ListByRole(ctx context.Context, roleName domain.RoleName) ([]domain.User, error)
	// UpdateProfileFields applies a partial update (only the keys present in
	// updates, e.g. "department") to a single user row. Used by
	// PATCH /api/admin/users/{userID} since that field isn't set anywhere
	// during Google OAuth login.
	UpdateProfileFields(ctx context.Context, userID uuid.UUID, updates map[string]interface{}) error
}

type userRepository struct {
	db dbtx
}

func NewUserRepository(db dbtx) UserRepository {
	return &userRepository{db: db}
}

// userSelectCols joins users to roles so every read gets Role populated in
// one round trip, matching the previous Preload("Role") behavior.
const userSelectCols = `
	u.id, u.google_sub, u.email, u.display_name, u.role_id, u.department, u.created_at, u.last_login_at,
	r.id, r.role_name, r.description`

const userSelectFrom = `FROM users u JOIN roles r ON r.id = u.role_id`

func scanUser(row pgx.Row) (*domain.User, error) {
	var u domain.User
	var department *string
	err := row.Scan(
		&u.ID, &u.GoogleSub, &u.Email, &u.Name, &u.RoleID, &department, &u.CreatedAt, &u.LastLoginAt,
		&u.Role.ID, &u.Role.Name, &u.Role.Description,
	)
	if err != nil {
		return nil, err
	}
	if department != nil {
		u.Department = *department
	}
	return &u, nil
}

func (r *userRepository) FindByGoogleSub(ctx context.Context, sub string) (*domain.User, error) {
	row := r.db.QueryRow(ctx, `SELECT `+userSelectCols+` `+userSelectFrom+` WHERE u.google_sub = $1`, sub)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	row := r.db.QueryRow(ctx, `SELECT `+userSelectCols+` `+userSelectFrom+` WHERE u.id = $1`, id)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	row := r.db.QueryRow(ctx, `SELECT `+userSelectCols+` `+userSelectFrom+` WHERE u.email = $1`, email)
	user, err := scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return user, nil
}

func (r *userRepository) FindRoleByName(ctx context.Context, roleName domain.RoleName) (*domain.Role, error) {
	var role domain.Role
	err := r.db.QueryRow(ctx, `SELECT id, role_name, description FROM roles WHERE role_name = $1`, roleName).
		Scan(&role.ID, &role.Name, &role.Description)
	if err != nil {
		return nil, err
	}
	return &role, nil
}

func (r *userRepository) CreateUser(ctx context.Context, user *domain.User) error {
	return r.db.QueryRow(ctx, `
		INSERT INTO users (google_sub, email, display_name, role_id, department)
		VALUES ($1, $2, $3, $4, NULLIF($5, ''))
		RETURNING id, created_at`,
		user.GoogleSub, user.Email, user.Name, user.RoleID, user.Department,
	).Scan(&user.ID, &user.CreatedAt)
}

// UpdateUser persists the fields Google OAuth login can actually change on
// an existing user (display name, email, last login time) - role and
// enrollment/department have their own dedicated update paths (SetRole,
// UpdateProfileFields) and are intentionally left untouched here.
func (r *userRepository) UpdateUser(ctx context.Context, user *domain.User) error {
	_, err := r.db.Exec(ctx, `
		UPDATE users SET display_name = $1, email = $2, last_login_at = $3
		WHERE id = $4`,
		user.Name, user.Email, user.LastLoginAt, user.ID,
	)
	return err
}

func (r *userRepository) LinkGoogleSub(ctx context.Context, userID uuid.UUID, sub string) error {
	_, err := r.db.Exec(ctx, `UPDATE users SET google_sub = $1 WHERE id = $2`, sub, userID)
	return err
}

func (r *userRepository) SetRole(ctx context.Context, userID uuid.UUID, roleName domain.RoleName) error {
	role, err := r.FindRoleByName(ctx, roleName) // Role must exist in DB (seed data)
	if err != nil {
		return err
	}

	_, err = r.db.Exec(ctx, `UPDATE users SET role_id = $1 WHERE id = $2`, role.ID, userID)
	return err
}

// UpdateProfileFields applies a partial update built from a caller-supplied
// map. Only "department" is a valid column to touch this way (see
// AdminHandler.UpdateUserProfile) - anything else is ignored rather than
// risking an injected column name.
func (r *userRepository) UpdateProfileFields(ctx context.Context, userID uuid.UUID, updates map[string]interface{}) error {
	if len(updates) == 0 {
		return nil
	}

	allowed := map[string]bool{"department": true}

	setClauses := make([]string, 0, len(updates))
	args := make([]interface{}, 0, len(updates)+1)
	i := 1
	for _, col := range []string{"department"} {
		val, ok := updates[col]
		if !ok || !allowed[col] {
			continue
		}
		i++
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", col, i))
		args = append(args, val)
	}
	if len(setClauses) == 0 {
		return nil
	}

	args = append([]interface{}{userID}, args...)
	sql := fmt.Sprintf(`UPDATE users SET %s WHERE id = $1`, strings.Join(setClauses, ", "))

	tag, err := r.db.Exec(ctx, sql, args...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

func (r *userRepository) ListByRole(ctx context.Context, roleName domain.RoleName) ([]domain.User, error) {
	rows, err := r.db.Query(ctx, `SELECT `+userSelectCols+` `+userSelectFrom+`
		WHERE r.role_name = $1 ORDER BY u.display_name ASC`, roleName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []domain.User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		users = append(users, *user)
	}
	return users, rows.Err()
}

func (r *userRepository) ListUsers(ctx context.Context, offset, limit int) ([]domain.User, int64, error) {
	var count int64
	if err := r.db.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(ctx, `SELECT `+userSelectCols+` `+userSelectFrom+`
		ORDER BY u.created_at ASC OFFSET $1 LIMIT $2`, offset, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	users := []domain.User{}
	for rows.Next() {
		user, err := scanUser(rows)
		if err != nil {
			return nil, 0, err
		}
		users = append(users, *user)
	}
	return users, count, rows.Err()
}
