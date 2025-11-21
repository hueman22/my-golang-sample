package product

import "context"

type Repository interface {
	Create(ctx context.Context, p *Product) (*Product, error)
	Update(ctx context.Context, p *Product) (*Product, error)
	Delete(ctx context.Context, id int64) error
	GetByID(ctx context.Context, id int64) (*Product, error)
	List(ctx context.Context, filter ListFilter) ([]*Product, error)
	GetByIDs(ctx context.Context, ids []int64) ([]*Product, error)
}

