package category

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	dom "example.com/my-golang-sample/app/internal/domain/category"
)

type mockCategoryRepository struct {
	categories map[int64]*dom.Category
	categoriesBySlug map[string]*dom.Category
	nextID     int64
	created    *dom.Category
	updated    *dom.Category
	deletedID  int64
	createErr  error
	updateErr  error
}

func newMockCategoryRepository() *mockCategoryRepository {
	return &mockCategoryRepository{
		categories:       make(map[int64]*dom.Category),
		categoriesBySlug: make(map[string]*dom.Category),
		nextID:           1,
	}
}

func (m *mockCategoryRepository) Create(ctx context.Context, c *dom.Category) (*dom.Category, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}

	// Check for duplicate slug
	if _, exists := m.categoriesBySlug[c.Slug]; exists {
		return nil, dom.ErrCategorySlugExists
	}

	c.ID = m.nextID
	m.nextID++
	m.categories[c.ID] = c
	m.categoriesBySlug[c.Slug] = c
	m.created = c
	return c, nil
}

func (m *mockCategoryRepository) Update(ctx context.Context, c *dom.Category) (*dom.Category, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}

	existing, ok := m.categories[c.ID]
	if !ok {
		return nil, dom.ErrCategoryNotFound
	}

	// Check for duplicate slug (excluding current category)
	if c.Slug != existing.Slug {
		if other, exists := m.categoriesBySlug[c.Slug]; exists && other.ID != c.ID {
			return nil, dom.ErrCategorySlugExists
		}
		delete(m.categoriesBySlug, existing.Slug)
		m.categoriesBySlug[c.Slug] = c
	}

	m.categories[c.ID] = c
	m.updated = c
	return c, nil
}

func (m *mockCategoryRepository) Delete(ctx context.Context, id int64) error {
	category, ok := m.categories[id]
	if !ok {
		return dom.ErrCategoryNotFound
	}
	delete(m.categoriesBySlug, category.Slug)
	delete(m.categories, id)
	m.deletedID = id
	return nil
}

func (m *mockCategoryRepository) GetByID(ctx context.Context, id int64) (*dom.Category, error) {
	if category, ok := m.categories[id]; ok {
		cloned := *category
		return &cloned, nil
	}
	return nil, dom.ErrCategoryNotFound
}

func (m *mockCategoryRepository) List(ctx context.Context, filter dom.ListFilter) ([]*dom.Category, error) {
	var result []*dom.Category
	for _, cat := range m.categories {
		if filter.OnlyActive && !cat.IsActive {
			continue
		}
		cloned := *cat
		result = append(result, &cloned)
	}
	return result, nil
}

func TestCreateCategory_Valid(t *testing.T) {
	repo := newMockCategoryRepository()
	svc := NewService(repo)

	category, err := svc.Create(context.Background(), CreateInput{
		Name:        "Electronics",
		Description: "Electronic devices and gadgets",
	})

	require.NoError(t, err)
	require.NotNil(t, category)
	require.Equal(t, "Electronics", category.Name)
	require.Equal(t, "electronics", category.Slug)
	require.Equal(t, "Electronic devices and gadgets", category.Description)
	require.True(t, category.IsActive, "should default to active")
	require.NotZero(t, category.ID)
	require.Equal(t, repo.created, category)
}

func TestCreateCategory_ValidWithCustomSlug(t *testing.T) {
	repo := newMockCategoryRepository()
	svc := NewService(repo)

	customSlug := "custom-slug-123"
	isActive := false
	category, err := svc.Create(context.Background(), CreateInput{
		Name:        "Home & Garden",
		Slug:        &customSlug,
		Description: "Home improvement items",
		IsActive:    &isActive,
	})

	require.NoError(t, err)
	require.NotNil(t, category)
	require.Equal(t, "Home & Garden", category.Name)
	require.Equal(t, "custom-slug-123", category.Slug)
	require.False(t, category.IsActive)
}

func TestCreateCategory_EmptyName(t *testing.T) {
	tests := []struct {
		name     string
		inputName string
	}{
		{
			name:      "Empty string",
			inputName: "",
		},
		{
			name:      "Only spaces",
			inputName: "   ",
		},
		{
			name:      "Only tabs",
			inputName: "\t\t",
		},
		{
			name:      "Mixed whitespace",
			inputName: " \t \n ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockCategoryRepository()
			svc := NewService(repo)

			category, err := svc.Create(context.Background(), CreateInput{
				Name: tt.inputName,
			})

			require.ErrorIs(t, err, dom.ErrCategoryInvalidName)
			require.Nil(t, category)
			require.Nil(t, repo.created, "repository Create should NOT be called for empty name")
		})
	}
}

func TestCreateCategory_EmptySlug(t *testing.T) {
	tests := []struct {
		name      string
		inputName string
		customSlug *string
	}{
		{
			name:      "Slug generated from name with only special characters",
			inputName: "!!!",
			customSlug: nil,
		},
		{
			name:      "Custom slug with only special characters",
			inputName: "Valid Name",
			customSlug: stringPtr("***"),
		},
		{
			name:      "Custom slug with only spaces",
			inputName: "Valid Name",
			customSlug: stringPtr("   "),
		},
		{
			name:      "Custom slug with only dashes",
			inputName: "Valid Name",
			customSlug: stringPtr("---"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockCategoryRepository()
			svc := NewService(repo)

			category, err := svc.Create(context.Background(), CreateInput{
				Name: tt.inputName,
				Slug: tt.customSlug,
			})

			require.ErrorIs(t, err, dom.ErrCategoryInvalidSlug)
			require.Nil(t, category)
			require.Nil(t, repo.created, "repository Create should NOT be called for empty slug")
		})
	}
}

func TestCreateCategory_DuplicateSlug(t *testing.T) {
	repo := newMockCategoryRepository()
	svc := NewService(repo)

	// Create first category
	firstCategory, err := svc.Create(context.Background(), CreateInput{
		Name: "Electronics",
		Description: "First category",
	})
	require.NoError(t, err)
	require.NotNil(t, firstCategory)
	require.Equal(t, "electronics", firstCategory.Slug)

	// Try to create second category with same slug (via name)
	secondCategory, err := svc.Create(context.Background(), CreateInput{
		Name: "Electronics",
		Description: "Second category",
	})

	require.ErrorIs(t, err, dom.ErrCategorySlugExists)
	require.Nil(t, secondCategory)
}

func TestCreateCategory_DuplicateSlugWithCustomSlug(t *testing.T) {
	repo := newMockCategoryRepository()
	svc := NewService(repo)

	// Create first category with custom slug
	customSlug := "my-custom-slug"
	firstCategory, err := svc.Create(context.Background(), CreateInput{
		Name: "First Category",
		Slug: &customSlug,
	})
	require.NoError(t, err)
	require.Equal(t, customSlug, firstCategory.Slug)

	// Try to create second category with same custom slug
	secondCategory, err := svc.Create(context.Background(), CreateInput{
		Name: "Second Category",
		Slug: &customSlug,
	})

	require.ErrorIs(t, err, dom.ErrCategorySlugExists)
	require.Nil(t, secondCategory)
}

func TestGetCategory_NotFound(t *testing.T) {
	repo := newMockCategoryRepository()
	svc := NewService(repo)

	category, err := svc.GetByID(context.Background(), 999)

	require.ErrorIs(t, err, dom.ErrCategoryNotFound)
	require.Nil(t, category)
}

func TestGetCategory_Found(t *testing.T) {
	repo := newMockCategoryRepository()
	svc := NewService(repo)

	// Create a category first
	created, err := svc.Create(context.Background(), CreateInput{
		Name:        "Test Category",
		Description: "Test description",
	})
	require.NoError(t, err)
	require.NotNil(t, created)

	// Get it back
	category, err := svc.GetByID(context.Background(), created.ID)

	require.NoError(t, err)
	require.NotNil(t, category)
	require.Equal(t, created.ID, category.ID)
	require.Equal(t, "Test Category", category.Name)
	require.Equal(t, "test-category", category.Slug)
}

func TestUpdateCategory_NotFound(t *testing.T) {
	repo := newMockCategoryRepository()
	svc := NewService(repo)

	newName := "Updated Name"
	category, err := svc.Update(context.Background(), UpdateInput{
		ID:   999,
		Name: &newName,
	})

	require.ErrorIs(t, err, dom.ErrCategoryNotFound)
	require.Nil(t, category)
	require.Nil(t, repo.updated)
}

func TestUpdateCategory_DuplicateSlug(t *testing.T) {
	repo := newMockCategoryRepository()
	svc := NewService(repo)

	// Create first category
	firstCategory, err := svc.Create(context.Background(), CreateInput{
		Name: "First Category",
	})
	require.NoError(t, err)

	// Create second category
	secondCategory, err := svc.Create(context.Background(), CreateInput{
		Name: "Second Category",
	})
	require.NoError(t, err)

	// Try to update second category to use first category's slug
	duplicateSlug := firstCategory.Slug
	updated, err := svc.Update(context.Background(), UpdateInput{
		ID:   secondCategory.ID,
		Slug: &duplicateSlug,
	})

	require.ErrorIs(t, err, dom.ErrCategorySlugExists)
	require.Nil(t, updated)
}

func TestDeleteCategory_NotFound(t *testing.T) {
	repo := newMockCategoryRepository()
	svc := NewService(repo)

	err := svc.Delete(context.Background(), 999)

	require.ErrorIs(t, err, dom.ErrCategoryNotFound)
	require.Zero(t, repo.deletedID)
}

func TestDeleteCategory_Success(t *testing.T) {
	repo := newMockCategoryRepository()
	svc := NewService(repo)

	// Create a category first
	created, err := svc.Create(context.Background(), CreateInput{
		Name: "To Be Deleted",
	})
	require.NoError(t, err)
	require.NotNil(t, created)

	// Delete it
	err = svc.Delete(context.Background(), created.ID)

	require.NoError(t, err)
	require.Equal(t, created.ID, repo.deletedID)

	// Verify it's deleted
	_, err = svc.GetByID(context.Background(), created.ID)
	require.ErrorIs(t, err, dom.ErrCategoryNotFound)
}

func TestCreateCategory_SlugGeneration(t *testing.T) {
	tests := []struct {
		name        string
		inputName   string
		expectedSlug string
	}{
		{
			name:         "Simple name",
			inputName:    "Electronics",
			expectedSlug: "electronics",
		},
		{
			name:         "Name with spaces",
			inputName:    "Home & Garden",
			expectedSlug: "home-garden",
		},
		{
			name:         "Name with special characters",
			inputName:    "Men's Fashion!!!",
			expectedSlug: "men-s-fashion",
		},
		{
			name:         "Name with numbers",
			inputName:    "Category 123",
			expectedSlug: "category-123",
		},
		{
			name:         "Name with multiple spaces",
			inputName:    "  Electronics   &   Gadgets  ",
			expectedSlug: "electronics-gadgets",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockCategoryRepository()
			svc := NewService(repo)

			category, err := svc.Create(context.Background(), CreateInput{
				Name: tt.inputName,
			})

			require.NoError(t, err)
			require.Equal(t, tt.expectedSlug, category.Slug)
		})
	}
}

func TestListCategories_AllCategories(t *testing.T) {
	repo := newMockCategoryRepository()
	svc := NewService(repo)

	// Create multiple categories
	_, err := svc.Create(context.Background(), CreateInput{
		Name:     "Category 1",
		IsActive: boolPtr(true),
	})
	require.NoError(t, err)

	_, err = svc.Create(context.Background(), CreateInput{
		Name:     "Category 2",
		IsActive: boolPtr(false),
	})
	require.NoError(t, err)

	_, err = svc.Create(context.Background(), CreateInput{
		Name:     "Category 3",
		IsActive: boolPtr(true),
	})
	require.NoError(t, err)

	// List all
	categories, err := svc.List(context.Background(), dom.ListFilter{})

	require.NoError(t, err)
	require.Len(t, categories, 3)
}

func TestListCategories_OnlyActive(t *testing.T) {
	repo := newMockCategoryRepository()
	svc := NewService(repo)

	// Create active and inactive categories
	_, err := svc.Create(context.Background(), CreateInput{
		Name:     "Active Category",
		IsActive: boolPtr(true),
	})
	require.NoError(t, err)

	_, err = svc.Create(context.Background(), CreateInput{
		Name:     "Inactive Category",
		IsActive: boolPtr(false),
	})
	require.NoError(t, err)

	// List only active
	categories, err := svc.List(context.Background(), dom.ListFilter{
		OnlyActive: true,
	})

	require.NoError(t, err)
	require.Len(t, categories, 1)
	require.True(t, categories[0].IsActive)
	require.Equal(t, "Active Category", categories[0].Name)
}

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func boolPtr(b bool) *bool {
	return &b
}
