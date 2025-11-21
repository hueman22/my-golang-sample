package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domuser "example.com/my-golang-sample/app/internal/domain/user"
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

