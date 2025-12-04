package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	dom "example.com/my-golang-sample/app/internal/domain/user"
	domrole "example.com/my-golang-sample/app/internal/domain/userrole"
)

type UserRepository struct {
	db *sql.DB
}

func NewUserRepository(db *sql.DB) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) GetRoleIDByCode(ctx context.Context, code dom.RoleCode) (int64, error) {
	var id int64
	err := r.db.QueryRowContext(ctx,
		`SELECT id FROM user_roles WHERE code = ?`,
		string(code),
	).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("role not found: %w", domrole.ErrRoleNotFound)
		}
		return 0, err
	}
	return id, nil
}

func (r *UserRepository) Create(ctx context.Context, u *dom.User) (*dom.User, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO users (name, email, password_hash, user_role_id)
         VALUES (?, ?, ?, ?)`,
		u.Name, u.Email, u.PasswordHash, u.UserRoleID,
	)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, dom.ErrEmailAlreadyUsed
		}
		return nil, err
	}
	id, _ := res.LastInsertId()
	u.ID = id
	return u, nil
}

func (r *UserRepository) GetByID(ctx context.Context, id int64) (*dom.User, error) {
	row := r.db.QueryRowContext(ctx, `
        SELECT u.id, u.name, u.email, u.password_hash, u.user_role_id, r.code
        FROM users u
        JOIN user_roles r ON u.user_role_id = r.id
        WHERE u.id = ?
    `, id)

	var u dom.User
	var roleCode string
	if err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.UserRoleID, &roleCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dom.ErrUserNotFound
		}
		return nil, err
	}
	u.RoleCode = dom.RoleCode(roleCode)
	return &u, nil
}

func (r *UserRepository) Update(ctx context.Context, u *dom.User) (*dom.User, error) {
	res, err := r.db.ExecContext(ctx, `
        UPDATE users
        SET name = ?, email = ?, password_hash = ?, user_role_id = ?
        WHERE id = ?
    `, u.Name, u.Email, u.PasswordHash, u.UserRoleID, u.ID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, dom.ErrEmailAlreadyUsed
		}
		return nil, err
	}

	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, dom.ErrUserNotFound
	}

	return u, nil
}

func (r *UserRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return dom.ErrUserNotFound
	}
	return nil
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*dom.User, error) {
	row := r.db.QueryRowContext(ctx, `
        SELECT u.id, u.name, u.email, u.password_hash, u.user_role_id, r.code
        FROM users u
        JOIN user_roles r ON u.user_role_id = r.id
        WHERE u.email = ?
    `, email)

	var u dom.User
	var roleCode string
	if err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.UserRoleID, &roleCode); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, dom.ErrUserNotFound
		}
		return nil, err
	}
	u.RoleCode = dom.RoleCode(roleCode)
	return &u, nil
}

func (r *UserRepository) List(ctx context.Context, filter dom.ListUsersFilter) ([]*dom.User, error) {
	query := `
        SELECT u.id, u.name, u.email, u.password_hash, u.user_role_id, r.code
        FROM users u
        JOIN user_roles r ON u.user_role_id = r.id
    `
	args := []any{}
	if filter.RoleCode != nil {
		query += ` WHERE r.code = ?`
		args = append(args, string(*filter.RoleCode))
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []*dom.User
	for rows.Next() {
		var u dom.User
		var roleCode string
		if err := rows.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.UserRoleID, &roleCode); err != nil {
			return nil, err
		}
		u.RoleCode = dom.RoleCode(roleCode)
		users = append(users, &u)
	}
	return users, nil
}
