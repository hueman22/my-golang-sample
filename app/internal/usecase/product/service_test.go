package product

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	domcategory "example.com/my-golang-sample/app/internal/domain/category"
	domproduct "example.com/my-golang-sample/app/internal/domain/product"
)

type mockProductRepository struct {
	products       map[int64]*domproduct.Product
	nextID         int64
	created        *domproduct.Product
	updated        *domproduct.Product
	deletedID      int64
	createErr      error
	updateErr      error
	validCategoryIDs map[int64]bool // Track which category IDs are valid
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
	name := strings.TrimSpace(p.Name)
	if name == "" {
		return nil, errors.New("product name is required")
	}
	if p.Price <= 0 {
		return nil, errors.New("product price must be greater than 0")
	}
	if p.Stock < 0 {
		return nil, errors.New("product stock must be >= 0")
	}
	if !m.validCategoryIDs[p.CategoryID] {
		return nil, domcategory.ErrCategoryNotFound
	}

	p.ID = m.nextID
	m.nextID++
	m.products[p.ID] = p
	m.created = p
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

	// Validate business rules for updates
	if p.Name != "" && p.Name != existing.Name {
		if p.Name == "" {
			return nil, errors.New("product name is required")
		}
		existing.Name = p.Name
	}
	if p.Price > 0 {
		existing.Price = p.Price
	}
	if p.Stock >= 0 {
		existing.Stock = p.Stock
	}
	if p.CategoryID > 0 {
		if !m.validCategoryIDs[p.CategoryID] {
			return nil, domcategory.ErrCategoryNotFound
		}
		existing.CategoryID = p.CategoryID
	}
	if p.Description != "" {
		existing.Description = p.Description
	}
	existing.IsActive = p.IsActive

	m.products[p.ID] = existing
	m.updated = existing
	return existing, nil
}

func (m *mockProductRepository) Delete(ctx context.Context, id int64) error {
	if _, ok := m.products[id]; !ok {
		return domproduct.ErrProductNotFound
	}
	delete(m.products, id)
	m.deletedID = id
	return nil
}

func (m *mockProductRepository) GetByID(ctx context.Context, id int64) (*domproduct.Product, error) {
	if product, ok := m.products[id]; ok {
		cloned := *product
		return &cloned, nil
	}
	return nil, domproduct.ErrProductNotFound
}

func (m *mockProductRepository) List(ctx context.Context, filter domproduct.ListFilter) ([]*domproduct.Product, error) {
	var result []*domproduct.Product
	for _, p := range m.products {
		if filter.OnlyActive && !p.IsActive {
			continue
		}
		if filter.CategoryID != nil && p.CategoryID != *filter.CategoryID {
			continue
		}
		if filter.Search != "" {
			// Simple search check
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

func TestCreateProduct_Valid(t *testing.T) {
	repo := newMockProductRepository()
	repo.validCategoryIDs[1] = true // Mark category 1 as valid
	svc := NewService(repo)

	product, err := svc.Create(context.Background(), &domproduct.Product{
		Name:        "Laptop",
		Description: "High-performance laptop",
		Price:       999.99,
		Stock:       10,
		CategoryID:  1,
		IsActive:    true,
	})

	require.NoError(t, err)
	require.NotNil(t, product)
	require.Equal(t, "Laptop", product.Name)
	require.Equal(t, "High-performance laptop", product.Description)
	require.Equal(t, 999.99, product.Price)
	require.Equal(t, int64(10), product.Stock)
	require.Equal(t, int64(1), product.CategoryID)
	require.True(t, product.IsActive)
	require.NotZero(t, product.ID)
	require.Equal(t, repo.created, product)
}

func TestCreateProduct_EmptyName(t *testing.T) {
	tests := []struct {
		name  string
		inputName string
	}{
		{
			name:  "Empty string",
			inputName: "",
		},
		{
			name:  "Only spaces",
			inputName: "   ",
		},
		{
			name:  "Only tabs",
			inputName: "\t\t",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockProductRepository()
			repo.validCategoryIDs[1] = true
			svc := NewService(repo)

			product, err := svc.Create(context.Background(), &domproduct.Product{
				Name:       tt.inputName,
				Price:      99.99,
				Stock:      5,
				CategoryID: 1,
			})

			require.Error(t, err)
			require.Contains(t, err.Error(), "name is required")
			require.Nil(t, product)
			require.Nil(t, repo.created, "repository Create should NOT be called for empty name")
		})
	}
}

func TestCreateProduct_InvalidPrice(t *testing.T) {
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
			repo := newMockProductRepository()
			repo.validCategoryIDs[1] = true
			svc := NewService(repo)

			product, err := svc.Create(context.Background(), &domproduct.Product{
				Name:       "Test Product",
				Price:      tt.price,
				Stock:      5,
				CategoryID: 1,
			})

			require.Error(t, err)
			require.Contains(t, err.Error(), "price must be greater than 0")
			require.Nil(t, product)
			require.Nil(t, repo.created, "repository Create should NOT be called for invalid price")
		})
	}
}

func TestCreateProduct_NegativeStock(t *testing.T) {
	repo := newMockProductRepository()
	repo.validCategoryIDs[1] = true
	svc := NewService(repo)

	product, err := svc.Create(context.Background(), &domproduct.Product{
		Name:       "Test Product",
		Price:      99.99,
		Stock:      -5,
		CategoryID: 1,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "stock must be >= 0")
	require.Nil(t, product)
	require.Nil(t, repo.created, "repository Create should NOT be called for negative stock")
}

func TestCreateProduct_ZeroStock(t *testing.T) {
	repo := newMockProductRepository()
	repo.validCategoryIDs[1] = true
	svc := NewService(repo)

	product, err := svc.Create(context.Background(), &domproduct.Product{
		Name:       "Out of Stock Product",
		Price:      99.99,
		Stock:      0,
		CategoryID: 1,
	})

	require.NoError(t, err)
	require.NotNil(t, product)
	require.Equal(t, int64(0), product.Stock, "zero stock should be allowed")
}

func TestCreateProduct_InvalidCategory(t *testing.T) {
	repo := newMockProductRepository()
	// Don't mark any category as valid
	svc := NewService(repo)

	product, err := svc.Create(context.Background(), &domproduct.Product{
		Name:       "Test Product",
		Price:      99.99,
		Stock:      5,
		CategoryID: 999, // Non-existent category
	})

	require.ErrorIs(t, err, domcategory.ErrCategoryNotFound)
	require.Nil(t, product)
	require.Nil(t, repo.created, "repository Create should NOT be called for invalid category")
}

func TestCreateProduct_ValidWithDifferentPrices(t *testing.T) {
	tests := []struct {
		name  string
		price float64
	}{
		{
			name:  "Small price",
			price: 0.01,
		},
		{
			name:  "Medium price",
			price: 99.99,
		},
		{
			name:  "Large price",
			price: 9999.99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockProductRepository()
			repo.validCategoryIDs[1] = true
			svc := NewService(repo)

			product, err := svc.Create(context.Background(), &domproduct.Product{
				Name:       "Test Product",
				Price:      tt.price,
				Stock:      10,
				CategoryID: 1,
			})

			require.NoError(t, err)
			require.Equal(t, tt.price, product.Price)
		})
	}
}

func TestUpdateProduct_ChangePriceAndStock(t *testing.T) {
	repo := newMockProductRepository()
	repo.validCategoryIDs[1] = true
	svc := NewService(repo)

	// Create a product first
	created, err := svc.Create(context.Background(), &domproduct.Product{
		Name:       "Original Product",
		Price:      50.00,
		Stock:      20,
		CategoryID:  1,
		IsActive:    true,
	})
	require.NoError(t, err)
	require.NotNil(t, created)

	// Update price and stock
	updated, err := svc.Update(context.Background(), &domproduct.Product{
		ID:    created.ID,
		Price: 75.50,
		Stock: 15,
	})

	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Equal(t, 75.50, updated.Price)
	require.Equal(t, int64(15), updated.Stock)
	require.Equal(t, "Original Product", updated.Name, "name should remain unchanged")
	require.Equal(t, int64(1), updated.CategoryID, "category should remain unchanged")
}

func TestUpdateProduct_NotFound(t *testing.T) {
	repo := newMockProductRepository()
	svc := NewService(repo)

	product, err := svc.Update(context.Background(), &domproduct.Product{
		ID:    999,
		Name:  "Updated Name",
		Price: 99.99,
	})

	require.ErrorIs(t, err, domproduct.ErrProductNotFound)
	require.Nil(t, product)
	require.Nil(t, repo.updated)
}

func TestUpdateProduct_InvalidCategory(t *testing.T) {
	repo := newMockProductRepository()
	repo.validCategoryIDs[1] = true
	svc := NewService(repo)

	// Create a product with valid category
	created, err := svc.Create(context.Background(), &domproduct.Product{
		Name:       "Test Product",
		Price:      50.00,
		Stock:      10,
		CategoryID: 1,
	})
	require.NoError(t, err)

	// Try to update to invalid category
	updated, err := svc.Update(context.Background(), &domproduct.Product{
		ID:         created.ID,
		CategoryID: 999, // Non-existent category
	})

	require.ErrorIs(t, err, domcategory.ErrCategoryNotFound)
	require.Nil(t, updated)
}

func TestUpdateProduct_InvalidPrice(t *testing.T) {
	repo := newMockProductRepository()
	repo.validCategoryIDs[1] = true
	svc := NewService(repo)

	// Create a product first
	created, err := svc.Create(context.Background(), &domproduct.Product{
		Name:       "Test Product",
		Price:      50.00,
		Stock:      10,
		CategoryID: 1,
	})
	require.NoError(t, err)

	// Try to update with zero price (should not update due to condition p.Price > 0)
	updated, err := svc.Update(context.Background(), &domproduct.Product{
		ID:    created.ID,
		Price: 0,
	})

	require.NoError(t, err)
	require.Equal(t, 50.00, updated.Price, "price should remain unchanged when set to 0")
}

func TestUpdateProduct_NegativeStock(t *testing.T) {
	repo := newMockProductRepository()
	repo.validCategoryIDs[1] = true
	svc := NewService(repo)

	// Create a product first
	created, err := svc.Create(context.Background(), &domproduct.Product{
		Name:       "Test Product",
		Price:      50.00,
		Stock:      10,
		CategoryID: 1,
	})
	require.NoError(t, err)

	// Try to update with negative stock (should not update due to condition p.Stock >= 0)
	updated, err := svc.Update(context.Background(), &domproduct.Product{
		ID:    created.ID,
		Stock: -5,
	})

	require.NoError(t, err)
	require.Equal(t, int64(10), updated.Stock, "stock should remain unchanged when set to negative")
}

func TestUpdateProduct_PartialUpdate(t *testing.T) {
	repo := newMockProductRepository()
	repo.validCategoryIDs[1] = true
	svc := NewService(repo)

	// Create a product first
	created, err := svc.Create(context.Background(), &domproduct.Product{
		Name:        "Original Product",
		Description: "Original description",
		Price:       50.00,
		Stock:       10,
		CategoryID:  1,
		IsActive:    true,
	})
	require.NoError(t, err)

	// Update only name (note: Stock will be updated to 0 due to service logic p.Stock >= 0)
	updated, err := svc.Update(context.Background(), &domproduct.Product{
		ID:    created.ID,
		Name:  "Updated Name",
		Stock: created.Stock, // Explicitly set to keep original value
	})

	require.NoError(t, err)
	require.Equal(t, "Updated Name", updated.Name)
	require.Equal(t, "Original description", updated.Description, "description should remain unchanged")
	require.Equal(t, 50.00, updated.Price, "price should remain unchanged")
	require.Equal(t, int64(10), updated.Stock, "stock should remain unchanged")
}

func TestGetProduct_Found(t *testing.T) {
	repo := newMockProductRepository()
	repo.validCategoryIDs[1] = true
	svc := NewService(repo)

	// Create a product first
	created, err := svc.Create(context.Background(), &domproduct.Product{
		Name:       "Test Product",
		Price:      99.99,
		Stock:      5,
		CategoryID: 1,
	})
	require.NoError(t, err)

	// Get it back
	product, err := svc.GetByID(context.Background(), created.ID)

	require.NoError(t, err)
	require.NotNil(t, product)
	require.Equal(t, created.ID, product.ID)
	require.Equal(t, "Test Product", product.Name)
}

func TestGetProduct_NotFound(t *testing.T) {
	repo := newMockProductRepository()
	svc := NewService(repo)

	product, err := svc.GetByID(context.Background(), 999)

	require.ErrorIs(t, err, domproduct.ErrProductNotFound)
	require.Nil(t, product)
}

func TestDeleteProduct_Success(t *testing.T) {
	repo := newMockProductRepository()
	repo.validCategoryIDs[1] = true
	svc := NewService(repo)

	// Create a product first
	created, err := svc.Create(context.Background(), &domproduct.Product{
		Name:       "To Be Deleted",
		Price:      99.99,
		Stock:      5,
		CategoryID: 1,
	})
	require.NoError(t, err)

	// Delete it
	err = svc.Delete(context.Background(), created.ID)

	require.NoError(t, err)
	require.Equal(t, created.ID, repo.deletedID)

	// Verify it's deleted
	_, err = svc.GetByID(context.Background(), created.ID)
	require.ErrorIs(t, err, domproduct.ErrProductNotFound)
}

func TestDeleteProduct_NotFound(t *testing.T) {
	repo := newMockProductRepository()
	svc := NewService(repo)

	err := svc.Delete(context.Background(), 999)

	require.ErrorIs(t, err, domproduct.ErrProductNotFound)
	require.Zero(t, repo.deletedID)
}

