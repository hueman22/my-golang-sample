package order

import (
	"context"

	domcart "example.com/my-golang-sample/app/internal/domain/cart"
)

type Repository interface {
	CreateFromCart(ctx context.Context, userID int64, items []domcart.Item, payment PaymentMethod) (*Order, error)
	List(ctx context.Context) ([]*Order, error)
	GetByID(ctx context.Context, id int64) (*Order, error)
	UpdateStatus(ctx context.Context, id int64, status Status) (*Order, error)
}

