package order

import "errors"

var (
	ErrOrderNotFound      = errors.New("order not found")
	ErrInvalidStatus      = errors.New("invalid order status")
	ErrInvalidPayment     = errors.New("invalid payment method")
	ErrEmptyOrderItems    = errors.New("no items to checkout")
	ErrCheckoutValidation = errors.New("checkout validation failed")
)

