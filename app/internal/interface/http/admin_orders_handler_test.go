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

type fakeOrderRepo struct {
	orders map[int64]*domorder.Order
	nextID int64
}

func newFakeOrderRepo() *fakeOrderRepo {
	return &fakeOrderRepo{
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
			},
		},
		nextID: 3,
	}
}

func (f *fakeOrderRepo) CreateFromCart(ctx context.Context, userID int64, items []domcart.Item, payment domorder.PaymentMethod) (*domorder.Order, error) {
	return nil, nil
}

func (f *fakeOrderRepo) List(ctx context.Context) ([]*domorder.Order, error) {
	var result []*domorder.Order
	for _, order := range f.orders {
		cloned := *order
		result = append(result, &cloned)
	}
	return result, nil
}

func (f *fakeOrderRepo) GetByID(ctx context.Context, id int64) (*domorder.Order, error) {
	if order, ok := f.orders[id]; ok {
		cloned := *order
		return &cloned, nil
	}
	return nil, domorder.ErrOrderNotFound
}

func (f *fakeOrderRepo) UpdateStatus(ctx context.Context, id int64, status domorder.Status) (*domorder.Order, error) {
	order, ok := f.orders[id]
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

func setupOrderAPI(roleCode domuser.RoleCode) (*API, string) {
	orderRepo := newFakeOrderRepo()
	orderSvc := orderuc.NewService(orderRepo)
	tokenSvc := security.NewJWTService("test-secret", time.Hour)

	api := NewAPI(Dependencies{
		OrderService: orderSvc,
		TokenService: tokenSvc,
	})

	token, _ := tokenSvc.GenerateToken(&domuser.User{
		ID:       1,
		Name:     "Test User",
		Email:    "test@example.com",
		RoleCode: roleCode,
	})

	return api, token
}

func TestAdminOrders_ListOrdersAsAdminReturns200(t *testing.T) {
	api, token := setupOrderAPI(domuser.RoleCodeAdmin)
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	data, ok := response["data"].([]any)
	require.True(t, ok)
	require.GreaterOrEqual(t, len(data), 2, "should have at least 2 orders")
}

func TestAdminOrders_ListOrdersAsSuperAdminReturns200(t *testing.T) {
	api, token := setupOrderAPI(domuser.RoleCodeSuperAdmin)
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestAdminOrders_ListOrdersAsCustomerReturns403(t *testing.T) {
	api, token := setupOrderAPI(domuser.RoleCodeCustomer)
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

func TestAdminOrders_ListOrdersWithoutAuthReturns401(t *testing.T) {
	api, _ := setupOrderAPI(domuser.RoleCodeAdmin)
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}

func TestAdminOrders_GetOrderByIdAsAdminReturns200(t *testing.T) {
	api, token := setupOrderAPI(domuser.RoleCodeAdmin)
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var order map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &order))
	require.Equal(t, float64(1), order["id"])
	require.Equal(t, "PENDING", order["status"])
	require.Equal(t, "COD", order["payment_method"])
	require.Equal(t, 50.0, order["total_amount"])

	items, ok := order["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 2, "should have 2 items")
}

func TestAdminOrders_GetOrderByIdAsSuperAdminReturns200(t *testing.T) {
	api, token := setupOrderAPI(domuser.RoleCodeSuperAdmin)
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

func TestAdminOrders_GetOrderByIdAsCustomerReturns403(t *testing.T) {
	api, token := setupOrderAPI(domuser.RoleCodeCustomer)
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders/1", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

func TestAdminOrders_GetOrderByIdNotFoundReturns404(t *testing.T) {
	api, token := setupOrderAPI(domuser.RoleCodeAdmin)
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/orders/999", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestAdminOrders_UpdateOrderStatusAsAdminReturns200(t *testing.T) {
	api, token := setupOrderAPI(domuser.RoleCodeAdmin)
	router := api.Router()

	body := map[string]any{
		"status": "PAID",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/orders/1", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var order map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &order))
	require.Equal(t, "PAID", order["status"])
}

func TestAdminOrders_UpdateOrderStatusAsSuperAdminReturns200(t *testing.T) {
	api, token := setupOrderAPI(domuser.RoleCodeSuperAdmin)
	router := api.Router()

	body := map[string]any{
		"status": "SHIPPED",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/orders/1", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var order map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &order))
	require.Equal(t, "SHIPPED", order["status"])
}

func TestAdminOrders_UpdateOrderStatusAsCustomerReturns403(t *testing.T) {
	api, token := setupOrderAPI(domuser.RoleCodeCustomer)
	router := api.Router()

	body := map[string]any{
		"status": "PAID",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/orders/1", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
}

func TestAdminOrders_UpdateOrderStatusWithoutAuthReturns401(t *testing.T) {
	api, _ := setupOrderAPI(domuser.RoleCodeAdmin)
	router := api.Router()

	body := map[string]any{
		"status": "PAID",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/orders/1", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}

func TestAdminOrders_UpdateOrderStatusWithInvalidStatusReturns422(t *testing.T) {
	api, token := setupOrderAPI(domuser.RoleCodeAdmin)
	router := api.Router()

	body := map[string]any{
		"status": "INVALID_STATUS",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/orders/1", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
}

func TestAdminOrders_UpdateOrderStatusNotFoundReturns404(t *testing.T) {
	api, token := setupOrderAPI(domuser.RoleCodeAdmin)
	router := api.Router()

	body := map[string]any{
		"status": "PAID",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/orders/999", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestAdminOrders_UpdateOrderStatusAllValidStatuses(t *testing.T) {
	api, token := setupOrderAPI(domuser.RoleCodeAdmin)
	router := api.Router()

	validStatuses := []string{"PENDING", "PAID", "SHIPPED", "CANCELED"}

	for _, status := range validStatuses {
		body := map[string]any{
			"status": status,
		}
		payload, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/orders/1", bytes.NewReader(payload))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		router.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code, "status %s should be valid", status)

		var order map[string]any
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &order))
		require.Equal(t, status, order["status"])
	}
}
