package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	domcategory "example.com/my-golang-sample/app/internal/domain/category"
	domuser "example.com/my-golang-sample/app/internal/domain/user"
	"example.com/my-golang-sample/app/internal/infra/security"
	categoryuc "example.com/my-golang-sample/app/internal/usecase/category"
)

type memoryCategoryRepo struct {
	nextID int64
	items  map[int64]*domcategory.Category
}

func newMemoryCategoryRepo() *memoryCategoryRepo {
	return &memoryCategoryRepo{
		nextID: 0,
		items:  map[int64]*domcategory.Category{},
	}
}

func (m *memoryCategoryRepo) clone(c *domcategory.Category) *domcategory.Category {
	if c == nil {
		return nil
	}
	cp := *c
	return &cp
}

func (m *memoryCategoryRepo) slugExists(slug string, ignoreID int64) bool {
	for id, item := range m.items {
		if id == ignoreID {
			continue
		}
		if item.Slug == slug {
			return true
		}
	}
	return false
}

func (m *memoryCategoryRepo) Create(ctx context.Context, c *domcategory.Category) (*domcategory.Category, error) {
	if m.slugExists(c.Slug, 0) {
		return nil, domcategory.ErrCategorySlugExists
	}
	m.nextID++
	c.ID = m.nextID
	m.items[c.ID] = m.clone(c)
	return m.clone(c), nil
}

func (m *memoryCategoryRepo) Update(ctx context.Context, c *domcategory.Category) (*domcategory.Category, error) {
	if _, ok := m.items[c.ID]; !ok {
		return nil, domcategory.ErrCategoryNotFound
	}
	if m.slugExists(c.Slug, c.ID) {
		return nil, domcategory.ErrCategorySlugExists
	}
	m.items[c.ID] = m.clone(c)
	return m.clone(c), nil
}

func (m *memoryCategoryRepo) Delete(ctx context.Context, id int64) error {
	if _, ok := m.items[id]; !ok {
		return domcategory.ErrCategoryNotFound
	}
	delete(m.items, id)
	return nil
}

func (m *memoryCategoryRepo) GetByID(ctx context.Context, id int64) (*domcategory.Category, error) {
	if c, ok := m.items[id]; ok {
		return m.clone(c), nil
	}
	return nil, domcategory.ErrCategoryNotFound
}

func (m *memoryCategoryRepo) List(ctx context.Context, filter domcategory.ListFilter) ([]*domcategory.Category, error) {
	result := make([]*domcategory.Category, 0, len(m.items))
	for _, c := range m.items {
		if filter.OnlyActive && !c.IsActive {
			continue
		}
		result = append(result, m.clone(c))
	}
	return result, nil
}

func setupCategoryAPI(repo domcategory.Repository) (*API, string) {
	categorySvc := categoryuc.NewService(repo)
	tokenSvc := security.NewJWTService("cat-secret", time.Hour)

	api := NewAPI(Dependencies{
		CategoryService: categorySvc,
		TokenService:    tokenSvc,
	})

	token, _ := tokenSvc.GenerateToken(&domuser.User{
		ID:       1,
		Name:     "Root",
		Email:    "root@example.com",
		RoleCode: domuser.RoleCodeSuperAdmin,
	})

	return api, token
}

func TestAdminCategoryCRUDFlow(t *testing.T) {
	repo := newMemoryCategoryRepo()
	api, token := setupCategoryAPI(repo)
	router := api.Router()

	// Create category without custom slug -> service should generate it.
	body := map[string]any{
		"name":        "Electronics & Gadgets",
		"description": "Devices",
		"is_active":   true,
	}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/categories", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &created)
	id := int(created["id"].(float64))
	if created["slug"].(string) != "electronics-gadgets" {
		t.Fatalf("expected slug electronics-gadgets, got %s", created["slug"])
	}

	// List all
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/categories", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list expected 200, got %d", rec.Code)
	}

	// List only active
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/categories?only_active=true", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("only_active expected 200, got %d", rec.Code)
	}

	// Get detail
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/categories/"+strconv.Itoa(id), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get expected 200, got %d", rec.Code)
	}

	// Update category with custom slug and deactivate
	update := map[string]any{
		"name":        "Electronics Updated",
		"slug":        "updated-electronics",
		"description": "New desc",
		"is_active":   false,
	}
	updatePayload, _ := json.Marshal(update)
	req = httptest.NewRequest(http.MethodPut, "/api/v1/admin/categories/"+strconv.Itoa(id), bytes.NewReader(updatePayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Delete category
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/categories/"+strconv.Itoa(id), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete expected 204, got %d", rec.Code)
	}

	// Ensure deleted
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/categories/"+strconv.Itoa(id), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", rec.Code)
	}
}

func TestAdminCategory_CreateDuplicateSlugReturns409(t *testing.T) {
	repo := newMemoryCategoryRepo()
	api, token := setupCategoryAPI(repo)
	router := api.Router()

	body := map[string]any{
		"name":        "Books",
		"slug":        "books",
		"description": "All books",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/categories", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first create expected 201, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/categories", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate expected 409, got %d", rec.Code)
	}
}
