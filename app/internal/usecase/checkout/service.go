package checkout

import (
	"context"

	domcart "example.com/my-golang-sample/app/internal/domain/cart"
	domorder "example.com/my-golang-sample/app/internal/domain/order"
)

type CartRepository interface {
	ListItems(ctx context.Context, userID int64) ([]domcart.Item, error)
	Clear(ctx context.Context, userID int64) error
}

type OrderRepository interface {
	CreateFromCart(ctx context.Context, userID int64, items []domcart.Item, payment domorder.PaymentMethod) (*domorder.Order, error)
}

type Service struct {
	cartRepo  CartRepository
	orderRepo OrderRepository
}

func NewService(cartRepo CartRepository, orderRepo OrderRepository) *Service {
	return &Service{
		cartRepo:  cartRepo,
		orderRepo: orderRepo,
	}
}

func (s *Service) Checkout(ctx context.Context, userID int64, method domorder.PaymentMethod) (*domorder.Order, error) {
	if !method.IsValid() {
		return nil, domorder.ErrInvalidPayment
	}

	items, err := s.cartRepo.ListItems(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, domorder.ErrEmptyOrderItems
	}

	order, err := s.orderRepo.CreateFromCart(ctx, userID, items, method)
	if err != nil {
		return nil, err
	}

	if err := s.cartRepo.Clear(ctx, userID); err != nil {
		return nil, err
	}

	return order, nil
}

