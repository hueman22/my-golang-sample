package order

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	domcart "example.com/my-golang-sample/app/internal/domain/cart"
	domorder "example.com/my-golang-sample/app/internal/domain/order"
)

type mockOrderRepository struct {
	orders     map[int64]*domorder.Order
	nextID     int64
	updated    map[int64]*domorder.Order
	listErr    error
	getErr     error
	updateErr  error
}

func newMockOrderRepository() *mockOrderRepository {
	return &mockOrderRepository{
		orders:  make(map[int64]*domorder.Order),
		nextID:  1,
		updated: make(map[int64]*domorder.Order),
	}
}

func (m *mockOrderRepository) CreateFromCart(ctx context.Context, userID int64, items []domcart.Item, payment domorder.PaymentMethod) (*domorder.Order, error) {
	return nil, nil
}

func (m *mockOrderRepository) List(ctx context.Context) ([]*domorder.Order, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	result := make([]*domorder.Order, 0, len(m.orders))
	for _, order := range m.orders {
		cloned := *order
		result = append(result, &cloned)
	}
	return result, nil
}

func (m *mockOrderRepository) GetByID(ctx context.Context, id int64) (*domorder.Order, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if order, ok := m.orders[id]; ok {
		cloned := *order
		return &cloned, nil
	}
	return nil, domorder.ErrOrderNotFound
}

func (m *mockOrderRepository) UpdateStatus(ctx context.Context, id int64, status domorder.Status) (*domorder.Order, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}
	order, ok := m.orders[id]
	if !ok {
		return nil, domorder.ErrOrderNotFound
	}
	order.Status = status
	m.orders[id] = order
	m.updated[id] = order
	cloned := *order
	return &cloned, nil
}

func TestGetOrder_NotFound(t *testing.T) {
	repo := newMockOrderRepository()
	svc := NewService(repo)

	order, err := svc.GetByID(context.Background(), 999)

	require.ErrorIs(t, err, domorder.ErrOrderNotFound)
	require.Nil(t, order)
}

func TestGetOrder_Found(t *testing.T) {
	repo := newMockOrderRepository()
	expectedOrder := &domorder.Order{
		ID:            1,
		UserID:        100,
		Status:        domorder.StatusPending,
		PaymentMethod: domorder.PaymentCOD,
		TotalAmount:   199.98,
		Items: []domorder.OrderItem{
			{
				ID:        1,
				OrderID:   1,
				ProductID: 1,
				Name:      "Laptop",
				Price:     999.99,
				Quantity:  2,
			},
		},
		CreatedAt: time.Now(),
	}
	repo.orders[1] = expectedOrder

	svc := NewService(repo)

	order, err := svc.GetByID(context.Background(), 1)

	require.NoError(t, err)
	require.NotNil(t, order)
	require.Equal(t, int64(1), order.ID)
	require.Equal(t, int64(100), order.UserID)
	require.Equal(t, domorder.StatusPending, order.Status)
	require.Equal(t, domorder.PaymentCOD, order.PaymentMethod)
	require.Equal(t, 199.98, order.TotalAmount)
	require.Len(t, order.Items, 1)
	require.Equal(t, "Laptop", order.Items[0].Name)
}

func TestUpdateOrderStatus_InvalidStatus(t *testing.T) {
	tests := []struct {
		name  string
		status domorder.Status
	}{
		{
			name:  "Empty status",
			status: domorder.Status(""),
		},
		{
			name:  "Invalid status",
			status: domorder.Status("INVALID"),
		},
		{
			name:  "Processing status (not allowed)",
			status: domorder.Status("PROCESSING"),
		},
		{
			name:  "Delivered status (not allowed)",
			status: domorder.Status("DELIVERED"),
		},
		{
			name:  "Lowercase pending",
			status: domorder.Status("pending"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockOrderRepository()
			repo.orders[1] = &domorder.Order{
				ID:     1,
				UserID: 100,
				Status: domorder.StatusPending,
			}

			svc := NewService(repo)

			order, err := svc.UpdateStatus(context.Background(), 1, tt.status)

			require.ErrorIs(t, err, domorder.ErrInvalidStatus)
			require.Nil(t, order)
			require.Equal(t, domorder.StatusPending, repo.orders[1].Status, "order status should not be updated")
			require.Nil(t, repo.updated[1], "order should not be marked as updated")
		})
	}
}

func TestUpdateOrderStatus_OrderNotFound(t *testing.T) {
	repo := newMockOrderRepository()
	svc := NewService(repo)

	order, err := svc.UpdateStatus(context.Background(), 999, domorder.StatusPaid)

	require.ErrorIs(t, err, domorder.ErrOrderNotFound)
	require.Nil(t, order)
	require.Empty(t, repo.updated, "no orders should be updated")
}

func TestUpdateOrderStatus_Valid(t *testing.T) {
	tests := []struct {
		name         string
		initialStatus domorder.Status
		newStatus     domorder.Status
	}{
		{
			name:          "PENDING to PAID",
			initialStatus: domorder.StatusPending,
			newStatus:     domorder.StatusPaid,
		},
		{
			name:          "PAID to SHIPPED",
			initialStatus: domorder.StatusPaid,
			newStatus:     domorder.StatusShipped,
		},
		{
			name:          "PENDING to CANCELED",
			initialStatus: domorder.StatusPending,
			newStatus:     domorder.StatusCanceled,
		},
		{
			name:          "PAID to CANCELED",
			initialStatus: domorder.StatusPaid,
			newStatus:     domorder.StatusCanceled,
		},
		{
			name:          "Same status (PENDING to PENDING)",
			initialStatus: domorder.StatusPending,
			newStatus:     domorder.StatusPending,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockOrderRepository()
			repo.orders[1] = &domorder.Order{
				ID:            1,
				UserID:        100,
				Status:        tt.initialStatus,
				PaymentMethod: domorder.PaymentCOD,
				TotalAmount:   199.98,
				Items:         []domorder.OrderItem{},
				CreatedAt:     time.Now(),
			}

			svc := NewService(repo)

			order, err := svc.UpdateStatus(context.Background(), 1, tt.newStatus)

			require.NoError(t, err)
			require.NotNil(t, order)
			require.Equal(t, tt.newStatus, order.Status)
			require.Equal(t, tt.newStatus, repo.orders[1].Status, "order status should be updated in repository")
			require.NotNil(t, repo.updated[1], "order should be marked as updated")
		})
	}
}

func TestListOrders_Empty(t *testing.T) {
	repo := newMockOrderRepository()
	svc := NewService(repo)

	orders, err := svc.List(context.Background())

	require.NoError(t, err)
	require.NotNil(t, orders)
	require.Empty(t, orders)
}

func TestListOrders_WithMultipleOrders(t *testing.T) {
	repo := newMockOrderRepository()
	repo.orders[1] = &domorder.Order{
		ID:            1,
		UserID:        100,
		Status:        domorder.StatusPending,
		PaymentMethod: domorder.PaymentCOD,
		TotalAmount:   99.99,
		Items:         []domorder.OrderItem{},
		CreatedAt:     time.Now(),
	}
	repo.orders[2] = &domorder.Order{
		ID:            2,
		UserID:        200,
		Status:        domorder.StatusPaid,
		PaymentMethod: domorder.PaymentTamara,
		TotalAmount:   199.98,
		Items:         []domorder.OrderItem{},
		CreatedAt:     time.Now(),
	}
	repo.orders[3] = &domorder.Order{
		ID:            3,
		UserID:        100,
		Status:        domorder.StatusShipped,
		PaymentMethod: domorder.PaymentCOD,
		TotalAmount:   299.97,
		Items:         []domorder.OrderItem{},
		CreatedAt:     time.Now(),
	}

	svc := NewService(repo)

	orders, err := svc.List(context.Background())

	require.NoError(t, err)
	require.NotNil(t, orders)
	require.Len(t, orders, 3)

	// Verify all orders are returned
	orderIDs := make(map[int64]bool)
	for _, order := range orders {
		orderIDs[order.ID] = true
		require.Greater(t, order.ID, int64(0))
		require.Greater(t, order.UserID, int64(0))
	}
	require.True(t, orderIDs[1], "should contain order 1")
	require.True(t, orderIDs[2], "should contain order 2")
	require.True(t, orderIDs[3], "should contain order 3")
}

func TestGetOrder_WithItems(t *testing.T) {
	repo := newMockOrderRepository()
	expectedOrder := &domorder.Order{
		ID:            1,
		UserID:        100,
		Status:        domorder.StatusPending,
		PaymentMethod: domorder.PaymentCOD,
		TotalAmount:   499.95,
		Items: []domorder.OrderItem{
			{
				ID:        1,
				OrderID:   1,
				ProductID: 1,
				Name:      "Laptop",
				Price:     999.99,
				Quantity:  1,
			},
			{
				ID:        2,
				OrderID:   1,
				ProductID: 2,
				Name:      "Mouse",
				Price:     29.99,
				Quantity:  2,
			},
		},
		CreatedAt: time.Now(),
	}
	repo.orders[1] = expectedOrder

	svc := NewService(repo)

	order, err := svc.GetByID(context.Background(), 1)

	require.NoError(t, err)
	require.NotNil(t, order)
	require.Len(t, order.Items, 2)
	require.Equal(t, "Laptop", order.Items[0].Name)
	require.Equal(t, "Mouse", order.Items[1].Name)
	require.Equal(t, int64(1), order.Items[0].Quantity)
	require.Equal(t, int64(2), order.Items[1].Quantity)
}

func TestUpdateOrderStatus_AllValidStatuses(t *testing.T) {
	validStatuses := []domorder.Status{
		domorder.StatusPending,
		domorder.StatusPaid,
		domorder.StatusShipped,
		domorder.StatusCanceled,
	}

	for _, status := range validStatuses {
		t.Run(string(status), func(t *testing.T) {
			repo := newMockOrderRepository()
			repo.orders[1] = &domorder.Order{
				ID:     1,
				UserID: 100,
				Status: domorder.StatusPending,
			}

			svc := NewService(repo)

			order, err := svc.UpdateStatus(context.Background(), 1, status)

			require.NoError(t, err)
			require.NotNil(t, order)
			require.Equal(t, status, order.Status)
		})
	}
}

func TestListOrders_RepositoryError(t *testing.T) {
	repo := newMockOrderRepository()
	repo.listErr = domorder.ErrOrderNotFound
	svc := NewService(repo)

	orders, err := svc.List(context.Background())

	require.ErrorIs(t, err, domorder.ErrOrderNotFound)
	require.Nil(t, orders)
}

func TestGetOrder_RepositoryError(t *testing.T) {
	repo := newMockOrderRepository()
	repo.getErr = domorder.ErrOrderNotFound
	svc := NewService(repo)

	order, err := svc.GetByID(context.Background(), 1)

	require.ErrorIs(t, err, domorder.ErrOrderNotFound)
	require.Nil(t, order)
}

func TestUpdateOrderStatus_RepositoryError(t *testing.T) {
	repo := newMockOrderRepository()
	repo.orders[1] = &domorder.Order{
		ID:     1,
		UserID: 100,
		Status: domorder.StatusPending,
	}
	repo.updateErr = domorder.ErrOrderNotFound
	svc := NewService(repo)

	order, err := svc.UpdateStatus(context.Background(), 1, domorder.StatusPaid)

	require.ErrorIs(t, err, domorder.ErrOrderNotFound)
	require.Nil(t, order)
}

