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

// Mock user repository for auth tests
type mockAuthUserRepo struct {
	user    *domuser.User
	getErr  error
}

func (m *mockAuthUserRepo) Create(ctx context.Context, u *domuser.User) (*domuser.User, error) {
	return nil, nil
}

func (m *mockAuthUserRepo) GetByID(ctx context.Context, id int64) (*domuser.User, error) {
	return nil, nil
}

func (m *mockAuthUserRepo) GetByEmail(ctx context.Context, email string) (*domuser.User, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	if m.user != nil && m.user.Email == email {
		return m.user, nil
	}
	return nil, domuser.ErrUserNotFound
}

func (m *mockAuthUserRepo) List(ctx context.Context, filter domuser.ListUsersFilter) ([]*domuser.User, error) {
	return nil, nil
}

func (m *mockAuthUserRepo) Update(ctx context.Context, u *domuser.User) (*domuser.User, error) {
	return nil, nil
}

func (m *mockAuthUserRepo) Delete(ctx context.Context, id int64) error {
	return nil
}

func (m *mockAuthUserRepo) GetRoleIDByCode(ctx context.Context, code domuser.RoleCode) (int64, error) {
	return 0, nil
}

// Mock password checker
type mockPasswordChecker struct {
	shouldSucceed bool
}

func (m *mockPasswordChecker) Compare(hash, password string) error {
	if m.shouldSucceed {
		return nil
	}
	return domuser.ErrInvalidCredential
}

// Setup function that creates a real auth service with mocked dependencies
func setupLoginRouter(user *domuser.User, passwordValid bool) http.Handler {
	repo := &mockAuthUserRepo{user: user}
	checker := &mockPasswordChecker{shouldSucceed: passwordValid}
	tokenSvc := security.NewJWTService("test-secret", time.Hour)
	authSvc := authuc.NewService(repo, checker, tokenSvc)

	api := NewAPI(Dependencies{
		AuthService:  authSvc,
		TokenService: tokenSvc,
	})
	return api.Router()
}

func TestLogin_Success(t *testing.T) {
	user := &domuser.User{
		ID:           100,
		Name:         "Test User",
		Email:        "test@example.com",
		RoleCode:     domuser.RoleCodeCustomer,
		PasswordHash: "hashed_password",
	}
	router := setupLoginRouter(user, true) // password check succeeds

	body := map[string]string{
		"email":    "test@example.com",
		"password": "password123",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "should return 200 OK on success")

	var response map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err, "response should be valid JSON")

	// Verify token in response
	token, ok := response["token"].(string)
	require.True(t, ok, "token should be present in response")
	require.NotEmpty(t, token, "token should not be empty")

	// Verify user in response
	userObj, ok := response["user"].(map[string]any)
	require.True(t, ok, "user should be present in response")
	require.Equal(t, float64(100), userObj["id"])
	require.Equal(t, "test@example.com", userObj["email"])
}

func TestLogin_InvalidCredentials(t *testing.T) {
	user := &domuser.User{
		ID:           100,
		Email:        "test@example.com",
		PasswordHash: "hashed_password",
	}
	router := setupLoginRouter(user, false) // password check fails

	body := map[string]string{
		"email":    "test@example.com",
		"password": "wrong_password",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, "should return 401 on invalid credentials")
}

func TestLogin_UserNotFound(t *testing.T) {
	router := setupLoginRouter(nil, true) // no user in repo

	body := map[string]string{
		"email":    "notfound@example.com",
		"password": "password123",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, "should return 401 when user not found")
}

func TestLogin_InvalidBody(t *testing.T) {
	router := setupLoginRouter(nil, true)

	// Invalid JSON body
	payload := []byte(`{"email": "test@example.com", "password": invalid}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, "should return 400 on invalid JSON body")
}

func TestLogin_InvalidBody_MissingFields(t *testing.T) {
	router := setupLoginRouter(nil, true)

	// Missing required fields
	body := map[string]string{
		"email": "test@example.com",
		// password is missing
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, "should return 400 when required fields are missing")
}

func TestLogin_InvalidBody_InvalidEmailFormat(t *testing.T) {
	router := setupLoginRouter(nil, true)

	// Invalid email format
	body := map[string]string{
		"email":    "invalid-email",
		"password": "password123",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, "should return 400 when email format is invalid")
}

func TestLogin_InvalidBody_EmptyBody(t *testing.T) {
	router := setupLoginRouter(nil, true)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader([]byte("")))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code, "should return 400 on empty body")
}

