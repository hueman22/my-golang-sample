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

	domuser "example.com/my-golang-sample/app/internal/domain/user"
	"example.com/my-golang-sample/app/internal/infra/security"
	authuc "example.com/my-golang-sample/app/internal/usecase/auth"
	useruc "example.com/my-golang-sample/app/internal/usecase/user"
)

// setupUserAPIWithRole creates an API instance with a user service and returns a token for the specified role
// Reuses memoryUserRepo and fakePasswordService from admin_users_handler_test.go
func setupUserAPIWithRole(repo domuser.Repository, role domuser.RoleCode) (*API, string) {
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
		Name:     "Test Admin",
		Email:    "admin@example.com",
		RoleCode: role,
	})

	return api, token
}

// Test 1: List users as SUPER_ADMIN
func TestListUsers_AsSuperAdmin_Returns200(t *testing.T) {
	repo := newMemoryUserRepo()
	// Pre-populate with some users
	repo.Create(context.Background(), &domuser.User{
		Name:         "User 1",
		Email:        "user1@example.com",
		PasswordHash: "hash1",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	})
	repo.Create(context.Background(), &domuser.User{
		Name:         "User 2",
		Email:        "user2@example.com",
		PasswordHash: "hash2",
		UserRoleID:   2,
		RoleCode:     domuser.RoleCodeAdmin,
	})

	api, token := setupUserAPIWithRole(repo, domuser.RoleCodeSuperAdmin)
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "should return 200 OK")
	
	var response map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err, "response should be valid JSON")
	
	data, ok := response["data"].([]any)
	require.True(t, ok, "response should have 'data' field")
	require.GreaterOrEqual(t, len(data), 2, "should return at least 2 users")
}

// Test 2: Create CUSTOMER user as SUPER_ADMIN
func TestCreateUser_CustomerAsSuperAdmin_Returns201(t *testing.T) {
	repo := newMemoryUserRepo()
	api, token := setupUserAPIWithRole(repo, domuser.RoleCodeSuperAdmin)
	router := api.Router()

	body := map[string]any{
		"name":      "John Customer",
		"email":     "john@example.com",
		"password":  "password123",
		"role_code": "CUSTOMER",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, "should return 201 Created")

	var user map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &user)
	require.NoError(t, err, "response should be valid JSON")
	require.Equal(t, "John Customer", user["name"])
	require.Equal(t, "john@example.com", user["email"])
	require.Equal(t, "CUSTOMER", user["role_code"])
	require.NotZero(t, user["id"], "user should have an ID")
}

// Test 3: Create ADMIN user as SUPER_ADMIN (allowed)
func TestCreateUser_AdminAsSuperAdmin_Returns201(t *testing.T) {
	repo := newMemoryUserRepo()
	api, token := setupUserAPIWithRole(repo, domuser.RoleCodeSuperAdmin)
	router := api.Router()

	body := map[string]any{
		"name":      "New Admin",
		"email":     "newadmin@example.com",
		"password":  "password123",
		"role_code": "ADMIN",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, "should return 201 Created")

	var user map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &user)
	require.NoError(t, err, "response should be valid JSON")
	require.Equal(t, "ADMIN", user["role_code"], "role_code should be stored as ADMIN")
	require.Equal(t, "newadmin@example.com", user["email"])
}

// Test 4: Create ADMIN user as ADMIN (NOT allowed)
func TestCreateUser_AdminAsAdmin_Returns422(t *testing.T) {
	repo := newMemoryUserRepo()
	api, token := setupUserAPIWithRole(repo, domuser.RoleCodeAdmin) // ADMIN, not SUPER_ADMIN
	router := api.Router()

	body := map[string]any{
		"name":      "Another Admin",
		"email":     "anotheradmin@example.com",
		"password":  "password123",
		"role_code": "ADMIN",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, "should return 422 Unprocessable Entity")

	// Verify user was NOT created
	users, _ := repo.List(context.Background(), domuser.ListUsersFilter{})
	require.Len(t, users, 0, "user should NOT be created")
}

// Test 5: Get user not found
func TestGetUser_NotFound_Returns404(t *testing.T) {
	repo := newMemoryUserRepo()
	api, token := setupUserAPIWithRole(repo, domuser.RoleCodeSuperAdmin)
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/999", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNotFound, rec.Code, "should return 404 Not Found")
}

// Test 6: Delete user successfully
func TestDeleteUser_Success_Returns204(t *testing.T) {
	repo := newMemoryUserRepo()
	// Create a user first
	created, _ := repo.Create(context.Background(), &domuser.User{
		Name:         "To Delete",
		Email:        "delete@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	})

	api, token := setupUserAPIWithRole(repo, domuser.RoleCodeSuperAdmin)
	router := api.Router()

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/admin/users/%d", created.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusNoContent, rec.Code, "should return 204 No Content")

	// Verify user was deleted
	_, err := repo.GetByID(context.Background(), created.ID)
	require.ErrorIs(t, err, domuser.ErrUserNotFound, "user should be deleted")
}

// Additional test cases for better coverage

func TestListUsers_AsAdmin_Returns200(t *testing.T) {
	repo := newMemoryUserRepo()
	repo.Create(context.Background(), &domuser.User{
		Name:         "Customer",
		Email:        "customer@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	})

	api, token := setupUserAPIWithRole(repo, domuser.RoleCodeAdmin)
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
}

func TestGetUser_Found_Returns200(t *testing.T) {
	repo := newMemoryUserRepo()
	created, _ := repo.Create(context.Background(), &domuser.User{
		Name:         "Test User",
		Email:        "test@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	})

	api, token := setupUserAPIWithRole(repo, domuser.RoleCodeSuperAdmin)
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/admin/users/%d", created.ID), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	
	var user map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &user)
	require.NoError(t, err)
	require.Equal(t, float64(created.ID), user["id"])
	require.Equal(t, "Test User", user["name"])
}

func TestCreateUser_InvalidRoleCode_ReturnsError(t *testing.T) {
	repo := newMemoryUserRepo()
	api, token := setupUserAPIWithRole(repo, domuser.RoleCodeSuperAdmin)
	router := api.Router()

	body := map[string]any{
		"name":      "Test",
		"email":     "test@example.com",
		"password":  "password123",
		"role_code": "INVALID_ROLE",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.NotEqual(t, http.StatusCreated, rec.Code, "should not return 201")
	require.GreaterOrEqual(t, rec.Code, 400, "should return 4xx error")
}

func TestCreateUser_DuplicateEmail_Returns409(t *testing.T) {
	repo := newMemoryUserRepo()
	repo.Create(context.Background(), &domuser.User{
		Name:         "Existing",
		Email:        "existing@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	})

	api, token := setupUserAPIWithRole(repo, domuser.RoleCodeSuperAdmin)
	router := api.Router()

	body := map[string]any{
		"name":      "Duplicate",
		"email":     "existing@example.com",
		"password":  "password123",
		"role_code": "CUSTOMER",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusConflict, rec.Code, "should return 409 Conflict")
}

func TestUpdateUser_AdminCannotPromoteToAdmin_Returns422(t *testing.T) {
	repo := newMemoryUserRepo()
	// Create a customer user
	created, _ := repo.Create(context.Background(), &domuser.User{
		Name:         "Customer",
		Email:        "customer@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	})

	api, token := setupUserAPIWithRole(repo, domuser.RoleCodeAdmin) // ADMIN actor
	router := api.Router()

	body := map[string]any{
		"role_code": "ADMIN",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", created.ID), bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, "should return 422")
}

func TestUpdateUser_SuperAdminCanPromoteToAdmin_Returns200(t *testing.T) {
	repo := newMemoryUserRepo()
	// Create a customer user
	created, _ := repo.Create(context.Background(), &domuser.User{
		Name:         "Customer",
		Email:        "customer@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	})

	api, token := setupUserAPIWithRole(repo, domuser.RoleCodeSuperAdmin)
	router := api.Router()

	body := map[string]any{
		"role_code": "ADMIN",
	}
	payload, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", created.ID), bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, "should return 200 OK")
	
	var user map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &user)
	require.NoError(t, err)
	require.Equal(t, "ADMIN", user["role_code"], "role should be updated to ADMIN")
}

func TestListUsers_WithRoleFilter_ReturnsFiltered(t *testing.T) {
	repo := newMemoryUserRepo()
	repo.Create(context.Background(), &domuser.User{
		Name:         "Customer 1",
		Email:        "customer1@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	})
	repo.Create(context.Background(), &domuser.User{
		Name:         "Admin 1",
		Email:        "admin1@example.com",
		PasswordHash: "hash",
		UserRoleID:   2,
		RoleCode:     domuser.RoleCodeAdmin,
	})

	api, token := setupUserAPIWithRole(repo, domuser.RoleCodeSuperAdmin)
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users?role_code=CUSTOMER", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	
	var response map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	
	data := response["data"].([]any)
	require.Len(t, data, 1, "should return only CUSTOMER users")
	
	user := data[0].(map[string]any)
	require.Equal(t, "CUSTOMER", user["role_code"])
}

func TestAdminUsers_Unauthenticated_Returns401(t *testing.T) {
	repo := newMemoryUserRepo()
	api, _ := setupUserAPIWithRole(repo, domuser.RoleCodeSuperAdmin)
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	// No Authorization header
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusUnauthorized, rec.Code, "should return 401 Unauthorized")
}

func TestAdminUsers_CustomerRole_Returns403(t *testing.T) {
	repo := newMemoryUserRepo()
	api, token := setupUserAPIWithRole(repo, domuser.RoleCodeCustomer) // CUSTOMER role
	router := api.Router()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusForbidden, rec.Code, "should return 403 Forbidden")
}

