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
	domproduct "example.com/my-golang-sample/app/internal/domain/product"
	domuser "example.com/my-golang-sample/app/internal/domain/user"
	"example.com/my-golang-sample/app/internal/infra/security"
	cartuc "example.com/my-golang-sample/app/internal/usecase/cart"
)

type fakeCartRepo struct {
	items map[int64]map[int64]int64 // userID -> productID -> quantity
}

func newFakeCartRepo() *fakeCartRepo {
	return &fakeCartRepo{
		items: make(map[int64]map[int64]int64),
	}
}

func (f *fakeCartRepo) AddOrUpdateItem(ctx context.Context, userID, productID, quantity int64) error {
	if f.items[userID] == nil {
		f.items[userID] = make(map[int64]int64)
	}
	f.items[userID][productID] += quantity
	return nil
}

func (f *fakeCartRepo) ListItems(ctx context.Context, userID int64) ([]domcart.Item, error) {
	userItems := f.items[userID]
	if userItems == nil {
		return []domcart.Item{}, nil
	}
	var items []domcart.Item
	for productID, quantity := range userItems {
		items = append(items, domcart.Item{
			ProductID: productID,
			Quantity:  quantity,
		})
	}
	return items, nil
}

func (f *fakeCartRepo) Clear(ctx context.Context, userID int64) error {
	delete(f.items, userID)
	return nil
}

type fakeProductRepoForCart struct {
	products map[int64]*domproduct.Product
}

func newFakeProductRepoForCart() *fakeProductRepoForCart {
	return &fakeProductRepoForCart{
		products: map[int64]*domproduct.Product{
			1: {ID: 1, Name: "Product 1", Price: 10.0, Stock: 100, IsActive: true},
			2: {ID: 2, Name: "Product 2", Price: 20.0, Stock: 5, IsActive: true},
			3: {ID: 3, Name: "Inactive Product", Price: 30.0, Stock: 50, IsActive: false},
		},
	}
}

func (f *fakeProductRepoForCart) GetByID(ctx context.Context, id int64) (*domproduct.Product, error) {
	if p, ok := f.products[id]; ok {
		return p, nil
	}
	return nil, domproduct.ErrProductNotFound
}

func (f *fakeProductRepoForCart) GetByIDs(ctx context.Context, ids []int64) ([]*domproduct.Product, error) {
	var result []*domproduct.Product
	for _, id := range ids {
		if p, ok := f.products[id]; ok {
			result = append(result, p)
		}
	}
	return result, nil
}

type fakeOrderRepoForCart struct {
	createdOrders []*domorder.Order
}

func (f *fakeOrderRepoForCart) CreateFromCart(ctx context.Context, userID int64, items []domcart.Item, payment domorder.PaymentMethod) (*domorder.Order, error) {
	if len(items) == 0 {
		return nil, domorder.ErrEmptyOrderItems
	}

	var totalAmount float64
	orderItems := make([]domorder.OrderItem, 0, len(items))
	productRepo := newFakeProductRepoForCart()

	for _, item := range items {
		product, err := productRepo.GetByID(ctx, item.ProductID)
		if err != nil {
			return nil, domorder.ErrCheckoutValidation
		}
		if product.Stock < item.Quantity {
			return nil, domorder.ErrCheckoutValidation
		}
		totalAmount += product.Price * float64(item.Quantity)
		orderItems = append(orderItems, domorder.OrderItem{
			ID:        int64(len(orderItems) + 1),
			OrderID:   1,
			ProductID: item.ProductID,
			Name:      product.Name,
			Price:     product.Price,
			Quantity:  item.Quantity,
		})
	}

	order := &domorder.Order{
		ID:            1,
		UserID:        userID,
		Status:        domorder.StatusPending,
		PaymentMethod: payment,
		TotalAmount:   totalAmount,
		Items:         orderItems,
	}

	if f.createdOrders == nil {
		f.createdOrders = make([]*domorder.Order, 0)
	}
	f.createdOrders = append(f.createdOrders, order)

	return order, nil
}

func setupCartAPI() (*API, string, *fakeCartRepo, *fakeOrderRepoForCart) {
	cartRepo := newFakeCartRepo()
	productRepo := newFakeProductRepoForCart()
	orderRepo := &fakeOrderRepoForCart{}

	cartSvc := cartuc.NewService(cartRepo, productRepo, orderRepo)
	tokenSvc := security.NewJWTService("test-secret", time.Hour)

	api := NewAPI(Dependencies{
		CartService:  cartSvc,
		TokenService: tokenSvc,
	})

	token, _ := tokenSvc.GenerateToken(&domuser.User{
		ID:       100,
		Name:     "Test User",
		Email:    "test@example.com",
		RoleCode: domuser.RoleCodeCustomer,
	})

	return api, token, cartRepo, orderRepo
}

func TestCart_AddItemSuccess(t *testing.T) {
	api, token, _, _ := setupCartAPI()
	router := api.Router()

	body := map[string]any{
		"product_id": 1,
		"quantity":   5,
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart/items", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestCart_GetCartReturnsItems(t *testing.T) {
	api, token, _, _ := setupCartAPI()
	router := api.Router()

	// Add item first
	body := map[string]any{
		"product_id": 1,
		"quantity":   3,
	}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart/items", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Get cart
	req = httptest.NewRequest(http.MethodGet, "/api/v1/me/cart", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, float64(100), response["user_id"], "user_id should match token")

	items, ok := response["items"].([]any)
	require.True(t, ok, "items should be an array")
	require.Len(t, items, 1, "should have 1 item")

	item := items[0].(map[string]any)
	require.Equal(t, float64(1), item["product_id"])
	require.Equal(t, float64(3), item["quantity"])
	require.Equal(t, "Product 1", item["name"])
	require.Equal(t, 10.0, item["price"])
}

func TestCart_AddItemToInactiveProductReturns404(t *testing.T) {
	api, token, _, _ := setupCartAPI()
	router := api.Router()

	body := map[string]any{
		"product_id": 3, // inactive product
		"quantity":   1,
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart/items", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
}

func TestCart_AddItemExceedsStockReturns422(t *testing.T) {
	api, token, _, _ := setupCartAPI()
	router := api.Router()

	body := map[string]any{
		"product_id": 2,  // stock = 5
		"quantity":   10, // exceeds stock
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart/items", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
}

func TestCart_AddItemWithoutAuthReturns401(t *testing.T) {
	api, _, _, _ := setupCartAPI()
	router := api.Router()

	body := map[string]any{
		"product_id": 1,
		"quantity":   1,
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart/items", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}

func TestCart_GetCartWithoutAuthReturns401(t *testing.T) {
	api, _, _, _ := setupCartAPI()
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/cart", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}

func TestCart_GetEmptyCartReturnsEmptyArray(t *testing.T) {
	api, token, _, _ := setupCartAPI()
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/cart", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, float64(100), response["user_id"])

	items, ok := response["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 0, "empty cart should return empty array")
}

func TestCart_AddMultipleItemsAndGetCart(t *testing.T) {
	api, token, _, _ := setupCartAPI()
	router := api.Router()

	// Add first item
	body1 := map[string]any{
		"product_id": 1,
		"quantity":   2,
	}
	payload1, _ := json.Marshal(body1)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart/items", bytes.NewReader(payload1))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Add second item
	body2 := map[string]any{
		"product_id": 2,
		"quantity":   3,
	}
	payload2, _ := json.Marshal(body2)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/me/cart/items", bytes.NewReader(payload2))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Get cart
	req = httptest.NewRequest(http.MethodGet, "/api/v1/me/cart", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

	items, ok := response["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 2, "should have 2 items")
}

func TestCheckout_WithCODReturnsOrderAndClearsCart(t *testing.T) {
	api, token, cartRepo, orderRepo := setupCartAPI()
	router := api.Router()

	// Add items to cart
	body1 := map[string]any{
		"product_id": 1,
		"quantity":   2,
	}
	payload1, _ := json.Marshal(body1)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart/items", bytes.NewReader(payload1))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Checkout with COD
	checkoutBody := map[string]any{
		"payment_method": "COD",
	}
	checkoutPayload, _ := json.Marshal(checkoutBody)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/me/checkout", bytes.NewReader(checkoutPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var orderResponse map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &orderResponse))
	require.Equal(t, float64(100), orderResponse["user_id"])
	require.Equal(t, "COD", orderResponse["payment_method"])
	require.Equal(t, "PENDING", orderResponse["status"])

	// Verify cart is cleared
	items, err := cartRepo.ListItems(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, items, 0, "cart should be cleared after checkout")

	// Verify order was created
	require.Len(t, orderRepo.createdOrders, 1, "order should be created")
}

func TestCheckout_WithTAMARAReturnsOrderAndClearsCart(t *testing.T) {
	api, token, cartRepo, orderRepo := setupCartAPI()
	router := api.Router()

	// Add items to cart
	body1 := map[string]any{
		"product_id": 1,
		"quantity":   3,
	}
	payload1, _ := json.Marshal(body1)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart/items", bytes.NewReader(payload1))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Checkout with TAMARA
	checkoutBody := map[string]any{
		"payment_method": "TAMARA",
	}
	checkoutPayload, _ := json.Marshal(checkoutBody)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/me/checkout", bytes.NewReader(checkoutPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var orderResponse map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &orderResponse))
	require.Equal(t, "TAMARA", orderResponse["payment_method"])
	require.Equal(t, "PENDING", orderResponse["status"])

	// Verify cart is cleared
	items, err := cartRepo.ListItems(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, items, 0, "cart should be cleared after checkout")

	// Verify order was created
	require.Len(t, orderRepo.createdOrders, 1, "order should be created")
}

func TestCheckout_EmptyCartReturns422(t *testing.T) {
	api, token, _, orderRepo := setupCartAPI()
	_ = orderRepo // suppress unused warning
	router := api.Router()

	checkoutBody := map[string]any{
		"payment_method": "COD",
	}
	checkoutPayload, _ := json.Marshal(checkoutBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/checkout", bytes.NewReader(checkoutPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
}

func TestCheckout_InvalidPaymentMethodReturns400(t *testing.T) {
	api, token, _, _ := setupCartAPI()
	router := api.Router()

	// Add item first
	body1 := map[string]any{
		"product_id": 1,
		"quantity":   1,
	}
	payload1, _ := json.Marshal(body1)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart/items", bytes.NewReader(payload1))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Try checkout with invalid payment method
	checkoutBody := map[string]any{
		"payment_method": "INVALID",
	}
	checkoutPayload, _ := json.Marshal(checkoutBody)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/me/checkout", bytes.NewReader(checkoutPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestCheckout_WithoutAuthReturns401(t *testing.T) {
	api, _, _, _ := setupCartAPI()
	router := api.Router()

	checkoutBody := map[string]any{
		"payment_method": "COD",
	}
	checkoutPayload, _ := json.Marshal(checkoutBody)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/checkout", bytes.NewReader(checkoutPayload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}

func TestCheckout_CreatesOrderWithCorrectItemsAndTotal(t *testing.T) {
	api, token, cartRepo, orderRepo := setupCartAPI()
	router := api.Router()

	// Add multiple items
	body1 := map[string]any{
		"product_id": 1, // Price: 10.0
		"quantity":   2,
	}
	payload1, _ := json.Marshal(body1)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart/items", bytes.NewReader(payload1))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	body2 := map[string]any{
		"product_id": 2, // Price: 20.0
		"quantity":   1,
	}
	payload2, _ := json.Marshal(body2)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/me/cart/items", bytes.NewReader(payload2))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code)

	// Checkout
	checkoutBody := map[string]any{
		"payment_method": "COD",
	}
	checkoutPayload, _ := json.Marshal(checkoutBody)
	req = httptest.NewRequest(http.MethodPost, "/api/v1/me/checkout", bytes.NewReader(checkoutPayload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var orderResponse map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &orderResponse))

	// Verify total amount: (10.0 * 2) + (20.0 * 1) = 40.0
	require.Equal(t, 40.0, orderResponse["total_amount"])

	// Verify order items
	items, ok := orderResponse["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 2, "should have 2 order items")

	// Verify cart is cleared
	cartItems, err := cartRepo.ListItems(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, cartItems, 0, "cart should be cleared")

	// Verify order was created
	require.Len(t, orderRepo.createdOrders, 1, "order should be created")
}
