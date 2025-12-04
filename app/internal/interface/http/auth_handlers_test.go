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

	domuser "example.com/my-golang-sample/app/internal/domain/user"
	"example.com/my-golang-sample/app/internal/infra/security"
	authuc "example.com/my-golang-sample/app/internal/usecase/auth"
)

type fakeAuthUserRepo struct {
	user *domuser.User
}

func (f *fakeAuthUserRepo) Create(ctx context.Context, u *domuser.User) (*domuser.User, error) {
	return nil, nil
}

func (f *fakeAuthUserRepo) GetByID(ctx context.Context, id int64) (*domuser.User, error) {
	return nil, nil
}

func (f *fakeAuthUserRepo) GetByEmail(ctx context.Context, email string) (*domuser.User, error) {
	if f.user != nil && f.user.Email == email {
		return f.user, nil
	}
	return nil, domuser.ErrUserNotFound
}

func (f *fakeAuthUserRepo) List(ctx context.Context, filter domuser.ListUsersFilter) ([]*domuser.User, error) {
	return nil, nil
}

func (f *fakeAuthUserRepo) Update(ctx context.Context, u *domuser.User) (*domuser.User, error) {
	return nil, nil
}

func (f *fakeAuthUserRepo) Delete(ctx context.Context, id int64) error {
	return nil
}

func (f *fakeAuthUserRepo) GetRoleIDByCode(ctx context.Context, code domuser.RoleCode) (int64, error) {
	return 0, nil
}

type fakePasswordChecker struct {
	shouldSucceed bool
}

func (f *fakePasswordChecker) Compare(hash, password string) error {
	if f.shouldSucceed {
		return nil
	}
	return domuser.ErrInvalidCredential
}

func setupAuthAPI(user *domuser.User, passwordValid bool) *API {
	repo := &fakeAuthUserRepo{user: user}
	checker := &fakePasswordChecker{shouldSucceed: passwordValid}
	tokenSvc := security.NewJWTService("test-secret", time.Hour)
	authSvc := authuc.NewService(repo, checker, tokenSvc)

	return NewAPI(Dependencies{
		AuthService:  authSvc,
		TokenService: tokenSvc,
	})
}

func TestLogin_SuccessReturnsJWTWithUserIDAndRole(t *testing.T) {
	user := &domuser.User{
		ID:           100,
		Name:         "Test User",
		Email:        "test@example.com",
		RoleCode:     domuser.RoleCodeCustomer,
		PasswordHash: "hashed_password",
	}
	api := setupAuthAPI(user, true)
	router := api.Router()

	body := map[string]any{
		"email":    "test@example.com",
		"password": "password123",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

	// Verify token exists
	token, ok := response["token"].(string)
	require.True(t, ok, "token should be a string")
	require.NotEmpty(t, token, "token should not be empty")

	// Verify token contains user id and role
	tokenSvc := security.NewJWTService("test-secret", time.Hour)
	claims, err := tokenSvc.ParseToken(token)
	require.NoError(t, err)
	require.Equal(t, int64(100), claims.UserID, "token should contain user id")
	require.Equal(t, domuser.RoleCodeCustomer, claims.RoleCode, "token should contain role code")

	// Verify user object in response
	userObj, ok := response["user"].(map[string]any)
	require.True(t, ok, "user should be an object")
	require.Equal(t, float64(100), userObj["id"], "user id should match")
	require.Equal(t, "CUSTOMER", userObj["role_code"], "user role should match")
}

func TestLogin_InvalidCredentialsReturns401(t *testing.T) {
	user := &domuser.User{
		ID:           100,
		Email:        "test@example.com",
		RoleCode:     domuser.RoleCodeCustomer,
		PasswordHash: "hashed_password",
	}
	api := setupAuthAPI(user, false) // password check fails
	router := api.Router()

	body := map[string]any{
		"email":    "test@example.com",
		"password": "wrong_password",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}

func TestLogin_UserNotFoundReturns401(t *testing.T) {
	api := setupAuthAPI(nil, true) // no user in repo
	router := api.Router()

	body := map[string]any{
		"email":    "notfound@example.com",
		"password": "password123",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, rec.Body.String())
}

func TestLogin_InvalidEmailFormatReturns400(t *testing.T) {
	api := setupAuthAPI(nil, true)
	router := api.Router()

	body := map[string]any{
		"email":    "invalid-email",
		"password": "password123",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestLogin_MissingPasswordReturns400(t *testing.T) {
	api := setupAuthAPI(nil, true)
	router := api.Router()

	body := map[string]any{
		"email": "test@example.com",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
}

func TestLogin_AdminUserReturnsAdminRoleInToken(t *testing.T) {
	user := &domuser.User{
		ID:           200,
		Name:         "Admin User",
		Email:        "admin@example.com",
		RoleCode:     domuser.RoleCodeAdmin,
		PasswordHash: "hashed_password",
	}
	api := setupAuthAPI(user, true)
	router := api.Router()

	body := map[string]any{
		"email":    "admin@example.com",
		"password": "password123",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	var response map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))

	token := response["token"].(string)
	tokenSvc := security.NewJWTService("test-secret", time.Hour)
	claims, err := tokenSvc.ParseToken(token)
	require.NoError(t, err)
	require.Equal(t, int64(200), claims.UserID)
	require.Equal(t, domuser.RoleCodeAdmin, claims.RoleCode, "token should contain ADMIN role")
}


