package userrole

import (
	"context"
)

type Repository interface {
	Create(ctx context.Context, role *UserRole) (*UserRole, error)
	Update(ctx context.Context, role *UserRole) (*UserRole, error)
	Delete(ctx context.Context, id int64) error
	GetByID(ctx context.Context, id int64) (*UserRole, error)
	GetByCode(ctx context.Context, code string) (*UserRole, error)
	List(ctx context.Context, filter ListFilter) ([]*UserRole, error)
}

