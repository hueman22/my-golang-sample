package cart

import (
	"context"
	"errors"

	domcart "example.com/my-golang-sample/app/internal/domain/cart"
	domorder "example.com/my-golang-sample/app/internal/domain/order"
	domproduct "example.com/my-golang-sample/app/internal/domain/product"
)

type CartRepository interface {
	domcart.Repository
}

type ProductRepository interface {
	GetByIDs(ctx context.Context, ids []int64) ([]*domproduct.Product, error)
}

type OrderRepository interface {
	CreateFromCart(ctx context.Context, userID int64, items []domcart.Item, payment domorder.PaymentMethod) (*domorder.Order, error)
}

type Service struct {
	cartRepo    CartRepository
	productRepo ProductRepository
	orderRepo   OrderRepository
}

func NewService(cartRepo CartRepository, productRepo ProductRepository, orderRepo OrderRepository) *Service {
	return &Service{
		cartRepo:    cartRepo,
		productRepo: productRepo,
		orderRepo:   orderRepo,
	}
}

func (s *Service) AddToCart(ctx context.Context, userID, productID int64, quantity int64) error {
	if quantity <= 0 {
		return errors.New("quantity must be positive")
	}
	return s.cartRepo.AddOrUpdateItem(ctx, userID, productID, quantity)
}

func (s *Service) GetCart(ctx context.Context, userID int64) (*domcart.Cart, error) {
	items, err := s.cartRepo.ListItems(ctx, userID)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return &domcart.Cart{UserID: userID, Items: []domcart.DetailedItem{}}, nil
	}

	ids := make([]int64, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ProductID)
	}

	products, err := s.productRepo.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	productMap := make(map[int64]*domproduct.Product)
	for _, p := range products {
		productMap[p.ID] = p
	}

	cart := &domcart.Cart{
		UserID: userID,
		Items:  make([]domcart.DetailedItem, 0, len(items)),
	}

	for _, item := range items {
		if p, ok := productMap[item.ProductID]; ok {
			cart.Items = append(cart.Items, domcart.DetailedItem{
				Item: domcart.Item{
					ProductID: item.ProductID,
					Quantity:  item.Quantity,
				},
				ProductName:  p.Name,
				ProductPrice: p.Price,
			})
		}
	}

	return cart, nil
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

