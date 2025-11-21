package product

import "errors"

var (
	ErrProductNotFound = errors.New("product not found")
	ErrOutOfStock      = errors.New("product out of stock")
)

