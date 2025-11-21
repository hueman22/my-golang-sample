package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	domuser "example.com/my-golang-sample/app/internal/domain/user"
	domrole "example.com/my-golang-sample/app/internal/domain/userrole"
)

type UserRoleRepository struct {
	db *sql.DB
}

func NewUserRoleRepository(db *sql.DB) *UserRoleRepository {
	return &UserRoleRepository{db: db}
}

func (r *UserRoleRepository) Create(ctx context.Context, role *domrole.UserRole) (*domrole.UserRole, error) {
	res, err := r.db.ExecContext(ctx, `
        INSERT INTO user_roles (code, name, description)
        VALUES (?, ?, ?)
    `, role.Code, role.Name, role.Description)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return nil, domrole.ErrRoleCodeExisted
		}
		return nil, err
	}
	role.ID, _ = res.LastInsertId()
	return role, nil
}

func (r *UserRoleRepository) Update(ctx context.Context, role *domrole.UserRole) (*domrole.UserRole, error) {
	res, err := r.db.ExecContext(ctx, `
        UPDATE user_roles
        SET name = ?, description = ?
        WHERE id = ?
    `, role.Name, role.Description, role.ID)
	if err != nil {
		return nil, err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, domrole.ErrRoleNotFound
	}
	return role, nil
}

func (r *UserRoleRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM user_roles WHERE id = ? AND is_system = 0`, id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return domrole.ErrRoleNotFound
	}
	return nil
}

func (r *UserRoleRepository) GetByID(ctx context.Context, id int64) (*domrole.UserRole, error) {
	row := r.db.QueryRowContext(ctx, `
        SELECT id, code, name, description, is_system
        FROM user_roles
        WHERE id = ?
    `, id)

	var role domrole.UserRole
	var code string
	if err := row.Scan(&role.ID, &code, &role.Name, &role.Description, &role.IsSystem); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domrole.ErrRoleNotFound
		}
		return nil, err
	}
	role.Code = domuser.RoleCode(code)
	return &role, nil
}

func (r *UserRoleRepository) GetByCode(ctx context.Context, code string) (*domrole.UserRole, error) {
	row := r.db.QueryRowContext(ctx, `
        SELECT id, code, name, description, is_system
        FROM user_roles
        WHERE code = ?
    `, code)

	var role domrole.UserRole
	var dbCode string
	if err := row.Scan(&role.ID, &dbCode, &role.Name, &role.Description, &role.IsSystem); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domrole.ErrRoleNotFound
		}
		return nil, err
	}
	role.Code = domuser.RoleCode(dbCode)
	return &role, nil
}

func (r *UserRoleRepository) List(ctx context.Context, filter domrole.ListFilter) ([]*domrole.UserRole, error) {
	query := `
        SELECT id, code, name, description, is_system
        FROM user_roles
    `
	var args []any
	if filter.Query != nil {
		query += " WHERE name LIKE ? OR code LIKE ?"
		arg := fmt.Sprintf("%%%s%%", *filter.Query)
		args = append(args, arg, arg)
	}
	query += " ORDER BY id DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []*domrole.UserRole
	for rows.Next() {
		var role domrole.UserRole
		var code string
		if err := rows.Scan(&role.ID, &code, &role.Name, &role.Description, &role.IsSystem); err != nil {
			return nil, err
		}
		role.Code = domuser.RoleCode(code)
		roles = append(roles, &role)
	}
	return roles, nil
}
