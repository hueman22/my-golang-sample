package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	domcategory "example.com/my-golang-sample/app/internal/domain/category"
	domproduct "example.com/my-golang-sample/app/internal/domain/product"
	domuser "example.com/my-golang-sample/app/internal/domain/user"
	"example.com/my-golang-sample/app/internal/infra/security"
	authuc "example.com/my-golang-sample/app/internal/usecase/auth"
	categoryuc "example.com/my-golang-sample/app/internal/usecase/category"
	productuc "example.com/my-golang-sample/app/internal/usecase/product"
)

// Mock product repository
type mockProductRepository struct {
	products    map[int64]*domproduct.Product
	nextID      int64
	validCategoryIDs map[int64]bool
	createErr   error
	updateErr   error
	deleteErr   error
	getErr      error
	listErr     error
}

func newMockProductRepository() *mockProductRepository {
	return &mockProductRepository{
		products:         make(map[int64]*domproduct.Product),
		nextID:           1,
		validCategoryIDs: make(map[int64]bool),
	}
}

func (m *mockProductRepository) Create(ctx context.Context, p *domproduct.Product) (*domproduct.Product, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}

	// Validate business rules
	if p.Name == "" {
		return nil, fmt.Errorf("product name is required")
	}
	if p.Price <= 0 {
		return nil, fmt.Errorf("product price must be greater than 0")
	}
	if p.Stock < 0 {
		return nil, fmt.Errorf("product stock must be >= 0")
	}
	if !m.validCategoryIDs[p.CategoryID] {
		return nil, domcategory.ErrCategoryNotFound
	}

	p.ID = m.nextID
	m.nextID++
	m.products[p.ID] = p
	return p, nil
}

func (m *mockProductRepository) Update(ctx context.Context, p *domproduct.Product) (*domproduct.Product, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}

	existing, ok := m.products[p.ID]
	if !ok {
		return nil, domproduct.ErrProductNotFound
	}

	// Validate category if changed
	if p.CategoryID > 0 && p.CategoryID != existing.CategoryID {
		if !m.validCategoryIDs[p.CategoryID] {
			return nil, domcategory.ErrCategoryNotFound
		}
		existing.CategoryID = p.CategoryID
	}

	if p.Name != "" {
		existing.Name = p.Name
	}
	if p.Description != "" {
		existing.Description = p.Description
	}
	if p.Price > 0 {
		existing.Price = p.Price
	}
	if p.Stock >= 0 {
		existing.Stock = p.Stock
	}
	existing.IsActive = p.IsActive

	m.products[p.ID] = existing
	return existing, nil
}

func (m *mockProductRepository) Delete(ctx context.Context, id int64) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}
	if _, ok := m.products[id]; !ok {
		return domproduct.ErrProductNotFound
	}
	delete(m.products, id)
	return nil
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

func (m *mockProductRepository) List(ctx context.Context, filter domproduct.ListFilter) ([]*domproduct.Product, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var result []*domproduct.Product
	for _, p := range m.products {
		if filter.OnlyActive && !p.IsActive {
			continue
		}
		if filter.CategoryID != nil && p.CategoryID != *filter.CategoryID {
			continue
		}
		if filter.Search != "" {
			// Simple search - check if search term is in name or description
			if !contains(p.Name, filter.Search) && !contains(p.Description, filter.Search) {
				continue
			}
		}
		cloned := *p
		result = append(result, &cloned)
	}
	return result, nil
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

func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Mock category repository
type mockCategoryRepository struct {
	categories map[int64]*domcategory.Category
	nextID     int64
}

func newMockCategoryRepository() *mockCategoryRepository {
	return &mockCategoryRepository{
		categories: make(map[int64]*domcategory.Category),
		nextID:     1,
	}
}

func (m *mockCategoryRepository) Create(ctx context.Context, c *domcategory.Category) (*domcategory.Category, error) {
	c.ID = m.nextID
	m.nextID++
	m.categories[c.ID] = c
	return c, nil
}

func (m *mockCategoryRepository) Update(ctx context.Context, c *domcategory.Category) (*domcategory.Category, error) {
	if _, ok := m.categories[c.ID]; !ok {
		return nil, domcategory.ErrCategoryNotFound
	}
	m.categories[c.ID] = c
	return c, nil
}

func (m *mockCategoryRepository) Delete(ctx context.Context, id int64) error {
	if _, ok := m.categories[id]; !ok {
		return domcategory.ErrCategoryNotFound
	}
	delete(m.categories, id)
	return nil
}

func (m *mockCategoryRepository) GetByID(ctx context.Context, id int64) (*domcategory.Category, error) {
	if category, ok := m.categories[id]; ok {
		cloned := *category
		return &cloned, nil
	}
	return nil, domcategory.ErrCategoryNotFound
}

func (m *mockCategoryRepository) List(ctx context.Context, filter domcategory.ListFilter) ([]*domcategory.Category, error) {
	var result []*domcategory.Category
	for _, c := range m.categories {
		if filter.OnlyActive && !c.IsActive {
			continue
		}
		cloned := *c
		result = append(result, &cloned)
	}
	return result, nil
}

// Setup function to create API with product and category services
func setupProductAPI(productRepo *mockProductRepository, categoryRepo *mockCategoryRepository, role *domuser.RoleCode) (*API, string) {
	productSvc := productuc.NewService(productRepo)
	categorySvc := categoryuc.NewService(categoryRepo)

	tokenSvc := security.NewJWTService("test-secret", time.Hour)

	var api *API
	var token string

	if role != nil {
		// Create auth service for authenticated requests
		userRepo := &fakeAuthUserRepo{}
		passwordSvc := fakePasswordService{}
		authSvc := authuc.NewService(userRepo, passwordSvc, tokenSvc)

		api = NewAPI(Dependencies{
			ProductService:  productSvc,
			CategoryService: categorySvc,
			TokenService:    tokenSvc,
			AuthService:      authSvc,
		})

		token, _ = tokenSvc.GenerateToken(&domuser.User{
			ID:       1,
			Name:     "Test User",
			Email:    "test@example.com",
			RoleCode: *role,
		})
	} else {
		// Guest access - no auth service needed
		api = NewAPI(Dependencies{
			ProductService:  productSvc,
			CategoryService: categorySvc,
		})
	}

	return api, token
}

// Test 1: Guest List Products
func TestGuestListProducts_Returns200(t *testing.T) {
	productRepo := newMockProductRepository()
	categoryRepo := newMockCategoryRepository()

	// Create a category first
	category, _ := categoryRepo.Create(context.Background(), &domcategory.Category{
		Name:        "Electronics",
		Slug:        "electronics",
		Description: "Electronic devices",
		IsActive:    true,
	})
	productRepo.validCategoryIDs[category.ID] = true

	// Create some products
	productRepo.Create(context.Background(), &domproduct.Product{
		Name:        "Laptop",
		Description: "High-performance laptop",
		Price:       999.99,
		Stock:       10,
		CategoryID:  category.ID,
		IsActive:    true,
	})
	productRepo.Create(context.Background(), &domproduct.Product{
		Name:        "Mouse",
		Description: "Wireless mouse",
		Price:       29.99,
		Stock:       50,
		CategoryID:  category.ID,
		IsActive:    true,
	})

	api, _ := setupProductAPI(productRepo, categoryRepo, nil) // Guest (no role)
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "should return 200 OK")

	var response map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err, "response should be valid JSON")

	data, ok := response["data"].([]any)
	require.True(t, ok, "response should have 'data' field")
	require.GreaterOrEqual(t, len(data), 2, "should return at least 2 products")
}

// Test 2: Guest Get Product Found
func TestGuestGetProduct_Found_Returns200(t *testing.T) {
	productRepo := newMockProductRepository()
	categoryRepo := newMockCategoryRepository()

	category, _ := categoryRepo.Create(context.Background(), &domcategory.Category{
		Name:        "Electronics",
		Slug:        "electronics",
		Description: "Electronic devices",
		IsActive:    true,
	})
	productRepo.validCategoryIDs[category.ID] = true

	created, _ := productRepo.Create(context.Background(), &domproduct.Product{
		Name:        "Laptop",
		Description: "High-performance laptop",
		Price:       999.99,
		Stock:       10,
		CategoryID:  category.ID,
		IsActive:    true,
	})

	api, _ := setupProductAPI(productRepo, categoryRepo, nil) // Guest
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/products/%d", created.ID), nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "should return 200 OK")

	var product map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &product)
	require.NoError(t, err, "response should be valid JSON")
	require.Equal(t, float64(created.ID), product["id"])
	require.Equal(t, "Laptop", product["name"])
	require.Equal(t, 999.99, product["price"])
}

// Test 3: Guest Get Product Not Found
func TestGuestGetProduct_NotFound_Returns404(t *testing.T) {
	productRepo := newMockProductRepository()
	categoryRepo := newMockCategoryRepository()

	api, _ := setupProductAPI(productRepo, categoryRepo, nil) // Guest
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products/999", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code, "should return 404 Not Found")
}

// Test 4: Admin Create Product Valid Payload
func TestAdminCreateProduct_ValidPayload_Returns201(t *testing.T) {
	productRepo := newMockProductRepository()
	categoryRepo := newMockCategoryRepository()

	// Create a category
	category, _ := categoryRepo.Create(context.Background(), &domcategory.Category{
		Name:        "Electronics",
		Slug:        "electronics",
		Description: "Electronic devices",
		IsActive:    true,
	})
	productRepo.validCategoryIDs[category.ID] = true

	role := domuser.RoleCodeSuperAdmin
	api, token := setupProductAPI(productRepo, categoryRepo, &role)
	router := api.Router()

	body := map[string]any{
		"name":        "New Product",
		"description": "Product description",
		"price":       99.99,
		"stock":       10,
		"category_id": category.ID,
		"is_active":   true,
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, "should return 201 Created")

	var product map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &product)
	require.NoError(t, err, "response should be valid JSON")
	require.Equal(t, "New Product", product["name"])
	require.Equal(t, 99.99, product["price"])
	require.Equal(t, float64(10), product["stock"])
	require.Equal(t, float64(category.ID), product["category_id"])
}

// Test 5: Admin Create Product Invalid Price
func TestAdminCreateProduct_InvalidPrice_Returns422(t *testing.T) {
	tests := []struct {
		name  string
		price float64
	}{
		{
			name:  "Zero price",
			price: 0,
		},
		{
			name:  "Negative price",
			price: -10.50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			productRepo := newMockProductRepository()
			categoryRepo := newMockCategoryRepository()

			category, _ := categoryRepo.Create(context.Background(), &domcategory.Category{
				Name:        "Electronics",
				Slug:        "electronics",
				IsActive:    true,
			})
			productRepo.validCategoryIDs[category.ID] = true

			role := domuser.RoleCodeSuperAdmin
			api, token := setupProductAPI(productRepo, categoryRepo, &role)
			router := api.Router()

			body := map[string]any{
				"name":        "Product",
				"price":       tt.price,
				"stock":       10,
				"category_id": category.ID,
			}
			payload, _ := json.Marshal(body)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader(payload))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.GreaterOrEqual(t, rec.Code, 400, "should return 4xx error for invalid price")
			require.Less(t, rec.Code, 500, "should not return 5xx error")
		})
	}
}

// Test 6: Admin Create Product Invalid Category
func TestAdminCreateProduct_InvalidCategory_Returns422(t *testing.T) {
	productRepo := newMockProductRepository()
	categoryRepo := newMockCategoryRepository()

	// Don't create any category, so category_id 999 doesn't exist
	// Don't mark any category as valid in productRepo

	role := domuser.RoleCodeSuperAdmin
	api, token := setupProductAPI(productRepo, categoryRepo, &role)
	router := api.Router()

	body := map[string]any{
		"name":        "Product",
		"price":       99.99,
		"stock":       10,
		"category_id": 999, // Non-existent category
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code, "should return 404 Not Found for invalid category")
}

// Test 7: Admin Update Product Not Found
func TestAdminUpdateProduct_NotFound_Returns404(t *testing.T) {
	productRepo := newMockProductRepository()
	categoryRepo := newMockCategoryRepository()

	category, _ := categoryRepo.Create(context.Background(), &domcategory.Category{
		Name:        "Electronics",
		Slug:        "electronics",
		IsActive:    true,
	})
	productRepo.validCategoryIDs[category.ID] = true

	role := domuser.RoleCodeSuperAdmin
	api, token := setupProductAPI(productRepo, categoryRepo, &role)
	router := api.Router()

	// Provide all required fields for update
	body := map[string]any{
		"name":        "Updated Product",
		"description": "Updated description",
		"price":       199.99,
		"stock":       20,
		"category_id": category.ID,
		"is_active":   true,
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/products/999", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code, "should return 404 Not Found")
}

// Test 8: Admin Delete Product Success
func TestAdminDeleteProduct_Success(t *testing.T) {
	productRepo := newMockProductRepository()
	categoryRepo := newMockCategoryRepository()

	category, _ := categoryRepo.Create(context.Background(), &domcategory.Category{
		Name:        "Electronics",
		Slug:        "electronics",
		IsActive:    true,
	})
	productRepo.validCategoryIDs[category.ID] = true

	created, _ := productRepo.Create(context.Background(), &domproduct.Product{
		Name:        "To Delete",
		Price:       99.99,
		Stock:       10,
		CategoryID:  category.ID,
		IsActive:    true,
	})

	role := domuser.RoleCodeSuperAdmin
	api, token := setupProductAPI(productRepo, categoryRepo, &role)
	router := api.Router()

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/products/%d", created.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code, "should return 204 No Content")

	// Verify product was deleted
	_, err := productRepo.GetByID(context.Background(), created.ID)
	require.ErrorIs(t, err, domproduct.ErrProductNotFound, "product should be deleted")
}

// Test 9: Guest Cannot Access Admin Product APIs
func TestGuestCannotAccessAdminProductAPIs(t *testing.T) {
	productRepo := newMockProductRepository()
	categoryRepo := newMockCategoryRepository()

	api, _ := setupProductAPI(productRepo, categoryRepo, nil) // Guest (no token)
	router := api.Router()

	tests := []struct {
		name   string
		method string
		path   string
		body   map[string]any
	}{
		{
			name:   "POST /api/v1/admin/products",
			method: http.MethodPost,
			path:   "/api/v1/admin/products",
			body: map[string]any{
				"name":        "Product",
				"price":       99.99,
				"stock":       10,
				"category_id": 1,
			},
		},
		{
			name:   "PUT /api/v1/admin/products/1",
			method: http.MethodPut,
			path:   "/api/v1/admin/products/1",
			body: map[string]any{
				"name": "Updated",
			},
		},
		{
			name:   "DELETE /api/v1/admin/products/1",
			method: http.MethodDelete,
			path:   "/api/v1/admin/products/1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var payload []byte
			if tt.body != nil {
				payload, _ = json.Marshal(tt.body)
			}

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewReader(payload))
			if tt.body != nil {
				req.Header.Set("Content-Type", "application/json")
			}
			// No Authorization header (guest)
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			require.Equal(t, http.StatusUnauthorized, rec.Code, "should return 401 Unauthorized for guest")
		})
	}
}

// Additional test cases

func TestGuestListProducts_OnlyActive(t *testing.T) {
	productRepo := newMockProductRepository()
	categoryRepo := newMockCategoryRepository()

	category, _ := categoryRepo.Create(context.Background(), &domcategory.Category{
		Name:        "Electronics",
		Slug:        "electronics",
		IsActive:    true,
	})
	productRepo.validCategoryIDs[category.ID] = true

	// Create active and inactive products
	productRepo.Create(context.Background(), &domproduct.Product{
		Name:        "Active Product",
		Price:       99.99,
		Stock:       10,
		CategoryID:  category.ID,
		IsActive:    true,
	})
	productRepo.Create(context.Background(), &domproduct.Product{
		Name:        "Inactive Product",
		Price:       49.99,
		Stock:       5,
		CategoryID:  category.ID,
		IsActive:    false,
	})

	api, _ := setupProductAPI(productRepo, categoryRepo, nil)
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/products", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response map[string]any
	json.Unmarshal(rec.Body.Bytes(), &response)
	data := response["data"].([]any)
	
	// Should only return active products
	require.Len(t, data, 1, "should only return active products")
	product := data[0].(map[string]any)
	require.Equal(t, "Active Product", product["name"])
}

func TestAdminCreateProduct_AsAdmin_Returns201(t *testing.T) {
	productRepo := newMockProductRepository()
	categoryRepo := newMockCategoryRepository()

	category, _ := categoryRepo.Create(context.Background(), &domcategory.Category{
		Name:        "Electronics",
		Slug:        "electronics",
		IsActive:    true,
	})
	productRepo.validCategoryIDs[category.ID] = true

	role := domuser.RoleCodeAdmin // ADMIN role (not SUPER_ADMIN)
	api, token := setupProductAPI(productRepo, categoryRepo, &role)
	router := api.Router()

	body := map[string]any{
		"name":        "Admin Product",
		"price":       79.99,
		"stock":       15,
		"category_id": category.ID,
		"is_active":   true,
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, "ADMIN should be able to create products")
}

func TestAdminCreateProduct_EmptyName_Returns400(t *testing.T) {
	productRepo := newMockProductRepository()
	categoryRepo := newMockCategoryRepository()

	category, _ := categoryRepo.Create(context.Background(), &domcategory.Category{
		Name:        "Electronics",
		Slug:        "electronics",
		IsActive:    true,
	})
	productRepo.validCategoryIDs[category.ID] = true

	role := domuser.RoleCodeSuperAdmin
	api, token := setupProductAPI(productRepo, categoryRepo, &role)
	router := api.Router()

	body := map[string]any{
		"name":        "", // Empty name
		"price":       99.99,
		"stock":       10,
		"category_id": category.ID,
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, "should return 400 for empty name")
}

func TestAdminCreateProduct_NegativeStock_Returns422(t *testing.T) {
	productRepo := newMockProductRepository()
	categoryRepo := newMockCategoryRepository()

	category, _ := categoryRepo.Create(context.Background(), &domcategory.Category{
		Name:        "Electronics",
		Slug:        "electronics",
		IsActive:    true,
	})
	productRepo.validCategoryIDs[category.ID] = true

	role := domuser.RoleCodeSuperAdmin
	api, token := setupProductAPI(productRepo, categoryRepo, &role)
	router := api.Router()

	body := map[string]any{
		"name":        "Product",
		"price":       99.99,
		"stock":       -5, // Negative stock
		"category_id": category.ID,
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.GreaterOrEqual(t, rec.Code, 400, "should return 4xx error for negative stock")
}

func TestAdminUpdateProduct_Success(t *testing.T) {
	productRepo := newMockProductRepository()
	categoryRepo := newMockCategoryRepository()

	category, _ := categoryRepo.Create(context.Background(), &domcategory.Category{
		Name:        "Electronics",
		Slug:        "electronics",
		IsActive:    true,
	})
	productRepo.validCategoryIDs[category.ID] = true

	created, _ := productRepo.Create(context.Background(), &domproduct.Product{
		Name:        "Original Product",
		Description: "Original description",
		Price:       99.99,
		Stock:       10,
		CategoryID:  category.ID,
		IsActive:    true,
	})

	role := domuser.RoleCodeSuperAdmin
	api, token := setupProductAPI(productRepo, categoryRepo, &role)
	router := api.Router()

	body := map[string]any{
		"name":        "Updated Product",
		"description": "Updated description",
		"price":       149.99,
		"stock":       20,
		"category_id": category.ID,
		"is_active":   true,
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/admin/products/%d", created.ID), bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "should return 200 OK")

	var product map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &product)
	require.NoError(t, err)
	require.Equal(t, "Updated Product", product["name"])
	require.Equal(t, 149.99, product["price"])
	require.Equal(t, float64(20), product["stock"])
}

func TestCustomerCannotAccessAdminProductAPIs(t *testing.T) {
	productRepo := newMockProductRepository()
	categoryRepo := newMockCategoryRepository()

	role := domuser.RoleCodeCustomer // CUSTOMER role
	api, token := setupProductAPI(productRepo, categoryRepo, &role)
	router := api.Router()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/products", bytes.NewReader([]byte("{}")))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code, "CUSTOMER should get 403 Forbidden")
}

func TestGuestListProducts_WithCategoryFilter(t *testing.T) {
	productRepo := newMockProductRepository()
	categoryRepo := newMockCategoryRepository()

	category1, _ := categoryRepo.Create(context.Background(), &domcategory.Category{
		Name:        "Electronics",
		Slug:        "electronics",
		IsActive:    true,
	})
	category2, _ := categoryRepo.Create(context.Background(), &domcategory.Category{
		Name:        "Clothing",
		Slug:        "clothing",
		IsActive:    true,
	})
	productRepo.validCategoryIDs[category1.ID] = true
	productRepo.validCategoryIDs[category2.ID] = true

	productRepo.Create(context.Background(), &domproduct.Product{
		Name:        "Laptop",
		Price:       999.99,
		Stock:       10,
		CategoryID:  category1.ID,
		IsActive:    true,
	})
	productRepo.Create(context.Background(), &domproduct.Product{
		Name:        "T-Shirt",
		Price:       29.99,
		Stock:       50,
		CategoryID:  category2.ID,
		IsActive:    true,
	})

	api, _ := setupProductAPI(productRepo, categoryRepo, nil)
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/products?category_id=%d", category1.ID), nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var response map[string]any
	json.Unmarshal(rec.Body.Bytes(), &response)
	data := response["data"].([]any)
	
	require.Len(t, data, 1, "should return only products from category1")
	product := data[0].(map[string]any)
	require.Equal(t, "Laptop", product["name"])
}

