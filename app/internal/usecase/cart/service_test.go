package cart

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	domcart "example.com/my-golang-sample/app/internal/domain/cart"
	domorder "example.com/my-golang-sample/app/internal/domain/order"
	domproduct "example.com/my-golang-sample/app/internal/domain/product"
)

type mockCartRepository struct {
	itemsByUser map[int64][]domcart.Item
	addErr      error
	listErr     error
	clearErr    error
}

func newMockCartRepository() *mockCartRepository {
	return &mockCartRepository{
		itemsByUser: make(map[int64][]domcart.Item),
	}
}

func (m *mockCartRepository) AddOrUpdateItem(ctx context.Context, userID int64, productID int64, quantity int64) error {
	if m.addErr != nil {
		return m.addErr
	}

	items := m.itemsByUser[userID]
	found := false
	for i, item := range items {
		if item.ProductID == productID {
			items[i].Quantity += quantity
			found = true
			break
		}
	}
	if !found {
		items = append(items, domcart.Item{
			ProductID: productID,
			Quantity:  quantity,
		})
	}
	m.itemsByUser[userID] = items
	return nil
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
	delete(m.itemsByUser, userID)
	return nil
}

type mockProductRepository struct {
	products map[int64]*domproduct.Product
	getErr   error
}

func newMockProductRepository() *mockProductRepository {
	return &mockProductRepository{
		products: make(map[int64]*domproduct.Product),
	}
}

func (m *mockProductRepository) GetByID(ctx context.Context, id int64) (*domproduct.Product, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if product, ok := m.products[id]; ok {
		cloned := *product
		return &cloned, nil
	}
	return nil, domproduct.ErrProductNotFound
}

func (m *mockProductRepository) GetByIDs(ctx context.Context, ids []int64) ([]*domproduct.Product, error) {
	var result []*domproduct.Product
	for _, id := range ids {
		if product, ok := m.products[id]; ok {
			cloned := *product
			result = append(result, &cloned)
		}
	}
	return result, nil
}

type mockOrderRepository struct{}

func (m *mockOrderRepository) CreateFromCart(ctx context.Context, userID int64, items []domcart.Item, payment domorder.PaymentMethod) (*domorder.Order, error) {
	return nil, nil
}

func TestAddItem_ValidProductAndQuantity(t *testing.T) {
	cartRepo := newMockCartRepository()
	productRepo := newMockProductRepository()
	orderRepo := &mockOrderRepository{}

	// Setup: Add a valid product
	productRepo.products[1] = &domproduct.Product{
		ID:       1,
		Name:     "Laptop",
		Price:    999.99,
		Stock:    10,
		IsActive: true,
	}

	svc := NewService(cartRepo, productRepo, orderRepo)

	err := svc.AddToCart(context.Background(), 100, 1, 3)

	require.NoError(t, err)

	// Verify item was added
	items, err := cartRepo.ListItems(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(1), items[0].ProductID)
	require.Equal(t, int64(3), items[0].Quantity)
}

func TestAddItem_ProductNotFound(t *testing.T) {
	cartRepo := newMockCartRepository()
	productRepo := newMockProductRepository()
	orderRepo := &mockOrderRepository{}

	svc := NewService(cartRepo, productRepo, orderRepo)

	err := svc.AddToCart(context.Background(), 100, 999, 1)

	require.ErrorIs(t, err, domproduct.ErrProductNotFound)

	// Verify no item was added
	items, err := cartRepo.ListItems(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, items, 0)
}

func TestAddItem_ProductInactive(t *testing.T) {
	cartRepo := newMockCartRepository()
	productRepo := newMockProductRepository()
	orderRepo := &mockOrderRepository{}

	// Setup: Add an inactive product
	productRepo.products[1] = &domproduct.Product{
		ID:       1,
		Name:     "Inactive Product",
		Price:    99.99,
		Stock:    10,
		IsActive: false,
	}

	svc := NewService(cartRepo, productRepo, orderRepo)

	err := svc.AddToCart(context.Background(), 100, 1, 1)

	require.ErrorIs(t, err, domproduct.ErrProductNotFound)

	// Verify no item was added
	items, err := cartRepo.ListItems(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, items, 0)
}

func TestAddItem_InvalidQuantity(t *testing.T) {
	tests := []struct {
		name     string
		quantity int64
	}{
		{
			name:     "Zero quantity",
			quantity: 0,
		},
		{
			name:     "Negative quantity",
			quantity: -1,
		},
		{
			name:     "Large negative quantity",
			quantity: -100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cartRepo := newMockCartRepository()
			productRepo := newMockProductRepository()
			orderRepo := &mockOrderRepository{}

			svc := NewService(cartRepo, productRepo, orderRepo)

			err := svc.AddToCart(context.Background(), 100, 1, tt.quantity)

			require.Error(t, err)
			require.Contains(t, err.Error(), "quantity must be positive")

			// Verify no item was added
			items, err := cartRepo.ListItems(context.Background(), 100)
			require.NoError(t, err)
			require.Len(t, items, 0)
		})
	}
}

func TestAddItem_OutOfStock(t *testing.T) {
	cartRepo := newMockCartRepository()
	productRepo := newMockProductRepository()
	orderRepo := &mockOrderRepository{}

	// Setup: Add a product with limited stock
	productRepo.products[1] = &domproduct.Product{
		ID:       1,
		Name:     "Limited Stock Product",
		Price:    99.99,
		Stock:    5,
		IsActive: true,
	}

	svc := NewService(cartRepo, productRepo, orderRepo)

	// Try to add more than available stock
	err := svc.AddToCart(context.Background(), 100, 1, 10)

	require.ErrorIs(t, err, domproduct.ErrOutOfStock)

	// Verify no item was added
	items, err := cartRepo.ListItems(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, items, 0)
}

func TestAddItem_UpdateExistingItem(t *testing.T) {
	cartRepo := newMockCartRepository()
	productRepo := newMockProductRepository()
	orderRepo := &mockOrderRepository{}

	// Setup: Add a product with sufficient stock
	productRepo.products[1] = &domproduct.Product{
		ID:       1,
		Name:     "Product",
		Price:    99.99,
		Stock:    20,
		IsActive: true,
	}

	svc := NewService(cartRepo, productRepo, orderRepo)

	// Add item first time
	err := svc.AddToCart(context.Background(), 100, 1, 3)
	require.NoError(t, err)

	// Add same product again (should update quantity)
	err = svc.AddToCart(context.Background(), 100, 1, 2)
	require.NoError(t, err)

	// Verify quantity was updated (3 + 2 = 5)
	items, err := cartRepo.ListItems(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(1), items[0].ProductID)
	require.Equal(t, int64(5), items[0].Quantity)
}

func TestAddItem_UpdateExistingItemExceedsStock(t *testing.T) {
	cartRepo := newMockCartRepository()
	productRepo := newMockProductRepository()
	orderRepo := &mockOrderRepository{}

	// Setup: Add a product with limited stock
	productRepo.products[1] = &domproduct.Product{
		ID:       1,
		Name:     "Limited Product",
		Price:    99.99,
		Stock:    5,
		IsActive: true,
	}

	svc := NewService(cartRepo, productRepo, orderRepo)

	// Add item first time
	err := svc.AddToCart(context.Background(), 100, 1, 3)
	require.NoError(t, err)

	// Try to add more than remaining stock (3 + 3 = 6 > 5)
	err = svc.AddToCart(context.Background(), 100, 1, 3)
	require.ErrorIs(t, err, domproduct.ErrOutOfStock)

	// Verify quantity was not updated
	items, err := cartRepo.ListItems(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(3), items[0].Quantity, "quantity should remain at 3")
}

func TestGetCart_ByUserID_ReturnsOnlyUserItems(t *testing.T) {
	cartRepo := newMockCartRepository()
	productRepo := newMockProductRepository()
	orderRepo := &mockOrderRepository{}

	// Setup: Add products
	productRepo.products[1] = &domproduct.Product{
		ID:       1,
		Name:     "Product 1",
		Price:    99.99,
		Stock:    10,
		IsActive: true,
	}
	productRepo.products[2] = &domproduct.Product{
		ID:       2,
		Name:     "Product 2",
		Price:    149.99,
		Stock:    5,
		IsActive: true,
	}

	svc := NewService(cartRepo, productRepo, orderRepo)

	// Add items for user 100
	err := svc.AddToCart(context.Background(), 100, 1, 2)
	require.NoError(t, err)
	err = svc.AddToCart(context.Background(), 100, 2, 1)
	require.NoError(t, err)

	// Add items for user 200
	err = svc.AddToCart(context.Background(), 200, 1, 5)
	require.NoError(t, err)

	// Get cart for user 100
	cart, err := svc.GetCart(context.Background(), 100)

	require.NoError(t, err)
	require.NotNil(t, cart)
	require.Equal(t, int64(100), cart.UserID)
	require.Len(t, cart.Items, 2)

	// Verify items belong to user 100
	productIDs := make(map[int64]bool)
	for _, item := range cart.Items {
		productIDs[item.ProductID] = true
		require.Greater(t, item.Quantity, int64(0))
		require.NotEmpty(t, item.ProductName)
		require.Greater(t, item.ProductPrice, 0.0)
	}
	require.True(t, productIDs[1], "should contain product 1")
	require.True(t, productIDs[2], "should contain product 2")

	// Get cart for user 200
	cart2, err := svc.GetCart(context.Background(), 200)

	require.NoError(t, err)
	require.NotNil(t, cart2)
	require.Equal(t, int64(200), cart2.UserID)
	require.Len(t, cart2.Items, 1)
	require.Equal(t, int64(1), cart2.Items[0].ProductID)
	require.Equal(t, int64(5), cart2.Items[0].Quantity)
}

func TestGetCart_EmptyCart(t *testing.T) {
	cartRepo := newMockCartRepository()
	productRepo := newMockProductRepository()
	orderRepo := &mockOrderRepository{}

	svc := NewService(cartRepo, productRepo, orderRepo)

	cart, err := svc.GetCart(context.Background(), 100)

	require.NoError(t, err)
	require.NotNil(t, cart)
	require.Equal(t, int64(100), cart.UserID)
	require.Len(t, cart.Items, 0)
}

func TestGetCart_WithMultipleProducts(t *testing.T) {
	cartRepo := newMockCartRepository()
	productRepo := newMockProductRepository()
	orderRepo := &mockOrderRepository{}

	// Setup: Add multiple products
	productRepo.products[1] = &domproduct.Product{
		ID:       1,
		Name:     "Laptop",
		Price:    999.99,
		Stock:    10,
		IsActive: true,
	}
	productRepo.products[2] = &domproduct.Product{
		ID:       2,
		Name:     "Mouse",
		Price:    29.99,
		Stock:    50,
		IsActive: true,
	}
	productRepo.products[3] = &domproduct.Product{
		ID:       3,
		Name:     "Keyboard",
		Price:    79.99,
		Stock:    30,
		IsActive: true,
	}

	svc := NewService(cartRepo, productRepo, orderRepo)

	// Add multiple items
	err := svc.AddToCart(context.Background(), 100, 1, 1)
	require.NoError(t, err)
	err = svc.AddToCart(context.Background(), 100, 2, 2)
	require.NoError(t, err)
	err = svc.AddToCart(context.Background(), 100, 3, 1)
	require.NoError(t, err)

	// Get cart
	cart, err := svc.GetCart(context.Background(), 100)

	require.NoError(t, err)
	require.NotNil(t, cart)
	require.Len(t, cart.Items, 3)

	// Verify all items have correct details
	itemMap := make(map[int64]domcart.DetailedItem)
	for _, item := range cart.Items {
		itemMap[item.ProductID] = item
	}

	require.Equal(t, "Laptop", itemMap[1].ProductName)
	require.Equal(t, 999.99, itemMap[1].ProductPrice)
	require.Equal(t, int64(1), itemMap[1].Quantity)

	require.Equal(t, "Mouse", itemMap[2].ProductName)
	require.Equal(t, 29.99, itemMap[2].ProductPrice)
	require.Equal(t, int64(2), itemMap[2].Quantity)

	require.Equal(t, "Keyboard", itemMap[3].ProductName)
	require.Equal(t, 79.99, itemMap[3].ProductPrice)
	require.Equal(t, int64(1), itemMap[3].Quantity)
}

func TestAddItem_ExactStockLimit(t *testing.T) {
	cartRepo := newMockCartRepository()
	productRepo := newMockProductRepository()
	orderRepo := &mockOrderRepository{}

	// Setup: Add a product with exact stock
	productRepo.products[1] = &domproduct.Product{
		ID:       1,
		Name:     "Exact Stock Product",
		Price:    99.99,
		Stock:    5,
		IsActive: true,
	}

	svc := NewService(cartRepo, productRepo, orderRepo)

	// Add exactly the available stock
	err := svc.AddToCart(context.Background(), 100, 1, 5)

	require.NoError(t, err)

	// Verify item was added
	items, err := cartRepo.ListItems(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.Equal(t, int64(5), items[0].Quantity)
}

func TestAddItem_DifferentUsersIsolated(t *testing.T) {
	cartRepo := newMockCartRepository()
	productRepo := newMockProductRepository()
	orderRepo := &mockOrderRepository{}

	// Setup: Add a product
	productRepo.products[1] = &domproduct.Product{
		ID:       1,
		Name:     "Shared Product",
		Price:    99.99,
		Stock:    100,
		IsActive: true,
	}

	svc := NewService(cartRepo, productRepo, orderRepo)

	// Add item for user 100
	err := svc.AddToCart(context.Background(), 100, 1, 3)
	require.NoError(t, err)

	// Add item for user 200
	err = svc.AddToCart(context.Background(), 200, 1, 7)
	require.NoError(t, err)

	// Verify each user has their own cart
	cart100, err := svc.GetCart(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, cart100.Items, 1)
	require.Equal(t, int64(3), cart100.Items[0].Quantity)

	cart200, err := svc.GetCart(context.Background(), 200)
	require.NoError(t, err)
	require.Len(t, cart200.Items, 1)
	require.Equal(t, int64(7), cart200.Items[0].Quantity)
}

