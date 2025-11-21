package mysql

import (
	"context"
	"database/sql"

	domcart "example.com/my-golang-sample/app/internal/domain/cart"
)

type CartRepository struct {
	db *sql.DB
}

func NewCartRepository(db *sql.DB) *CartRepository {
	return &CartRepository{db: db}
}

func (r *CartRepository) AddOrUpdateItem(ctx context.Context, userID int64, productID int64, quantity int64) error {
	_, err := r.db.ExecContext(ctx, `
        INSERT INTO cart_items (user_id, product_id, quantity)
        VALUES (?, ?, ?)
        ON DUPLICATE KEY UPDATE quantity = quantity + VALUES(quantity)
    `, userID, productID, quantity)
	return err
}

func (r *CartRepository) ListItems(ctx context.Context, userID int64) ([]domcart.Item, error) {
	rows, err := r.db.QueryContext(ctx, `
        SELECT product_id, quantity
        FROM cart_items
        WHERE user_id = ?
    `, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domcart.Item
	for rows.Next() {
		var item domcart.Item
		if err := rows.Scan(&item.ProductID, &item.Quantity); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *CartRepository) Clear(ctx context.Context, userID int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM cart_items WHERE user_id = ?`, userID)
	return err
}

