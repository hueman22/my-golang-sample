package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	domcart "example.com/my-golang-sample/app/internal/domain/cart"
	domorder "example.com/my-golang-sample/app/internal/domain/order"
	domuser "example.com/my-golang-sample/app/internal/domain/user"
	"example.com/my-golang-sample/app/internal/infra/security"
	orderuc "example.com/my-golang-sample/app/internal/usecase/order"
)

// --- Mock Repository for Order Tests ---

type mockOrderRepository struct {
	orders    map[int64]*domorder.Order
	nextID    int64
	listErr   error
	getErr    error
	updateErr error
}

func newMockOrderRepository() *mockOrderRepository {
	return &mockOrderRepository{
		orders: map[int64]*domorder.Order{
			1: {
				ID:            1,
				UserID:        100,
				Status:        domorder.StatusPending,
				PaymentMethod: domorder.PaymentCOD,
				TotalAmount:   50.0,
				Items: []domorder.OrderItem{
					{ID: 1, OrderID: 1, ProductID: 1, Name: "Product 1", Price: 10.0, Quantity: 2},
					{ID: 2, OrderID: 1, ProductID: 2, Name: "Product 2", Price: 30.0, Quantity: 1},
				},
				CreatedAt: time.Now(),
			},
			2: {
				ID:            2,
				UserID:        101,
				Status:        domorder.StatusPaid,
				PaymentMethod: domorder.PaymentTamara,
				TotalAmount:   100.0,
				Items: []domorder.OrderItem{
					{ID: 3, OrderID: 2, ProductID: 3, Name: "Product 3", Price: 100.0, Quantity: 1},
				},
				CreatedAt: time.Now(),
			},
			3: {
				ID:            3,
				UserID:        102,
				Status:        domorder.StatusShipped,
				PaymentMethod: domorder.PaymentCOD,
				TotalAmount:   75.0,
				Items: []domorder.OrderItem{
					{ID: 4, OrderID: 3, ProductID: 4, Name: "Product 4", Price: 75.0, Quantity: 1},
				},
				CreatedAt: time.Now(),
			},
		},
		nextID: 4,
	}
}

func (m *mockOrderRepository) CreateFromCart(ctx context.Context, userID int64, items []domcart.Item, payment domorder.PaymentMethod) (*domorder.Order, error) {
	return nil, nil
}

func (m *mockOrderRepository) List(ctx context.Context) ([]*domorder.Order, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []*domorder.Order
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
	if !status.IsValid() {
		return nil, domorder.ErrInvalidStatus
	}
	order.Status = status
	cloned := *order
	return &cloned, nil
}

// --- Helper Functions ---

func setupOrderAPIWithRole(roleCode domuser.RoleCode, userID int64) (*API, string) {
	orderRepo := newMockOrderRepository()
	orderSvc := orderuc.NewService(orderRepo)
	tokenSvc := security.NewJWTService("test-secret", time.Hour)

	api := NewAPI(Dependencies{
		OrderService: orderSvc,
		TokenService: tokenSvc,
	})

	token, _ := tokenSvc.GenerateToken(&domuser.User{
		ID:       userID,
		Name:     string(roleCode) + " User",
		Email:    string(roleCode) + "@example.com",
		RoleCode: roleCode,
	})

	return api, token
}

func newAuthenticatedOrderRequest(method, path, token string, body any) *http.Request {
	var req *http.Request
	if body != nil {
		payload, _ := json.Marshal(body)
		req = httptest.NewRequest(method, path, bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// --- Test Cases ---

func TestAdminListOrders_AsSuperAdmin_Returns200(t *testing.T) {
	api, token := setupOrderAPIWithRole(domuser.RoleCodeSuperAdmin, 1)
	router := api.Router()

	req := newAuthenticatedOrderRequest(http.MethodGet, "/api/v1/admin/orders", token, nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

	data, ok := response["data"].([]any)
	require.True(t, ok, "response should have 'data' field with array")
	require.GreaterOrEqual(t, len(data), 1, "should return at least one order")

	// Verify first order structure
	order := data[0].(map[string]any)
	require.Contains(t, order, "id")
	require.Contains(t, order, "user_id")
	require.Contains(t, order, "status")
	require.Contains(t, order, "payment_method")
	require.Contains(t, order, "total_amount")
}

func TestAdminListOrders_AsAdmin_Returns200(t *testing.T) {
	api, token := setupOrderAPIWithRole(domuser.RoleCodeAdmin, 2)
	router := api.Router()

	req := newAuthenticatedOrderRequest(http.MethodGet, "/api/v1/admin/orders", token, nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

	data, ok := response["data"].([]any)
	require.True(t, ok)
	require.GreaterOrEqual(t, len(data), 1)
}

func TestAdminGetOrder_Found_Returns200(t *testing.T) {
	tests := []struct {
		name   string
		role   domuser.RoleCode
		userID int64
	}{
		{
			name:   "As SUPER_ADMIN",
			role:   domuser.RoleCodeSuperAdmin,
			userID: 1,
		},
		{
			name:   "As ADMIN",
			role:   domuser.RoleCodeAdmin,
			userID: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api, token := setupOrderAPIWithRole(tt.role, tt.userID)
			router := api.Router()

			req := newAuthenticatedOrderRequest(http.MethodGet, "/api/v1/admin/orders/1", token, nil)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

			var response map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

			require.Equal(t, float64(1), response["id"], "order id should match")
			require.Equal(t, float64(100), response["user_id"], "user id should match")
			require.Equal(t, "PENDING", response["status"], "status should match")
			require.Equal(t, "COD", response["payment_method"], "payment method should match")
			require.Equal(t, 50.0, response["total_amount"], "total amount should match")

			items, ok := response["items"].([]any)
			require.True(t, ok, "items should be an array")
			require.Len(t, items, 2, "should have 2 order items")
		})
	}
}

func TestAdminGetOrder_NotFound_Returns404(t *testing.T) {
	api, token := setupOrderAPIWithRole(domuser.RoleCodeAdmin, 1)
	router := api.Router()

	req := newAuthenticatedOrderRequest(http.MethodGet, "/api/v1/admin/orders/999", token, nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Contains(t, response["error"], domorder.ErrOrderNotFound.Error())
}

func TestAdminUpdateOrderStatus_Valid_Returns200(t *testing.T) {
	tests := []struct {
		name          string
		role          domuser.RoleCode
		orderID       int64
		initialStatus domorder.Status
		newStatus     string
	}{
		{
			name:          "PENDING to PAID as SUPER_ADMIN",
			role:          domuser.RoleCodeSuperAdmin,
			orderID:       1,
			initialStatus: domorder.StatusPending,
			newStatus:     "PAID",
		},
		{
			name:          "PAID to SHIPPED as ADMIN",
			role:          domuser.RoleCodeAdmin,
			orderID:       2,
			initialStatus: domorder.StatusPaid,
			newStatus:     "SHIPPED",
		},
		{
			name:          "PENDING to CANCELED as SUPER_ADMIN",
			role:          domuser.RoleCodeSuperAdmin,
			orderID:       1,
			initialStatus: domorder.StatusPending,
			newStatus:     "CANCELED",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api, token := setupOrderAPIWithRole(tt.role, 1)
			router := api.Router()

			body := map[string]any{
				"status": tt.newStatus,
			}

			req := newAuthenticatedOrderRequest(http.MethodPatch, "/api/v1/admin/orders/1", token, body)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

			var response map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

			require.Equal(t, tt.newStatus, response["status"], "status should be updated")
			require.Equal(t, float64(1), response["id"], "order id should remain the same")
		})
	}
}

func TestAdminUpdateOrderStatus_InvalidStatus_Returns422Or400(t *testing.T) {
	tests := []struct {
		name          string
		status        string
		expectedError string
	}{
		{
			name:          "Empty status",
			status:        "",
			expectedError: "Field validation for 'Status' failed on the 'required' tag",
		},
		{
			name:          "Invalid status value",
			status:        "INVALID_STATUS",
			expectedError: domorder.ErrInvalidStatus.Error(),
		},
		{
			name:          "Lowercase pending",
			status:        "pending",
			expectedError: domorder.ErrInvalidStatus.Error(),
		},
		{
			name:          "Processing (not in allowed set)",
			status:        "PROCESSING",
			expectedError: domorder.ErrInvalidStatus.Error(),
		},
		{
			name:          "Delivered (not in allowed set)",
			status:        "DELIVERED",
			expectedError: domorder.ErrInvalidStatus.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api, token := setupOrderAPIWithRole(domuser.RoleCodeAdmin, 1)
			router := api.Router()

			body := map[string]any{
				"status": tt.status,
			}

			req := newAuthenticatedOrderRequest(http.MethodPatch, "/api/v1/admin/orders/1", token, body)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			// Empty status returns 400 (validation error), invalid status returns 422 (domain error)
			expectedCode := http.StatusBadRequest
			if tt.status != "" {
				expectedCode = http.StatusUnprocessableEntity
			}
			require.Equal(t, expectedCode, rec.Code, rec.Body.String())

			var response map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
			require.Contains(t, response["error"], tt.expectedError)
		})
	}
}

func TestAdminUpdateOrderStatus_OrderNotFound_Returns404(t *testing.T) {
	api, token := setupOrderAPIWithRole(domuser.RoleCodeAdmin, 1)
	router := api.Router()

	body := map[string]any{
		"status": "PAID",
	}

	req := newAuthenticatedOrderRequest(http.MethodPatch, "/api/v1/admin/orders/999", token, body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Contains(t, response["error"], domorder.ErrOrderNotFound.Error())
}

func TestCustomerCannotAccessAdminOrders_Returns401Or403(t *testing.T) {
	tests := []struct {
		name         string
		role         domuser.RoleCode
		method       string
		path         string
		body         any
		expectedCode int
	}{
		{
			name:         "CUSTOMER cannot list orders",
			role:         domuser.RoleCodeCustomer,
			method:       http.MethodGet,
			path:         "/api/v1/admin/orders",
			body:         nil,
			expectedCode: http.StatusForbidden,
		},
		{
			name:         "CUSTOMER cannot get order",
			role:         domuser.RoleCodeCustomer,
			method:       http.MethodGet,
			path:         "/api/v1/admin/orders/1",
			body:         nil,
			expectedCode: http.StatusForbidden,
		},
		{
			name: "CUSTOMER cannot update order status",
			role: domuser.RoleCodeCustomer,
			method: http.MethodPatch,
			path:   "/api/v1/admin/orders/1",
			body:   map[string]any{"status": "PAID"},
			expectedCode: http.StatusForbidden,
		},
		{
			name:         "Guest cannot list orders",
			role:         domuser.RoleCode("GUEST"),
			method:       http.MethodGet,
			path:         "/api/v1/admin/orders",
			body:         nil,
			expectedCode: http.StatusForbidden,
		},
		{
			name:         "Guest cannot get order",
			role:         domuser.RoleCode("GUEST"),
			method:       http.MethodGet,
			path:         "/api/v1/admin/orders/1",
			body:         nil,
			expectedCode: http.StatusForbidden,
		},
		{
			name: "Guest cannot update order status",
			role: domuser.RoleCode("GUEST"),
			method: http.MethodPatch,
			path:   "/api/v1/admin/orders/1",
			body:   map[string]any{"status": "PAID"},
			expectedCode: http.StatusForbidden,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api, token := setupOrderAPIWithRole(tt.role, 100)
			router := api.Router()

			req := newAuthenticatedOrderRequest(tt.method, tt.path, token, tt.body)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, tt.expectedCode, rec.Code, rec.Body.String())
			var response map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
			require.Contains(t, response["error"], errForbidden.Error())
		})
	}
}

func TestAdminOrders_Unauthenticated_Returns401(t *testing.T) {
	api, _ := setupOrderAPIWithRole(domuser.RoleCodeAdmin, 1)
	router := api.Router()

	tests := []struct {
		name   string
		method string
		path   string
		body   any
	}{
		{
			name:   "GET /api/v1/admin/orders without auth",
			method: http.MethodGet,
			path:   "/api/v1/admin/orders",
			body:   nil,
		},
		{
			name:   "GET /api/v1/admin/orders/1 without auth",
			method: http.MethodGet,
			path:   "/api/v1/admin/orders/1",
			body:   nil,
		},
		{
			name:   "PATCH /api/v1/admin/orders/1 without auth",
			method: http.MethodPatch,
			path:   "/api/v1/admin/orders/1",
			body:   map[string]any{"status": "PAID"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var req *http.Request
			if tt.body != nil {
				payload, _ := json.Marshal(tt.body)
				req = httptest.NewRequest(tt.method, tt.path, bytes.NewReader(payload))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.path, nil)
			}
			// No Authorization header
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
			var response map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
			require.Contains(t, response["error"], errUnauthenticated.Error())
		})
	}
}

func TestAdminUpdateOrderStatus_AllValidStatuses(t *testing.T) {
	validStatuses := []string{
		"PENDING",
		"PAID",
		"SHIPPED",
		"CANCELED",
	}

	for _, status := range validStatuses {
		t.Run(status, func(t *testing.T) {
			api, token := setupOrderAPIWithRole(domuser.RoleCodeSuperAdmin, 1)
			router := api.Router()

			body := map[string]any{
				"status": status,
			}

			req := newAuthenticatedOrderRequest(http.MethodPatch, "/api/v1/admin/orders/1", token, body)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

			var response map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
			require.Equal(t, status, response["status"], "status should be updated to "+status)
		})
	}
}

func TestAdminGetOrder_WithItems_ReturnsCompleteOrder(t *testing.T) {
	api, token := setupOrderAPIWithRole(domuser.RoleCodeAdmin, 1)
	router := api.Router()

	req := newAuthenticatedOrderRequest(http.MethodGet, "/api/v1/admin/orders/1", token, nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

	// Verify order fields
	require.Equal(t, float64(1), response["id"])
	require.Equal(t, float64(100), response["user_id"])
	require.Equal(t, "PENDING", response["status"])
	require.Equal(t, "COD", response["payment_method"])
	require.Equal(t, 50.0, response["total_amount"])

	// Verify order items
	items, ok := response["items"].([]any)
	require.True(t, ok, "items should be an array")
	require.Len(t, items, 2, "should have 2 items")

	item1 := items[0].(map[string]any)
	require.Equal(t, float64(1), item1["product_id"])
	require.Equal(t, "Product 1", item1["name"])
	require.Equal(t, 10.0, item1["price"])
	require.Equal(t, float64(2), item1["quantity"])

	item2 := items[1].(map[string]any)
	require.Equal(t, float64(2), item2["product_id"])
	require.Equal(t, "Product 2", item2["name"])
	require.Equal(t, 30.0, item2["price"])
	require.Equal(t, float64(1), item2["quantity"])
}

func TestAdminUpdateOrderStatus_MissingStatusField_Returns400(t *testing.T) {
	api, token := setupOrderAPIWithRole(domuser.RoleCodeAdmin, 1)
	router := api.Router()

	body := map[string]any{
		// status field is missing
	}

	req := newAuthenticatedOrderRequest(http.MethodPatch, "/api/v1/admin/orders/1", token, body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Contains(t, response["error"], "Field validation for 'Status' failed on the 'required' tag")
}

func TestAdminUpdateOrderStatus_InvalidJSON_Returns400(t *testing.T) {
	api, token := setupOrderAPIWithRole(domuser.RoleCodeAdmin, 1)
	router := api.Router()

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/orders/1", bytes.NewReader([]byte(`{"status": 123}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Contains(t, response["error"], "cannot unmarshal")
}

func TestAdminListOrders_EmptyList_Returns200WithEmptyArray(t *testing.T) {
	_, token := setupOrderAPIWithRole(domuser.RoleCodeSuperAdmin, 1)

	// Create a new API with empty repository
	emptyRepo := &mockOrderRepository{
		orders: make(map[int64]*domorder.Order),
		nextID: 1,
	}
	orderSvc := orderuc.NewService(emptyRepo)
	tokenSvc := security.NewJWTService("test-secret", time.Hour)

	emptyAPI := NewAPI(Dependencies{
		OrderService: orderSvc,
		TokenService: tokenSvc,
	})
	emptyRouter := emptyAPI.Router()

	req := newAuthenticatedOrderRequest(http.MethodGet, "/api/v1/admin/orders", token, nil)
	rec := httptest.NewRecorder()

	emptyRouter.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

	data, ok := response["data"].([]any)
	require.True(t, ok, "response should have 'data' field")
	require.Len(t, data, 0, "should return empty array when no orders exist")
}

