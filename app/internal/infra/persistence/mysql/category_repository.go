package mysql

import (
	"context"
	"database/sql"
	"errors"

	domcategory "example.com/my-golang-sample/app/internal/domain/category"
)

type CategoryRepository struct {
	db *sql.DB
}

func NewCategoryRepository(db *sql.DB) *CategoryRepository {
	return &CategoryRepository{db: db}
}

func (r *CategoryRepository) Create(ctx context.Context, c *domcategory.Category) (*domcategory.Category, error) {
	res, err := r.db.ExecContext(ctx, `
        INSERT INTO categories (name, description, is_active)
        VALUES (?, ?, ?)
    `, c.Name, c.Description, c.IsActive)
	if err != nil {
		return nil, err
	}
	c.ID, _ = res.LastInsertId()
	return c, nil
}

func (r *CategoryRepository) Update(ctx context.Context, c *domcategory.Category) (*domcategory.Category, error) {
	res, err := r.db.ExecContext(ctx, `
        UPDATE categories SET name = ?, description = ?, is_active = ?
        WHERE id = ?
    `, c.Name, c.Description, c.IsActive, c.ID)
	if err != nil {
		return nil, err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return nil, domcategory.ErrCategoryNotFound
	}
	return c, nil
}

func (r *CategoryRepository) Delete(ctx context.Context, id int64) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM categories WHERE id = ?`, id)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return domcategory.ErrCategoryNotFound
	}
	return nil
}

func (r *CategoryRepository) GetByID(ctx context.Context, id int64) (*domcategory.Category, error) {
	row := r.db.QueryRowContext(ctx, `
        SELECT id, name, description, is_active
        FROM categories
        WHERE id = ?
    `, id)

	var c domcategory.Category
	if err := row.Scan(&c.ID, &c.Name, &c.Description, &c.IsActive); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domcategory.ErrCategoryNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *CategoryRepository) List(ctx context.Context, filter domcategory.ListFilter) ([]*domcategory.Category, error) {
	query := `SELECT id, name, description, is_active FROM categories`
	args := []any{}
	if filter.OnlyActive {
		query += ` WHERE is_active = 1`
	}
	query += ` ORDER BY id DESC`

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []*domcategory.Category
	for rows.Next() {
		var c domcategory.Category
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.IsActive); err != nil {
			return nil, err
		}
		categories = append(categories, &c)
	}
	return categories, nil
}

