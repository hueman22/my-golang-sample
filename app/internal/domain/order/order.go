package order

import (
	"time"

	domcart "example.com/my-golang-sample/app/internal/domain/cart"
)

type Status string

const (
	StatusPending  Status = "PENDING"
	StatusPaid     Status = "PAID"
	StatusShipped  Status = "SHIPPED"
	StatusCanceled Status = "CANCELED"
)

func (s Status) IsValid() bool {
	switch s {
	case StatusPending, StatusPaid, StatusShipped, StatusCanceled:
		return true
	default:
		return false
	}
}

type PaymentMethod string

const (
	PaymentCOD    PaymentMethod = "COD"
	PaymentTamara PaymentMethod = "TAMARA"
)

func (p PaymentMethod) IsValid() bool {
	switch p {
	case PaymentCOD, PaymentTamara:
		return true
	default:
		return false
	}
}

type Order struct {
	ID            int64
	UserID        int64
	Status        Status
	PaymentMethod PaymentMethod
	TotalAmount   float64
	Items         []OrderItem
	CreatedAt     time.Time
}

type OrderItem struct {
	ID        int64
	OrderID   int64
	ProductID int64
	Name      string
	Price     float64
	Quantity  int64
}

type CreateFromCartResult struct {
	Order Order
	Cart  domcart.Cart
}

