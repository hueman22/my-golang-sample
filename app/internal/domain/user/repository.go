package user

import "context"

type Repository interface {
	Create(ctx context.Context, u *User) (*User, error)
	GetByID(ctx context.Context, id int64) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	List(ctx context.Context, filter ListUsersFilter) ([]*User, error)
	Update(ctx context.Context, u *User) (*User, error)
	Delete(ctx context.Context, id int64) error

	GetRoleIDByCode(ctx context.Context, code RoleCode) (int64, error)
}
