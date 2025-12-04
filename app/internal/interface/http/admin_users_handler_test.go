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

	domuser "example.com/my-golang-sample/app/internal/domain/user"
	domrole "example.com/my-golang-sample/app/internal/domain/userrole"
	"example.com/my-golang-sample/app/internal/infra/security"
	authuc "example.com/my-golang-sample/app/internal/usecase/auth"
	useruc "example.com/my-golang-sample/app/internal/usecase/user"
)

type fakeUserRepo struct {
	created          bool
	roleLookupCalled bool
}

func (f *fakeUserRepo) Create(ctx context.Context, u *domuser.User) (*domuser.User, error) {
	f.created = true
	return u, nil
}

func (f *fakeUserRepo) GetByID(ctx context.Context, id int64) (*domuser.User, error) {
	return nil, domuser.ErrUserNotFound
}

func (f *fakeUserRepo) GetByEmail(ctx context.Context, email string) (*domuser.User, error) {
	return nil, domuser.ErrUserNotFound
}

func (f *fakeUserRepo) List(ctx context.Context, filter domuser.ListUsersFilter) ([]*domuser.User, error) {
	return nil, nil
}

func (f *fakeUserRepo) Update(ctx context.Context, u *domuser.User) (*domuser.User, error) {
	return u, nil
}

func (f *fakeUserRepo) Delete(ctx context.Context, id int64) error {
	return nil
}

func (f *fakeUserRepo) GetRoleIDByCode(ctx context.Context, code domuser.RoleCode) (int64, error) {
	f.roleLookupCalled = true
	return 2, nil
}

type fakePasswordService struct{}

func (fakePasswordService) Hash(password string) (string, error) {
	return "hash:" + password, nil
}

func (fakePasswordService) Compare(hash, password string) error {
	return nil
}

func TestAdminCreateAdminRoleReturns422(t *testing.T) {
	repo := &fakeUserRepo{}
	passwordSvc := fakePasswordService{}
	userSvc := useruc.NewService(repo, passwordSvc)

	tokenSvc := security.NewJWTService("test-secret", time.Hour)
	authSvc := authuc.NewService(repo, passwordSvc, tokenSvc)

	api := NewAPI(Dependencies{
		AuthService:     authSvc,
		UserService:     userSvc,
		UserRoleService: nil,
		CategoryService: nil,
		ProductService:  nil,
		CartService:     nil,
		OrderService:    nil,
		TokenService:    tokenSvc,
	})

	token, err := tokenSvc.GenerateToken(&domuser.User{
		ID:       1,
		Name:     "Admin",
		Email:    "admin@example.com",
		RoleCode: domuser.RoleCodeAdmin,
	})
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	body := map[string]any{
		"name":      "New Admin",
		"email":     "newadmin@example.com",
		"password":  "StrongPass1",
		"role_code": "ADMIN",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rec := httptest.NewRecorder()
	api.Router().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d: %s", rec.Code, rec.Body.String())
	}
	if repo.created {
		t.Fatal("repository Create should not be called for forbidden role assignment")
	}
	if repo.roleLookupCalled {
		t.Fatal("role lookup should not be called for forbidden role assignment")
	}
}

type memoryUserRepo struct {
	nextID     int64
	users      map[int64]*domuser.User
	emailIndex map[string]int64
	roleIDs    map[domuser.RoleCode]int64
}

func newMemoryUserRepo() *memoryUserRepo {
	return &memoryUserRepo{
		nextID:     1,
		users:      map[int64]*domuser.User{},
		emailIndex: map[string]int64{},
		roleIDs: map[domuser.RoleCode]int64{
			domuser.RoleCodeSuperAdmin: 1,
			domuser.RoleCodeAdmin:      2,
			domuser.RoleCodeCustomer:   3,
		},
	}
}

func (m *memoryUserRepo) clone(u *domuser.User) *domuser.User {
	if u == nil {
		return nil
	}
	c := *u
	return &c
}

func (m *memoryUserRepo) Create(ctx context.Context, u *domuser.User) (*domuser.User, error) {
	if _, exists := m.emailIndex[u.Email]; exists {
		return nil, domuser.ErrEmailAlreadyUsed
	}
	u.ID = m.nextID
	m.nextID++
	m.users[u.ID] = m.clone(u)
	m.emailIndex[u.Email] = u.ID
	return m.clone(u), nil
}

func (m *memoryUserRepo) GetByID(ctx context.Context, id int64) (*domuser.User, error) {
	if user, ok := m.users[id]; ok {
		return m.clone(user), nil
	}
	return nil, domuser.ErrUserNotFound
}

func (m *memoryUserRepo) GetByEmail(ctx context.Context, email string) (*domuser.User, error) {
	if id, ok := m.emailIndex[email]; ok {
		return m.clone(m.users[id]), nil
	}
	return nil, domuser.ErrUserNotFound
}

func (m *memoryUserRepo) List(ctx context.Context, filter domuser.ListUsersFilter) ([]*domuser.User, error) {
	var result []*domuser.User
	for _, user := range m.users {
		if filter.RoleCode != nil && user.RoleCode != *filter.RoleCode {
			continue
		}
		result = append(result, m.clone(user))
	}
	return result, nil
}

func (m *memoryUserRepo) Update(ctx context.Context, u *domuser.User) (*domuser.User, error) {
	existing, ok := m.users[u.ID]
	if !ok {
		return nil, domuser.ErrUserNotFound
	}
	if u.Email != existing.Email {
		if _, exists := m.emailIndex[u.Email]; exists {
			return nil, domuser.ErrEmailAlreadyUsed
		}
		delete(m.emailIndex, existing.Email)
		m.emailIndex[u.Email] = u.ID
	}
	m.users[u.ID] = m.clone(u)
	return m.clone(u), nil
}

func (m *memoryUserRepo) Delete(ctx context.Context, id int64) error {
	if user, ok := m.users[id]; ok {
		delete(m.emailIndex, user.Email)
		delete(m.users, id)
		return nil
	}
	return domuser.ErrUserNotFound
}

func (m *memoryUserRepo) GetRoleIDByCode(ctx context.Context, code domuser.RoleCode) (int64, error) {
	if id, ok := m.roleIDs[code]; ok {
		return id, nil
	}
	return 0, fmt.Errorf("role not found: %w", domrole.ErrRoleNotFound)
}

func setupUserAPI(repo domuser.Repository) (*API, string) {
	passwordSvc := fakePasswordService{}
	userSvc := useruc.NewService(repo, passwordSvc)

	tokenSvc := security.NewJWTService("test-secret", time.Hour)
	authSvc := authuc.NewService(repo, passwordSvc, tokenSvc)

	api := NewAPI(Dependencies{
		AuthService:  authSvc,
		UserService:  userSvc,
		TokenService: tokenSvc,
	})

	token, _ := tokenSvc.GenerateToken(&domuser.User{
		ID:       1,
		Name:     "Root",
		Email:    "root@example.com",
		RoleCode: domuser.RoleCodeSuperAdmin,
	})

	return api, token
}

func TestAdminUserCRUDFlow(t *testing.T) {
	repo := newMemoryUserRepo()
	api, token := setupUserAPI(repo)
	router := api.Router()

	// Create customer user
	body := map[string]any{
		"name":      "Alice",
		"email":     "alice@example.com",
		"password":  "Secret123!",
		"role_code": "CUSTOMER",
	}
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var created map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	id := int(created["id"].(float64))

	// List all
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	var listResp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &listResp)
	data := listResp["data"].([]any)
	if len(data) != 1 {
		t.Fatalf("expected 1 user, got %d", len(data))
	}

	// List filtered by role_code
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?role_code=CUSTOMER", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("filter expected 200, got %d", rec.Code)
	}

	// Get detail
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/"+fmt.Sprint(id), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get expected 200, got %d", rec.Code)
	}

	// Update user (promote to admin)
	update := map[string]any{
		"name":      "Alice Admin",
		"role_code": "ADMIN",
	}
	updatePayload, _ := json.Marshal(update)
	req = httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/"+fmt.Sprint(id), bytes.NewReader(updatePayload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("update expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	// Delete user
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/"+fmt.Sprint(id), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete expected 204, got %d", rec.Code)
	}

	// Verify deleted
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/"+fmt.Sprint(id), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get deleted expected 404, got %d", rec.Code)
	}
}

func TestAdminUser_CreateDuplicateEmailReturns409(t *testing.T) {
	repo := newMemoryUserRepo()
	api, token := setupUserAPI(repo)
	router := api.Router()

	payload := map[string]any{
		"name":      "Bob",
		"email":     "dup@example.com",
		"password":  "Secret123!",
		"role_code": "CUSTOMER",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate expected 409, got %d", rec.Code)
	}
}

func TestAdminUser_CreateWithUnknownRoleReturns404(t *testing.T) {
	repo := newMemoryUserRepo()
	api, token := setupUserAPI(repo)
	router := api.Router()

	payload := map[string]any{
		"name":      "Charlie",
		"email":     "charlie@example.com",
		"password":  "Secret123!",
		"role_code": "SUPPORT_AGENT",
	}
	body, _ := json.Marshal(payload)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing role, got %d: %s", rec.Code, rec.Body.String())
	}
}
