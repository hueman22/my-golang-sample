package category

import "errors"

var (
	ErrCategoryNotFound    = errors.New("category not found")
	ErrCategoryInvalidName = errors.New("category name is required")
	ErrCategoryInvalidSlug = errors.New("invalid category slug")
	ErrCategorySlugExists  = errors.New("category slug already exists")
)
