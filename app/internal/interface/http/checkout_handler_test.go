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

// --- Mock Repositories for Checkout Tests ---

type mockCheckoutCartRepository struct {
	items    map[int64]map[int64]int64 // userID -> productID -> quantity
	listErr  error
	clearErr error
}

func newMockCheckoutCartRepository() *mockCheckoutCartRepository {
	return &mockCheckoutCartRepository{
		items: make(map[int64]map[int64]int64),
	}
}

func (m *mockCheckoutCartRepository) AddOrUpdateItem(ctx context.Context, userID, productID, quantity int64) error {
	if m.items[userID] == nil {
		m.items[userID] = make(map[int64]int64)
	}
	m.items[userID][productID] += quantity
	return nil
}

func (m *mockCheckoutCartRepository) ListItems(ctx context.Context, userID int64) ([]domcart.Item, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
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

func (m *mockCheckoutCartRepository) Clear(ctx context.Context, userID int64) error {
	if m.clearErr != nil {
		return m.clearErr
	}
	delete(m.items, userID)
	return nil
}

type mockCheckoutProductRepository struct {
	products map[int64]*domproduct.Product
}

func newMockCheckoutProductRepository() *mockCheckoutProductRepository {
	return &mockCheckoutProductRepository{
		products: map[int64]*domproduct.Product{
			1: {ID: 1, Name: "Product 1", Price: 10.0, Stock: 100, CategoryID: 1, IsActive: true},
			2: {ID: 2, Name: "Product 2", Price: 20.0, Stock: 50, CategoryID: 1, IsActive: true},
			3: {ID: 3, Name: "Product 3", Price: 30.0, Stock: 25, CategoryID: 1, IsActive: true},
		},
	}
}

func (m *mockCheckoutProductRepository) GetByID(ctx context.Context, id int64) (*domproduct.Product, error) {
	if p, ok := m.products[id]; ok {
		cloned := *p
		return &cloned, nil
	}
	return nil, domproduct.ErrProductNotFound
}

func (m *mockCheckoutProductRepository) GetByIDs(ctx context.Context, ids []int64) ([]*domproduct.Product, error) {
	var result []*domproduct.Product
	for _, id := range ids {
		if p, ok := m.products[id]; ok {
			cloned := *p
			result = append(result, &cloned)
		}
	}
	return result, nil
}

type mockCheckoutOrderRepository struct {
	createdOrders []*domorder.Order
	createErr     error
}

func newMockCheckoutOrderRepository() *mockCheckoutOrderRepository {
	return &mockCheckoutOrderRepository{
		createdOrders: make([]*domorder.Order, 0),
	}
}

func (m *mockCheckoutOrderRepository) CreateFromCart(ctx context.Context, userID int64, items []domcart.Item, payment domorder.PaymentMethod) (*domorder.Order, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	if len(items) == 0 {
		return nil, domorder.ErrEmptyOrderItems
	}

	var totalAmount float64
	orderItems := make([]domorder.OrderItem, 0, len(items))
	productRepo := newMockCheckoutProductRepository()

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
		ID:            int64(len(m.createdOrders) + 1),
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

func setupCheckoutAPI() (*API, string, *mockCheckoutCartRepository, *mockCheckoutOrderRepository) {
	cartRepo := newMockCheckoutCartRepository()
	productRepo := newMockCheckoutProductRepository()
	orderRepo := newMockCheckoutOrderRepository()

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

	return api, token, cartRepo, orderRepo
}

func newAuthenticatedCheckoutRequest(method, path, token string, body any) *http.Request {
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

func TestCheckout_EmptyCart_ReturnsError(t *testing.T) {
	api, token, _, _ := setupCheckoutAPI()
	router := api.Router()

	// Cart is empty by default (no items added)
	body := map[string]any{
		"payment_method": "COD",
	}

	req := newAuthenticatedCheckoutRequest(http.MethodPost, "/api/v1/me/checkout", token, body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Contains(t, response["error"], domorder.ErrEmptyOrderItems.Error())
}

func TestCheckout_InvalidPaymentMethod_ReturnsError(t *testing.T) {
	tests := []struct {
		name          string
		paymentMethod string
		expectedError string
	}{
		{
			name:          "Invalid payment method",
			paymentMethod: "INVALID",
			expectedError: "Field validation for 'PaymentMethod' failed on the 'oneof' tag",
		},
		{
			name:          "Empty payment method",
			paymentMethod: "",
			expectedError: "Field validation for 'PaymentMethod' failed on the 'required' tag",
		},
		{
			name:          "Lowercase cod",
			paymentMethod: "cod",
			expectedError: "Field validation for 'PaymentMethod' failed on the 'oneof' tag",
		},
		{
			name:          "Lowercase tamara",
			paymentMethod: "tamara",
			expectedError: "Field validation for 'PaymentMethod' failed on the 'oneof' tag",
		},
		{
			name:          "Credit card (not supported)",
			paymentMethod: "CREDIT_CARD",
			expectedError: "Field validation for 'PaymentMethod' failed on the 'oneof' tag",
		},
		{
			name:          "PayPal (not supported)",
			paymentMethod: "PAYPAL",
			expectedError: "Field validation for 'PaymentMethod' failed on the 'oneof' tag",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api, token, cartRepo, _ := setupCheckoutAPI()
			// Add items to cart first
			cartRepo.AddOrUpdateItem(context.Background(), 100, 1, 2)
			router := api.Router()

			body := map[string]any{
				"payment_method": tt.paymentMethod,
			}

			req := newAuthenticatedCheckoutRequest(http.MethodPost, "/api/v1/me/checkout", token, body)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			var response map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
			require.Contains(t, response["error"], tt.expectedError)
		})
	}
}

func TestCheckout_COD_Success_Returns201Or200(t *testing.T) {
	api, token, cartRepo, orderRepo := setupCheckoutAPI()
	router := api.Router()

	// Add items to cart
	cartRepo.AddOrUpdateItem(context.Background(), 100, 1, 2) // Product 1, quantity 2
	cartRepo.AddOrUpdateItem(context.Background(), 100, 2, 1) // Product 2, quantity 1

	body := map[string]any{
		"payment_method": "COD",
	}

	req := newAuthenticatedCheckoutRequest(http.MethodPost, "/api/v1/me/checkout", token, body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

	// Verify order was created
	require.Equal(t, float64(100), response["user_id"], "user_id should match authenticated user")
	require.Equal(t, "COD", response["payment_method"], "payment method should be COD")
	require.Equal(t, "PENDING", response["status"], "order status should be PENDING")

	// Verify total amount: (10.0 * 2) + (20.0 * 1) = 40.0
	require.Equal(t, 40.0, response["total_amount"], "total amount should be correct")

	// Verify order items
	items, ok := response["items"].([]any)
	require.True(t, ok, "items should be an array")
	require.Len(t, items, 2, "should have 2 order items")

	// Verify cart was cleared
	cartItems, err := cartRepo.ListItems(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, cartItems, 0, "cart should be cleared after successful checkout")

	// Verify order was created in repository
	require.Len(t, orderRepo.createdOrders, 1, "order should be created in repository")
	require.Equal(t, domorder.PaymentCOD, orderRepo.createdOrders[0].PaymentMethod)
}

func TestCheckout_Tamara_Success_Returns201Or200(t *testing.T) {
	api, token, cartRepo, orderRepo := setupCheckoutAPI()
	router := api.Router()

	// Add items to cart
	cartRepo.AddOrUpdateItem(context.Background(), 100, 1, 3) // Product 1, quantity 3

	body := map[string]any{
		"payment_method": "TAMARA",
	}

	req := newAuthenticatedCheckoutRequest(http.MethodPost, "/api/v1/me/checkout", token, body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

	// Verify order was created
	require.Equal(t, float64(100), response["user_id"], "user_id should match authenticated user")
	require.Equal(t, "TAMARA", response["payment_method"], "payment method should be TAMARA")
	require.Equal(t, "PENDING", response["status"], "order status should be PENDING")

	// Verify total amount: 10.0 * 3 = 30.0
	require.Equal(t, 30.0, response["total_amount"], "total amount should be correct")

	// Verify order items
	items, ok := response["items"].([]any)
	require.True(t, ok, "items should be an array")
	require.Len(t, items, 1, "should have 1 order item")

	// Verify cart was cleared
	cartItems, err := cartRepo.ListItems(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, cartItems, 0, "cart should be cleared after successful checkout")

	// Verify order was created in repository
	require.Len(t, orderRepo.createdOrders, 1, "order should be created in repository")
	require.Equal(t, domorder.PaymentTamara, orderRepo.createdOrders[0].PaymentMethod)
}

func TestCheckout_WithoutAuth_Returns401Or403(t *testing.T) {
	api, _, _, _ := setupCheckoutAPI()
	router := api.Router()

	body := map[string]any{
		"payment_method": "COD",
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/checkout", bytes.NewReader(func() []byte {
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

func TestCheckout_MissingPaymentMethod_Returns400(t *testing.T) {
	api, token, cartRepo, _ := setupCheckoutAPI()
	// Add items to cart first
	cartRepo.AddOrUpdateItem(context.Background(), 100, 1, 1)
	router := api.Router()

	body := map[string]any{
		// payment_method is missing
	}

	req := newAuthenticatedCheckoutRequest(http.MethodPost, "/api/v1/me/checkout", token, body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Contains(t, response["error"], "Field validation for 'PaymentMethod' failed on the 'required' tag")
}

func TestCheckout_InvalidJSON_Returns400(t *testing.T) {
	api, token, _, _ := setupCheckoutAPI()
	router := api.Router()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/checkout", bytes.NewReader([]byte(`{"payment_method": 123}`)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
	require.Contains(t, response["error"], "cannot unmarshal")
}

func TestCheckout_CreatesOrderWithMultipleItems(t *testing.T) {
	api, token, cartRepo, orderRepo := setupCheckoutAPI()
	router := api.Router()

	// Add multiple items to cart
	cartRepo.AddOrUpdateItem(context.Background(), 100, 1, 2) // Product 1: 10.0 * 2 = 20.0
	cartRepo.AddOrUpdateItem(context.Background(), 100, 2, 3) // Product 2: 20.0 * 3 = 60.0
	cartRepo.AddOrUpdateItem(context.Background(), 100, 3, 1) // Product 3: 30.0 * 1 = 30.0
	// Total: 20.0 + 60.0 + 30.0 = 110.0

	body := map[string]any{
		"payment_method": "COD",
	}

	req := newAuthenticatedCheckoutRequest(http.MethodPost, "/api/v1/me/checkout", token, body)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

	require.Equal(t, 110.0, response["total_amount"], "total amount should be sum of all items")

	items, ok := response["items"].([]any)
	require.True(t, ok)
	require.Len(t, items, 3, "should have 3 order items")

	// Verify cart was cleared
	cartItems, err := cartRepo.ListItems(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, cartItems, 0, "cart should be cleared")

	// Verify order in repository
	require.Len(t, orderRepo.createdOrders, 1)
	require.Equal(t, 110.0, orderRepo.createdOrders[0].TotalAmount)
	require.Len(t, orderRepo.createdOrders[0].Items, 3)
}

func TestCheckout_UserIsolation(t *testing.T) {
	api, token1, cartRepo1, _ := setupCheckoutAPI()
	router := api.Router()

	// Add items for user 1
	cartRepo1.AddOrUpdateItem(context.Background(), 100, 1, 2)

	// Create token for user 2
	tokenSvc := security.NewJWTService("test-secret", time.Hour)
	token2, _ := tokenSvc.GenerateToken(&domuser.User{
		ID:       200,
		Name:     "Another Customer",
		Email:    "customer2@example.com",
		RoleCode: domuser.RoleCodeCustomer,
	})

	// Setup separate repositories for user 2 (in real scenario, they'd share the same repo)
	cartRepo2 := newMockCheckoutCartRepository()
	productRepo2 := newMockCheckoutProductRepository()
	orderRepo2 := newMockCheckoutOrderRepository()
	cartSvc2 := cartuc.NewService(cartRepo2, productRepo2, orderRepo2)

	// Add items for user 2
	cartRepo2.AddOrUpdateItem(context.Background(), 200, 2, 1)

	// Checkout for user 1
	body1 := map[string]any{
		"payment_method": "COD",
	}
	req1 := newAuthenticatedCheckoutRequest(http.MethodPost, "/api/v1/me/checkout", token1, body1)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)

	require.Equal(t, http.StatusCreated, rec1.Code, rec1.Body.String())

	var response1 map[string]any
	require.NoError(t, json.Unmarshal(rec1.Body.Bytes(), &response1))
	require.Equal(t, float64(100), response1["user_id"], "order should belong to user 1")

	// Verify user 1's cart is cleared
	items1, err := cartRepo1.ListItems(context.Background(), 100)
	require.NoError(t, err)
	require.Len(t, items1, 0, "user 1's cart should be cleared")

	// Verify user 2's cart is NOT affected (still has items)
	items2, err := cartRepo2.ListItems(context.Background(), 200)
	require.NoError(t, err)
	require.Len(t, items2, 1, "user 2's cart should still have items")

	// Create a new API instance with user 2's services for checkout
	api2 := NewAPI(Dependencies{
		CartService:  cartSvc2,
		TokenService: tokenSvc,
	})
	router2 := api2.Router()

	// Checkout for user 2
	body2 := map[string]any{
		"payment_method": "TAMARA",
	}
	req2 := newAuthenticatedCheckoutRequest(http.MethodPost, "/api/v1/me/checkout", token2, body2)
	rec2 := httptest.NewRecorder()
	router2.ServeHTTP(rec2, req2)

	require.Equal(t, http.StatusCreated, rec2.Code, rec2.Body.String())

	var response2 map[string]any
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &response2))
	require.Equal(t, float64(200), response2["user_id"], "order should belong to user 2")
	require.Equal(t, "TAMARA", response2["payment_method"], "user 2's order should use TAMARA")
}

func TestCheckout_ValidPaymentMethods(t *testing.T) {
	tests := []struct {
		name          string
		paymentMethod string
		expectedCode  int
	}{
		{
			name:          "COD payment method",
			paymentMethod: "COD",
			expectedCode:  http.StatusCreated,
		},
		{
			name:          "TAMARA payment method",
			paymentMethod: "TAMARA",
			expectedCode:  http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api, token, cartRepo, orderRepo := setupCheckoutAPI()
			// Add items to cart
			cartRepo.AddOrUpdateItem(context.Background(), 100, 1, 1)
			router := api.Router()

			body := map[string]any{
				"payment_method": tt.paymentMethod,
			}

			req := newAuthenticatedCheckoutRequest(http.MethodPost, "/api/v1/me/checkout", token, body)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, tt.expectedCode, rec.Code, rec.Body.String())

			if tt.expectedCode == http.StatusCreated {
				var response map[string]any
				require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
				require.Equal(t, tt.paymentMethod, response["payment_method"])

				// Verify cart was cleared
				cartItems, err := cartRepo.ListItems(context.Background(), 100)
				require.NoError(t, err)
				require.Len(t, cartItems, 0, "cart should be cleared after successful checkout")

				// Verify order was created
				require.Len(t, orderRepo.createdOrders, 1, "order should be created")
			}
		})
	}
}

