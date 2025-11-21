package cart

import "context"

type Repository interface {
	AddOrUpdateItem(ctx context.Context, userID int64, productID int64, quantity int64) error
	ListItems(ctx context.Context, userID int64) ([]Item, error)
	Clear(ctx context.Context, userID int64) error
}

