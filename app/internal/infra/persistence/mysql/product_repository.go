package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	domproduct "example.com/my-golang-sample/app/internal/domain/product"
)

type ProductRepository struct {
	db *sql.DB
}

func NewProductRepository(db *sql.DB) *ProductRepository {
	return &ProductRepository{db: db}
}

func (r *ProductRepository) Create(ctx context.Context, p *domproduct.Product) (*domproduct.Product, error) {
	res, err := r.db.ExecContext(ctx, `
        INSERT INTO products (name, description, price, stock, category_id, is_active)
        VALUES (?, ?, ?, ?, ?, ?)
    `, p.Name, p.Description, p.Price, p.Stock, p.CategoryID, p.IsActive)
	if err != nil {
		return nil, err
	}
	p.ID, _ = res.LastInsertId()
	return p, nil
}

func (r *ProductRepository) Update(ctx context.Context, p *domproduct.Product) (*domproduct.Product, error) {
	res, err := r.db.ExecContext(ctx, `
        UPDATE products SET name = ?, description = ?, price = ?, stock = ?, category_id = ?, is_active = ?
        WHERE id = ?
    `, p.Name, p.Description, p.Price, p.Stock, p.CategoryID, p.IsActive, p.ID)
	if err != nil {
		return nil, err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, domproduct.ErrProductNotFound
	}
	return p, nil
}

func (r *ProductRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM products WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return domproduct.ErrProductNotFound
	}
	return nil
}

func (r *ProductRepository) GetByID(ctx context.Context, id int64) (*domproduct.Product, error) {
	row := r.db.QueryRowContext(ctx, `
        SELECT id, name, description, price, stock, category_id, is_active
        FROM products WHERE id = ?
    `, id)

	var p domproduct.Product
	if err := row.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.CategoryID, &p.IsActive); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domproduct.ErrProductNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *ProductRepository) List(ctx context.Context, filter domproduct.ListFilter) ([]*domproduct.Product, error) {
	query := `
        SELECT id, name, description, price, stock, category_id, is_active
        FROM products
    `
	var clauses []string
	var args []any

	if filter.CategoryID != nil {
		clauses = append(clauses, "category_id = ?")
		args = append(args, *filter.CategoryID)
	}
	if filter.Search != "" {
		clauses = append(clauses, "name LIKE ?")
		args = append(args, fmt.Sprintf("%%%s%%", filter.Search))
	}
	if filter.OnlyActive {
		clauses = append(clauses, "is_active = 1")
	}

	if len(clauses) > 0 {
		query += " WHERE " + strings.Join(clauses, " AND ")
	}
	query += " ORDER BY id DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []*domproduct.Product
	for rows.Next() {
		var p domproduct.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.CategoryID, &p.IsActive); err != nil {
			return nil, err
		}
		products = append(products, &p)
	}
	return products, nil
}

func (r *ProductRepository) GetByIDs(ctx context.Context, ids []int64) ([]*domproduct.Product, error) {
	if len(ids) == 0 {
		return []*domproduct.Product{}, nil
	}

	query := `
        SELECT id, name, description, price, stock, category_id, is_active
        FROM products
        WHERE id IN (?` + strings.Repeat(",?", len(ids)-1) + `)
    `

	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []*domproduct.Product
	for rows.Next() {
		var p domproduct.Product
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.CategoryID, &p.IsActive); err != nil {
			return nil, err
		}
		products = append(products, &p)
	}
	return products, nil
}

