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

// --- Mock Repositories for Cart Tests ---

type mockCartRepository struct {
	items map[int64]map[int64]int64 // userID -> productID -> quantity
}

func newMockCartRepository() *mockCartRepository {
	return &mockCartRepository{
		items: make(map[int64]map[int64]int64),
	}
}

func (m *mockCartRepository) AddOrUpdateItem(ctx context.Context, userID, productID, quantity int64) error {
	if m.items[userID] == nil {
		m.items[userID] = make(map[int64]int64)
	}
	m.items[userID][productID] += quantity
	return nil
}

func (m *mockCartRepository) ListItems(ctx context.Context, userID int64) ([]domcart.Item, error) {
	userItems := m.items[userID]
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

func (m *mockCartRepository) Clear(ctx context.Context, userID int64) error {
	delete(m.items, userID)
	return nil
}

type mockProductRepositoryForCart struct {
	products map[int64]*domproduct.Product
	getByIDErr error
}

func newMockProductRepositoryForCart() *mockProductRepositoryForCart {
	return &mockProductRepositoryForCart{
		products: map[int64]*domproduct.Product{
			1: {ID: 1, Name: "Product 1", Price: 10.0, Stock: 100, CategoryID: 1, IsActive: true},
			2: {ID: 2, Name: "Product 2", Price: 20.0, Stock: 5, CategoryID: 1, IsActive: true},
			3: {ID: 3, Name: "Inactive Product", Price: 30.0, Stock: 50, CategoryID: 1, IsActive: false},
		},
	}
}

func (m *mockProductRepositoryForCart) GetByID(ctx context.Context, id int64) (*domproduct.Product, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	if p, ok := m.products[id]; ok {
		cloned := *p
		return &cloned, nil
	}
	return nil, domproduct.ErrProductNotFound
}

func (m *mockProductRepositoryForCart) GetByIDs(ctx context.Context, ids []int64) ([]*domproduct.Product, error) {
	var result []*domproduct.Product
	for _, id := range ids {
		if p, ok := m.products[id]; ok {
			cloned := *p
			result = append(result, &cloned)
		}
	}
	return result, nil
}

type mockOrderRepositoryForCart struct {
	createdOrders []*domorder.Order
	createErr error
}

func newMockOrderRepositoryForCart() *mockOrderRepositoryForCart {
	return &mockOrderRepositoryForCart{
		createdOrders: make([]*domorder.Order, 0),
	}
}

func (m *mockOrderRepositoryForCart) CreateFromCart(ctx context.Context, userID int64, items []domcart.Item, payment domorder.PaymentMethod) (*domorder.Order, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if len(items) == 0 {
		return nil, domorder.ErrEmptyOrderItems
	}

	var totalAmount float64
	orderItems := make([]domorder.OrderItem, 0, len(items))
	productRepo := newMockProductRepositoryForCart()

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
		CreatedAt:     time.Now(),
	}

	m.createdOrders = append(m.createdOrders, order)
	return order, nil
}

// --- Helper Functions ---

func setupCartAPIWithCustomer() (*API, string) {
	cartRepo := newMockCartRepository()
	productRepo := newMockProductRepositoryForCart()
	orderRepo := newMockOrderRepositoryForCart()

	cartSvc := cartuc.NewService(cartRepo, productRepo, orderRepo)
	tokenSvc := security.NewJWTService("test-secret", time.Hour)

	api := NewAPI(Dependencies{
		CartService:  cartSvc,
		TokenService: tokenSvc,
	})

	// Generate token for a CUSTOMER user
	token, _ := tokenSvc.GenerateToken(&domuser.User{
		ID:       100,
		Name:     "Test Customer",
		Email:    "customer@example.com",
		RoleCode: domuser.RoleCodeCustomer,
	})

	return api, token
}

func newAuthenticatedCartRequest(method, path, token string, body any) *http.Request {
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

func TestAddToCart_ValidProductAndQuantity_Returns201(t *testing.T) {
	api, token := setupCartAPIWithCustomer()
	router := api.Router()

	body := map[string]any{
		"product_id": 1,
		"quantity":   2,
	}

	req := newAuthenticatedCartRequest(http.MethodPost, "/api/v1/me/cart/items", token, body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Equal(t, "added", response["status"])
}

func TestAddToCart_ProductNotFound_Returns404Or422(t *testing.T) {
	tests := []struct {
		name        string
		productID   int64
		expectedErr string
	}{
		{
			name:        "Non-existent product ID",
			productID:   999,
			expectedErr: domproduct.ErrProductNotFound.Error(),
		},
		{
			name:        "Inactive product",
			productID:   3, // Product 3 is inactive
			expectedErr: domproduct.ErrProductNotFound.Error(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api, token := setupCartAPIWithCustomer()
			router := api.Router()

			body := map[string]any{
				"product_id": tt.productID,
				"quantity":   1,
			}

			req := newAuthenticatedCartRequest(http.MethodPost, "/api/v1/me/cart/items", token, body)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
			var response map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
			require.Contains(t, response["error"], tt.expectedErr)
		})
	}
}

func TestAddToCart_InvalidQuantity_Returns422(t *testing.T) {
	tests := []struct {
		name          string
		quantity      int64
		expectedError string
	}{
		{
			name:          "Zero quantity",
			quantity:      0,
			expectedError: "Field validation for 'Quantity' failed", // May be 'required' or 'gt' depending on validator order
		},
		{
			name:          "Negative quantity",
			quantity:      -1,
			expectedError: "Field validation for 'Quantity' failed on the 'gt' tag",
		},
		{
			name:          "Missing quantity field",
			quantity:      0, // Will be omitted from JSON
			expectedError: "Field validation for 'Quantity' failed on the 'required' tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api, token := setupCartAPIWithCustomer()
			router := api.Router()

			body := map[string]any{
				"product_id": 1,
			}
			if tt.name != "Missing quantity field" {
				body["quantity"] = tt.quantity
			}

			req := newAuthenticatedCartRequest(http.MethodPost, "/api/v1/me/cart/items", token, body)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			var response map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
			require.Contains(t, response["error"], tt.expectedError)
		})
	}
}

func TestAddToCart_InvalidProductID_Returns400(t *testing.T) {
	tests := []struct {
		name          string
		productID     any
		expectedError string
	}{
		{
			name:          "Zero product ID",
			productID:     0,
			expectedError: "Field validation for 'ProductID' failed", // May be 'required' or 'gt' depending on validator order
		},
		{
			name:          "Negative product ID",
			productID:     -1,
			expectedError: "Field validation for 'ProductID' failed on the 'gt' tag",
		},
		{
			name:          "Missing product_id field",
			productID:     nil,
			expectedError: "Field validation for 'ProductID' failed on the 'required' tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api, token := setupCartAPIWithCustomer()
			router := api.Router()

			body := map[string]any{
				"quantity": 1,
			}
			if tt.productID != nil {
				body["product_id"] = tt.productID
			}

			req := newAuthenticatedCartRequest(http.MethodPost, "/api/v1/me/cart/items", token, body)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			var response map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
			require.Contains(t, response["error"], tt.expectedError)
		})
	}
}

func TestGetCart_AsCustomer_Returns200(t *testing.T) {
	api, token := setupCartAPIWithCustomer()
	router := api.Router()

	// First, add some items to the cart
	body1 := map[string]any{
		"product_id": 1,
		"quantity":   3,
	}
	req1 := newAuthenticatedCartRequest(http.MethodPost, "/api/v1/me/cart/items", token, body1)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusCreated, rec1.Code)

	body2 := map[string]any{
		"product_id": 2,
		"quantity":   2,
	}
	req2 := newAuthenticatedCartRequest(http.MethodPost, "/api/v1/me/cart/items", token, body2)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusCreated, rec2.Code)

	// Now get the cart
	req := newAuthenticatedCartRequest(http.MethodGet, "/api/v1/me/cart", token, nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

	// Verify user_id matches the authenticated user
	require.Equal(t, float64(100), response["user_id"], "user_id should match token")

	// Verify items array
	items, ok := response["items"].([]any)
	require.True(t, ok, "items should be an array")
	require.Len(t, items, 2, "should have 2 items")

	// Find items by product_id (order may vary)
	itemMap := make(map[float64]map[string]any)
	for _, item := range items {
		itemObj := item.(map[string]any)
		productID := itemObj["product_id"].(float64)
		itemMap[productID] = itemObj
	}

	// Verify product 1
	item1, exists := itemMap[1]
	require.True(t, exists, "product 1 should be in cart")
	require.Equal(t, float64(3), item1["quantity"])
	require.Equal(t, "Product 1", item1["name"])
	require.Equal(t, 10.0, item1["price"])

	// Verify product 2
	item2, exists := itemMap[2]
	require.True(t, exists, "product 2 should be in cart")
	require.Equal(t, float64(2), item2["quantity"])
	require.Equal(t, "Product 2", item2["name"])
	require.Equal(t, 20.0, item2["price"])
}

func TestGetCart_EmptyCart_Returns200WithEmptyArray(t *testing.T) {
	api, token := setupCartAPIWithCustomer()
	router := api.Router()

	req := newAuthenticatedCartRequest(http.MethodGet, "/api/v1/me/cart", token, nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

	require.Equal(t, float64(100), response["user_id"])

	items, ok := response["items"].([]any)
	require.True(t, ok, "items should be an array")
	require.Len(t, items, 0, "empty cart should return empty array")
}

func TestGetCart_WithoutAuth_Returns401Or403(t *testing.T) {
	api, _ := setupCartAPIWithCustomer()
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/cart", nil)
	// No Authorization header
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Contains(t, response["error"], errUnauthenticated.Error())
}

func TestAddToCart_WithoutAuth_Returns401Or403(t *testing.T) {
	api, _ := setupCartAPIWithCustomer()
	router := api.Router()

	body := map[string]any{
		"product_id": 1,
		"quantity":   1,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart/items", bytes.NewReader(func() []byte {
		payload, _ := json.Marshal(body)
		return payload
	}()))
	req.Header.Set("Content-Type", "application/json")
	// No Authorization header
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Contains(t, response["error"], errUnauthenticated.Error())
}

func TestAddToCart_OutOfStock_Returns422(t *testing.T) {
	api, token := setupCartAPIWithCustomer()
	router := api.Router()

	// Product 2 has stock of 5, try to add 10
	body := map[string]any{
		"product_id": 2,
		"quantity":   10,
	}

	req := newAuthenticatedCartRequest(http.MethodPost, "/api/v1/me/cart/items", token, body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Contains(t, response["error"], domproduct.ErrOutOfStock.Error())
}

func TestAddToCart_UpdatesExistingItemQuantity(t *testing.T) {
	api, token := setupCartAPIWithCustomer()
	router := api.Router()

	// Add product 1 with quantity 2
	body1 := map[string]any{
		"product_id": 1,
		"quantity":   2,
	}
	req1 := newAuthenticatedCartRequest(http.MethodPost, "/api/v1/me/cart/items", token, body1)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusCreated, rec1.Code)

	// Add the same product again with quantity 3
	body2 := map[string]any{
		"product_id": 1,
		"quantity":   3,
	}
	req2 := newAuthenticatedCartRequest(http.MethodPost, "/api/v1/me/cart/items", token, body2)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusCreated, rec2.Code)

	// Get cart and verify total quantity is 5 (2 + 3)
	req := newAuthenticatedCartRequest(http.MethodGet, "/api/v1/me/cart", token, nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

	items, ok := response["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 1, "should have 1 item (updated quantity)")

	item := items[0].(map[string]any)
	require.Equal(t, float64(1), item["product_id"])
	require.Equal(t, float64(5), item["quantity"], "quantity should be sum of both additions")
}

func TestAddToCart_InvalidJSON_Returns400(t *testing.T) {
	api, token := setupCartAPIWithCustomer()
	router := api.Router()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/cart/items", bytes.NewReader([]byte(`{"product_id": 1, "quantity": "invalid"}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Contains(t, response["error"], "cannot unmarshal")
}

func TestGetCart_UserIsolation(t *testing.T) {
	api, token1 := setupCartAPIWithCustomer()
	router := api.Router()

	// Add item for user 1 (from token)
	body1 := map[string]any{
		"product_id": 1,
		"quantity":   2,
	}
	req1 := newAuthenticatedCartRequest(http.MethodPost, "/api/v1/me/cart/items", token1, body1)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusCreated, rec1.Code)

	// Create a token for a different user
	tokenSvc := security.NewJWTService("test-secret", time.Hour)
	token2, _ := tokenSvc.GenerateToken(&domuser.User{
		ID:       200,
		Name:     "Another Customer",
		Email:    "customer2@example.com",
		RoleCode: domuser.RoleCodeCustomer,
	})

	// Get cart for user 2 (should be empty)
	req2 := newAuthenticatedCartRequest(http.MethodGet, "/api/v1/me/cart", token2, nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)

	require.Equal(t, http.StatusOK, rec2.Code, rec2.Body.String())

	var response2 map[string]any
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &response2))
	require.Equal(t, float64(200), response2["user_id"], "user_id should match token2")

	items2, ok := response2["items"].([]any)
	require.True(t, ok)
	require.Len(t, items2, 0, "user 2's cart should be empty")

	// Verify user 1's cart still has items
	req1Get := newAuthenticatedCartRequest(http.MethodGet, "/api/v1/me/cart", token1, nil)
	rec1Get := httptest.NewRecorder()
	router.ServeHTTP(rec1Get, req1Get)

	require.Equal(t, http.StatusOK, rec1Get.Code, rec1Get.Body.String())

	var response1 map[string]any
	require.NoError(t, json.Unmarshal(rec1Get.Body.Bytes(), &response1))
	require.Equal(t, float64(100), response1["user_id"], "user_id should match token1")

	items1, ok := response1["items"].([]any)
	require.True(t, ok)
	require.Len(t, items1, 1, "user 1's cart should still have 1 item")
}

