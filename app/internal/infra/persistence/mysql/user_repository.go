package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	dom "example.com/my-golang-sample/app/internal/domain/user"
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
			return 0, fmt.Errorf("role not found: %s", code)
		}
		return 0, err
	}
	return id, nil
}

func (r *UserRepository) Create(ctx context.Context, u *dom.User) (*dom.User, error) {
	res, err := r.db.ExecContext(ctx,
		`INSERT INTO users (name, email, password_hash, user_role_id)
         VALUES (?, ?, ?, ?)`,
		u.Name, u.Email, u.Password, u.UserRoleID,
	)
	if err != nil {
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
	if err := row.Scan(&u.ID, &u.Name, &u.Email, &u.Password, &u.UserRoleID, &roleCode); err != nil {
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
    `, u.Name, u.Email, u.Password, u.UserRoleID, u.ID)
	if err != nil {
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
