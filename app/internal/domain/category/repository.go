package category

import "context"

type Repository interface {
	Create(ctx context.Context, c *Category) (*Category, error)
	Update(ctx context.Context, c *Category) (*Category, error)
	Delete(ctx context.Context, id int64) error
	GetByID(ctx context.Context, id int64) (*Category, error)
	List(ctx context.Context, filter ListFilter) ([]*Category, error)
}

