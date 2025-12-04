package checkout

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	domcart "example.com/my-golang-sample/app/internal/domain/cart"
	domorder "example.com/my-golang-sample/app/internal/domain/order"
)

type mockCartRepository struct {
	itemsByUser map[int64][]domcart.Item
	listErr     error
	clearErr    error
	cleared     map[int64]bool
}

func newMockCartRepository() *mockCartRepository {
	return &mockCartRepository{
		itemsByUser: make(map[int64][]domcart.Item),
		cleared:     make(map[int64]bool),
	}
}

func (m *mockCartRepository) ListItems(ctx context.Context, userID int64) ([]domcart.Item, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	items := m.itemsByUser[userID]
	if items == nil {
		return []domcart.Item{}, nil
	}
	result := make([]domcart.Item, len(items))
	copy(result, items)
	return result, nil
}

func (m *mockCartRepository) Clear(ctx context.Context, userID int64) error {
	if m.clearErr != nil {
		return m.clearErr
	}
	m.cleared[userID] = true
	delete(m.itemsByUser, userID)
	return nil
}

type mockOrderRepository struct {
	createdOrder *domorder.Order
	createErr    error
}

func newMockOrderRepository() *mockOrderRepository {
	return &mockOrderRepository{}
}

func (m *mockOrderRepository) CreateFromCart(ctx context.Context, userID int64, items []domcart.Item, payment domorder.PaymentMethod) (*domorder.Order, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if m.createdOrder != nil {
		return m.createdOrder, nil
	}
	// Create a default order if none provided
	return &domorder.Order{
		ID:            1,
		UserID:        userID,
		Status:        domorder.StatusPending,
		PaymentMethod: payment,
		TotalAmount:   0,
		Items:         []domorder.OrderItem{},
	}, nil
}

func TestCheckout_WithEmptyCart_ReturnsError(t *testing.T) {
	cartRepo := newMockCartRepository()
	orderRepo := newMockOrderRepository()

	svc := NewService(cartRepo, orderRepo)

	order, err := svc.Checkout(context.Background(), 100, domorder.PaymentCOD)

	require.ErrorIs(t, err, domorder.ErrEmptyOrderItems)
	require.Nil(t, order)
	require.Nil(t, orderRepo.createdOrder, "order should not be created for empty cart")
	require.False(t, cartRepo.cleared[100], "cart should not be cleared for empty cart")
}

func TestCheckout_WithInvalidPaymentMethod_ReturnsError(t *testing.T) {
	tests := []struct {
		name          string
		paymentMethod domorder.PaymentMethod
	}{
		{
			name:          "Empty payment method",
			paymentMethod: domorder.PaymentMethod(""),
		},
		{
			name:          "Invalid payment method",
			paymentMethod: domorder.PaymentMethod("INVALID"),
		},
		{
			name:          "Credit card (not supported)",
			paymentMethod: domorder.PaymentMethod("CREDIT_CARD"),
		},
		{
			name:          "PayPal (not supported)",
			paymentMethod: domorder.PaymentMethod("PAYPAL"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cartRepo := newMockCartRepository()
			// Setup: Add items to cart
			cartRepo.itemsByUser[100] = []domcart.Item{
				{ProductID: 1, Quantity: 2},
			}
			orderRepo := newMockOrderRepository()

			svc := NewService(cartRepo, orderRepo)

			order, err := svc.Checkout(context.Background(), 100, tt.paymentMethod)

			require.ErrorIs(t, err, domorder.ErrInvalidPayment)
			require.Nil(t, order)
			require.Nil(t, orderRepo.createdOrder, "order should not be created for invalid payment method")
			require.False(t, cartRepo.cleared[100], "cart should not be cleared for invalid payment method")
		})
	}
}

func TestCheckout_WithCOD_Succeeds(t *testing.T) {
	cartRepo := newMockCartRepository()
	// Setup: Add items to cart
	cartRepo.itemsByUser[100] = []domcart.Item{
		{ProductID: 1, Quantity: 2},
		{ProductID: 2, Quantity: 1},
	}
	orderRepo := newMockOrderRepository()
	orderRepo.createdOrder = &domorder.Order{
		ID:            1,
		UserID:        100,
		Status:        domorder.StatusPending,
		PaymentMethod: domorder.PaymentCOD,
		TotalAmount:   199.98,
		Items: []domorder.OrderItem{
			{ProductID: 1, Quantity: 2},
			{ProductID: 2, Quantity: 1},
		},
	}

	svc := NewService(cartRepo, orderRepo)

	order, err := svc.Checkout(context.Background(), 100, domorder.PaymentCOD)

	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, domorder.PaymentCOD, order.PaymentMethod)
	require.Equal(t, int64(100), order.UserID)
	require.True(t, cartRepo.cleared[100], "cart should be cleared after successful checkout")
}

func TestCheckout_WithTamara_Succeeds(t *testing.T) {
	cartRepo := newMockCartRepository()
	// Setup: Add items to cart
	cartRepo.itemsByUser[100] = []domcart.Item{
		{ProductID: 1, Quantity: 3},
	}
	orderRepo := newMockOrderRepository()
	orderRepo.createdOrder = &domorder.Order{
		ID:            2,
		UserID:        100,
		Status:        domorder.StatusPending,
		PaymentMethod: domorder.PaymentTamara,
		TotalAmount:   299.97,
		Items: []domorder.OrderItem{
			{ProductID: 1, Quantity: 3},
		},
	}

	svc := NewService(cartRepo, orderRepo)

	order, err := svc.Checkout(context.Background(), 100, domorder.PaymentTamara)

	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, domorder.PaymentTamara, order.PaymentMethod)
	require.Equal(t, int64(100), order.UserID)
	require.True(t, cartRepo.cleared[100], "cart should be cleared after successful checkout")
}

func TestCheckout_CartListItemsError(t *testing.T) {
	cartRepo := newMockCartRepository()
	cartRepo.listErr = domorder.ErrCheckoutValidation
	orderRepo := newMockOrderRepository()

	svc := NewService(cartRepo, orderRepo)

	order, err := svc.Checkout(context.Background(), 100, domorder.PaymentCOD)

	require.ErrorIs(t, err, domorder.ErrCheckoutValidation)
	require.Nil(t, order)
	require.False(t, cartRepo.cleared[100], "cart should not be cleared on error")
}

func TestCheckout_OrderCreationError(t *testing.T) {
	cartRepo := newMockCartRepository()
	cartRepo.itemsByUser[100] = []domcart.Item{
		{ProductID: 1, Quantity: 2},
	}
	orderRepo := newMockOrderRepository()
	orderRepo.createErr = domorder.ErrCheckoutValidation

	svc := NewService(cartRepo, orderRepo)

	order, err := svc.Checkout(context.Background(), 100, domorder.PaymentCOD)

	require.ErrorIs(t, err, domorder.ErrCheckoutValidation)
	require.Nil(t, order)
	require.False(t, cartRepo.cleared[100], "cart should not be cleared if order creation fails")
}

func TestCheckout_CartClearError(t *testing.T) {
	cartRepo := newMockCartRepository()
	cartRepo.itemsByUser[100] = []domcart.Item{
		{ProductID: 1, Quantity: 2},
	}
	cartRepo.clearErr = domorder.ErrCheckoutValidation
	orderRepo := newMockOrderRepository()
	orderRepo.createdOrder = &domorder.Order{
		ID:            1,
		UserID:        100,
		Status:        domorder.StatusPending,
		PaymentMethod: domorder.PaymentCOD,
		TotalAmount:   199.98,
		Items:         []domorder.OrderItem{},
	}

	svc := NewService(cartRepo, orderRepo)

	order, err := svc.Checkout(context.Background(), 100, domorder.PaymentCOD)

	require.ErrorIs(t, err, domorder.ErrCheckoutValidation)
	require.Nil(t, order, "should return error if cart clear fails")
}

func TestCheckout_DifferentUsersIsolated(t *testing.T) {
	cartRepo := newMockCartRepository()
	// Setup: Add items for different users
	cartRepo.itemsByUser[100] = []domcart.Item{
		{ProductID: 1, Quantity: 2},
	}
	cartRepo.itemsByUser[200] = []domcart.Item{
		{ProductID: 2, Quantity: 3},
	}
	orderRepo := newMockOrderRepository()

	svc := NewService(cartRepo, orderRepo)

	// Checkout for user 100
	order1, err := svc.Checkout(context.Background(), 100, domorder.PaymentCOD)
	require.NoError(t, err)
	require.NotNil(t, order1)
	require.True(t, cartRepo.cleared[100], "user 100 cart should be cleared")
	require.False(t, cartRepo.cleared[200], "user 200 cart should not be cleared yet")

	// Checkout for user 200
	order2, err := svc.Checkout(context.Background(), 200, domorder.PaymentTamara)
	require.NoError(t, err)
	require.NotNil(t, order2)
	require.True(t, cartRepo.cleared[200], "user 200 cart should be cleared")
}

func TestCheckout_WithMultipleItems(t *testing.T) {
	cartRepo := newMockCartRepository()
	// Setup: Add multiple items to cart
	cartRepo.itemsByUser[100] = []domcart.Item{
		{ProductID: 1, Quantity: 2},
		{ProductID: 2, Quantity: 1},
		{ProductID: 3, Quantity: 5},
	}
	orderRepo := newMockOrderRepository()
	orderRepo.createdOrder = &domorder.Order{
		ID:            1,
		UserID:        100,
		Status:        domorder.StatusPending,
		PaymentMethod: domorder.PaymentCOD,
		TotalAmount:   499.95,
		Items: []domorder.OrderItem{
			{ProductID: 1, Quantity: 2},
			{ProductID: 2, Quantity: 1},
			{ProductID: 3, Quantity: 5},
		},
	}

	svc := NewService(cartRepo, orderRepo)

	order, err := svc.Checkout(context.Background(), 100, domorder.PaymentCOD)

	require.NoError(t, err)
	require.NotNil(t, order)
	require.Len(t, order.Items, 3)
	require.Equal(t, int64(2), order.Items[0].Quantity)
	require.Equal(t, int64(1), order.Items[1].Quantity)
	require.Equal(t, int64(5), order.Items[2].Quantity)
	require.True(t, cartRepo.cleared[100], "cart should be cleared after successful checkout")
}

func TestCheckout_ValidPaymentMethods(t *testing.T) {
	tests := []struct {
		name          string
		paymentMethod domorder.PaymentMethod
		shouldSucceed bool
	}{
		{
			name:          "COD payment method",
			paymentMethod: domorder.PaymentCOD,
			shouldSucceed: true,
		},
		{
			name:          "Tamara payment method",
			paymentMethod: domorder.PaymentTamara,
			shouldSucceed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cartRepo := newMockCartRepository()
			cartRepo.itemsByUser[100] = []domcart.Item{
				{ProductID: 1, Quantity: 1},
			}
			orderRepo := newMockOrderRepository()

			svc := NewService(cartRepo, orderRepo)

			order, err := svc.Checkout(context.Background(), 100, tt.paymentMethod)

			if tt.shouldSucceed {
				require.NoError(t, err)
				require.NotNil(t, order)
				require.Equal(t, tt.paymentMethod, order.PaymentMethod)
			} else {
				require.Error(t, err)
				require.Nil(t, order)
			}
		})
	}
}

