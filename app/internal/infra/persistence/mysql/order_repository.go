package mysql

import (
	"context"
	"database/sql"
	"errors"

	domcart "example.com/my-golang-sample/app/internal/domain/cart"
	domorder "example.com/my-golang-sample/app/internal/domain/order"
)

type OrderRepository struct {
	db *sql.DB
}

func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

func (r *OrderRepository) CreateFromCart(ctx context.Context, userID int64, items []domcart.Item, payment domorder.PaymentMethod) (_ *domorder.Order, retErr error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		if retErr != nil {
			_ = tx.Rollback()
		}
	}()

	var total float64
	orderItems := make([]domorder.OrderItem, 0, len(items))

	for _, item := range items {
		var name string
		var price float64
		var stock int64

		row := tx.QueryRowContext(ctx, `
            SELECT name, price, stock
            FROM products
            WHERE id = ?
            FOR UPDATE
        `, item.ProductID)
		if err = row.Scan(&name, &price, &stock); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				retErr = domorder.ErrCheckoutValidation
				return nil, retErr
			}
			retErr = err
			return nil, retErr
		}

		if stock < item.Quantity {
			retErr = domorder.ErrCheckoutValidation
			return nil, retErr
		}

		total += price * float64(item.Quantity)
		orderItems = append(orderItems, domorder.OrderItem{
			ProductID: item.ProductID,
			Name:      name,
			Price:     price,
			Quantity:  item.Quantity,
		})
	}

	res, err := tx.ExecContext(ctx, `
        INSERT INTO orders (user_id, status, payment_method, total_amount)
        VALUES (?, ?, ?, ?)
    `, userID, domorder.StatusPending, payment, total)
	if err != nil {
		retErr = err
		return nil, retErr
	}
	orderID, _ := res.LastInsertId()

	for _, item := range orderItems {
		_, err = tx.ExecContext(ctx, `
            INSERT INTO order_items (order_id, product_id, product_name, unit_price, quantity)
            VALUES (?, ?, ?, ?, ?)
        `, orderID, item.ProductID, item.Name, item.Price, item.Quantity)
		if err != nil {
			retErr = err
			return nil, retErr
		}
		_, err = tx.ExecContext(ctx, `
            UPDATE products SET stock = stock - ?
            WHERE id = ?
        `, item.Quantity, item.ProductID)
		if err != nil {
			retErr = err
			return nil, retErr
		}
	}

	if err = tx.Commit(); err != nil {
		retErr = err
		return nil, retErr
	}

	return r.GetByID(ctx, orderID)
}

func (r *OrderRepository) List(ctx context.Context) ([]*domorder.Order, error) {
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, user_id, status, payment_method, total_amount, created_at
        FROM orders
        ORDER BY id DESC
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*domorder.Order
	for rows.Next() {
		var o domorder.Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.Status, &o.PaymentMethod, &o.TotalAmount, &o.CreatedAt); err != nil {
			return nil, err
		}
		items, err := r.listOrderItems(ctx, o.ID)
		if err != nil {
			return nil, err
		}
		o.Items = items
		orders = append(orders, &o)
	}
	return orders, nil
}

func (r *OrderRepository) GetByID(ctx context.Context, id int64) (*domorder.Order, error) {
	row := r.db.QueryRowContext(ctx, `
        SELECT id, user_id, status, payment_method, total_amount, created_at
        FROM orders WHERE id = ?
    `, id)

	var o domorder.Order
	if err := row.Scan(&o.ID, &o.UserID, &o.Status, &o.PaymentMethod, &o.TotalAmount, &o.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domorder.ErrOrderNotFound
		}
		return nil, err
	}
	items, err := r.listOrderItems(ctx, o.ID)
	if err != nil {
		return nil, err
	}
	o.Items = items
	return &o, nil
}

func (r *OrderRepository) UpdateStatus(ctx context.Context, id int64, status domorder.Status) (*domorder.Order, error) {
	res, err := r.db.ExecContext(ctx, `
        UPDATE orders SET status = ? WHERE id = ?
    `, status, id)
	if err != nil {
		return nil, err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, domorder.ErrOrderNotFound
	}
	return r.GetByID(ctx, id)
}

func (r *OrderRepository) listOrderItems(ctx context.Context, orderID int64) ([]domorder.OrderItem, error) {
	rows, err := r.db.QueryContext(ctx, `
        SELECT id, order_id, product_id, product_name, unit_price, quantity
        FROM order_items WHERE order_id = ?
    `, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []domorder.OrderItem
	for rows.Next() {
		var item domorder.OrderItem
		if err := rows.Scan(&item.ID, &item.OrderID, &item.ProductID, &item.Name, &item.Price, &item.Quantity); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
