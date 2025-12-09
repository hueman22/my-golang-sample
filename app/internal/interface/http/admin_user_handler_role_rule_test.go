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

// minimalUserRepo is a minimal fake repository for testing
// The business rule is enforced by the usecase service, not the repository
type minimalUserRepo struct {
	users  map[int64]*domuser.User
	nextID int64
}

func newMinimalUserRepo() *minimalUserRepo {
	return &minimalUserRepo{
		users:  make(map[int64]*domuser.User),
		nextID: 1,
	}
}

func (m *minimalUserRepo) Create(ctx context.Context, u *domuser.User) (*domuser.User, error) {
	u.ID = m.nextID
	m.nextID++
	m.users[u.ID] = &domuser.User{
		ID:           u.ID,
		Name:         u.Name,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		UserRoleID:   u.UserRoleID,
		RoleCode:     u.RoleCode,
	}
	return m.users[u.ID], nil
}

func (m *minimalUserRepo) GetByID(ctx context.Context, id int64) (*domuser.User, error) {
	user, ok := m.users[id]
	if !ok {
		return nil, domuser.ErrUserNotFound
	}
	// Return a copy to avoid mutations
	return &domuser.User{
		ID:           user.ID,
		Name:         user.Name,
		Email:        user.Email,
		PasswordHash: user.PasswordHash,
		UserRoleID:   user.UserRoleID,
		RoleCode:     user.RoleCode,
	}, nil
}

func (m *minimalUserRepo) GetByEmail(ctx context.Context, email string) (*domuser.User, error) {
	for _, u := range m.users {
		if u.Email == email {
			return &domuser.User{
				ID:           u.ID,
				Name:         u.Name,
				Email:        u.Email,
				PasswordHash: u.PasswordHash,
				UserRoleID:   u.UserRoleID,
				RoleCode:     u.RoleCode,
			}, nil
		}
	}
	return nil, domuser.ErrUserNotFound
}

func (m *minimalUserRepo) List(ctx context.Context, filter domuser.ListUsersFilter) ([]*domuser.User, error) {
	users := make([]*domuser.User, 0, len(m.users))
	for _, u := range m.users {
		users = append(users, &domuser.User{
			ID:           u.ID,
			Name:         u.Name,
			Email:        u.Email,
			PasswordHash: u.PasswordHash,
			UserRoleID:   u.UserRoleID,
			RoleCode:     u.RoleCode,
		})
	}
	return users, nil
}

func (m *minimalUserRepo) Update(ctx context.Context, u *domuser.User) (*domuser.User, error) {
	existing, ok := m.users[u.ID]
	if !ok {
		return nil, domuser.ErrUserNotFound
	}
	existing.Name = u.Name
	existing.Email = u.Email
	existing.PasswordHash = u.PasswordHash
	existing.UserRoleID = u.UserRoleID
	existing.RoleCode = u.RoleCode
	return &domuser.User{
		ID:           existing.ID,
		Name:         existing.Name,
		Email:        existing.Email,
		PasswordHash: existing.PasswordHash,
		UserRoleID:   existing.UserRoleID,
		RoleCode:     existing.RoleCode,
	}, nil
}

func (m *minimalUserRepo) Delete(ctx context.Context, id int64) error {
	delete(m.users, id)
	return nil
}

func (m *minimalUserRepo) GetRoleIDByCode(ctx context.Context, code domuser.RoleCode) (int64, error) {
	switch code {
	case domuser.RoleCodeSuperAdmin:
		return 1, nil
	case domuser.RoleCodeAdmin:
		return 2, nil
	case domuser.RoleCodeCustomer:
		return 3, nil
	default:
		return 0, domuser.ErrInvalidRoleCode
	}
}

// setupAPIWithRole creates an API instance with a real user service (which enforces the business rule)
// and returns a JWT token for the specified role
func setupAPIWithRole(role domuser.RoleCode) (*API, *minimalUserRepo, string) {
	// Create a minimal fake user repo
	repo := newMinimalUserRepo()
	
	// Create password service
	passwordSvc := fakePasswordService{}
	
	// Create real user service (it will enforce the business rule)
	userSvc := useruc.NewService(repo, passwordSvc)
	
	// Create token service
	tokenSvc := security.NewJWTService("test-secret", time.Hour)
	
	// Create minimal auth service (not really used in these tests)
	fakeAuthRepo := &fakeAuthUserRepo{}
	authSvc := authuc.NewService(fakeAuthRepo, passwordSvc, tokenSvc)

	api := NewAPI(Dependencies{
		AuthService: authSvc,
		UserService: userSvc,
		TokenService: tokenSvc,
	})

	// Generate token for the specified role
	token, _ := tokenSvc.GenerateToken(&domuser.User{
		ID:       1,
		Name:     "Test User",
		Email:    "test@example.com",
		RoleCode: role,
	})

	return api, repo, token
}

// TestAdminCreateUser_AsAdmin_CannotCreateAdmin_Returns422 verifies that an ADMIN
// user cannot create another user with ADMIN role.
func TestAdminCreateUser_AsAdmin_CannotCreateAdmin_Returns422(t *testing.T) {
	// Setup: Create API with ADMIN role
	api, _, token := setupAPIWithRole(domuser.RoleCodeAdmin)
	router := api.Router()

	// Request body: trying to create ADMIN user
	body := map[string]any{
		"name":      "New Admin",
		"email":     "newadmin@example.com",
		"password":  "123456",
		"role_code": "ADMIN",
	}
	payload, _ := json.Marshal(body)

	// Execute: POST /api/v1/admin/users
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	// Assert: Should return 422 Unprocessable Entity
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, "should return 422 Unprocessable Entity")

	// Assert: Response should contain error JSON
	var errorResp map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &errorResp)
	require.NoError(t, err, "response should be valid JSON")
	require.Contains(t, errorResp, "error", "response should have 'error' field")
	require.Equal(t, "cannot assign role", errorResp["error"], "error message should match")
}

// TestAdminUpdateUser_AsAdmin_CannotPromoteToAdmin_Returns422 verifies that an ADMIN
// user cannot update another user and change their role to ADMIN.
func TestAdminUpdateUser_AsAdmin_CannotPromoteToAdmin_Returns422(t *testing.T) {
	// Setup: Create a shared repository
	repo := newMinimalUserRepo()
	
	// Create password service
	passwordSvc := fakePasswordService{}
	
	// Create user service with shared repo
	userSvc := useruc.NewService(repo, passwordSvc)
	
	// Create token service
	tokenSvc := security.NewJWTService("test-secret", time.Hour)
	
	// Create minimal auth service
	fakeAuthRepo := &fakeAuthUserRepo{}
	authSvc := authuc.NewService(fakeAuthRepo, passwordSvc, tokenSvc)

	// Create SUPER_ADMIN API to pre-create user
	superAdminAPI := NewAPI(Dependencies{
		AuthService: authSvc,
		UserService: userSvc,
		TokenService: tokenSvc,
	})
	superAdminToken, _ := tokenSvc.GenerateToken(&domuser.User{
		ID:       1,
		Name:     "Super Admin",
		Email:    "superadmin@example.com",
		RoleCode: domuser.RoleCodeSuperAdmin,
	})

	// Pre-create a user with CUSTOMER role using SUPER_ADMIN executor (to bypass rule)
	createBody := map[string]any{
		"name":      "Existing User",
		"email":     "user@example.com",
		"password":  "password123",
		"role_code": "CUSTOMER",
	}
	createPayload, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(createPayload))
	createReq.Header.Set("Authorization", "Bearer "+superAdminToken)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	superAdminAPI.Router().ServeHTTP(createRec, createReq)
	
	var createdUser map[string]any
	json.Unmarshal(createRec.Body.Bytes(), &createdUser)
	existingUserID := int64(createdUser["id"].(float64))

	// Now create ADMIN API with the same repository
	adminAPI := NewAPI(Dependencies{
		AuthService: authSvc,
		UserService: userSvc,
		TokenService: tokenSvc,
	})
	adminToken, _ := tokenSvc.GenerateToken(&domuser.User{
		ID:       2,
		Name:     "Admin",
		Email:    "admin@example.com",
		RoleCode: domuser.RoleCodeAdmin,
	})
	router := adminAPI.Router()
	token := adminToken

	// Request body: trying to update user to ADMIN role
	updateBody := map[string]any{
		"role_code": "ADMIN",
	}
	updatePayload, _ := json.Marshal(updateBody)

	// Execute: PUT /api/v1/admin/users/{id}
	updateReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", existingUserID), bytes.NewReader(updatePayload))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()

	router.ServeHTTP(updateRec, updateReq)

	// Assert: Should return 422 Unprocessable Entity
	require.Equal(t, http.StatusUnprocessableEntity, updateRec.Code, "should return 422 Unprocessable Entity")

	// Assert: Response should contain error JSON
	var errorResp map[string]any
	err := json.Unmarshal(updateRec.Body.Bytes(), &errorResp)
	require.NoError(t, err, "response should be valid JSON")
	require.Contains(t, errorResp, "error", "response should have 'error' field")
	require.Equal(t, "cannot assign role", errorResp["error"], "error message should match")
}

// TestAdminCreateUser_AsSuperAdmin_CanCreateAdmin_Returns201 verifies that a SUPER_ADMIN
// user can successfully create a user with ADMIN role.
func TestAdminCreateUser_AsSuperAdmin_CanCreateAdmin_Returns201(t *testing.T) {
	// Setup: Create API with SUPER_ADMIN role
	api, _, token := setupAPIWithRole(domuser.RoleCodeSuperAdmin)
	router := api.Router()

	// Request body: create ADMIN user (allowed for SUPER_ADMIN)
	body := map[string]any{
		"name":      "New Admin",
		"email":     "newadmin@example.com",
		"password":  "123456",
		"role_code": "ADMIN",
	}
	payload, _ := json.Marshal(body)

	// Execute: POST /api/v1/admin/users
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(payload))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	// Assert: Should return 201 Created
	require.Equal(t, http.StatusCreated, rec.Code, "should return 201 Created")

	// Assert: Response should contain created user JSON
	var user map[string]any
	err := json.Unmarshal(rec.Body.Bytes(), &user)
	require.NoError(t, err, "response should be valid JSON")
	require.Equal(t, "New Admin", user["name"], "user name should match")
	require.Equal(t, "newadmin@example.com", user["email"], "user email should match")
	require.Equal(t, "ADMIN", user["role_code"], "user role_code should be ADMIN")
	require.NotZero(t, user["id"], "user should have an ID")
}

// TestAdminUpdateUser_AsSuperAdmin_CanPromoteToAdmin_Returns200 verifies that a SUPER_ADMIN
// user can successfully update a user and change their role to ADMIN.
func TestAdminUpdateUser_AsSuperAdmin_CanPromoteToAdmin_Returns200(t *testing.T) {
	// Setup: Create API with SUPER_ADMIN role
	api, _, token := setupAPIWithRole(domuser.RoleCodeSuperAdmin)
	router := api.Router()

	// Pre-create a user with CUSTOMER role
	createBody := map[string]any{
		"name":      "Existing User",
		"email":     "user@example.com",
		"password":  "password123",
		"role_code": "CUSTOMER",
	}
	createPayload, _ := json.Marshal(createBody)
	createReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users", bytes.NewReader(createPayload))
	createReq.Header.Set("Authorization", "Bearer "+token)
	createReq.Header.Set("Content-Type", "application/json")
	createRec := httptest.NewRecorder()
	router.ServeHTTP(createRec, createReq)
	
	var createdUser map[string]any
	json.Unmarshal(createRec.Body.Bytes(), &createdUser)
	existingUserID := int64(createdUser["id"].(float64))

	// Request body: update user to ADMIN role (allowed for SUPER_ADMIN)
	updateBody := map[string]any{
		"role_code": "ADMIN",
	}
	updatePayload, _ := json.Marshal(updateBody)

	// Execute: PUT /api/v1/admin/users/{id}
	updateReq := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/api/v1/admin/users/%d", existingUserID), bytes.NewReader(updatePayload))
	updateReq.Header.Set("Authorization", "Bearer "+token)
	updateReq.Header.Set("Content-Type", "application/json")
	updateRec := httptest.NewRecorder()

	router.ServeHTTP(updateRec, updateReq)

	// Assert: Should return 200 OK
	require.Equal(t, http.StatusOK, updateRec.Code, "should return 200 OK")

	// Assert: Response should contain updated user JSON
	var user map[string]any
	err := json.Unmarshal(updateRec.Body.Bytes(), &user)
	require.NoError(t, err, "response should be valid JSON")
	require.Equal(t, "Existing User", user["name"], "user name should remain unchanged")
	require.Equal(t, "user@example.com", user["email"], "user email should remain unchanged")
	require.Equal(t, "ADMIN", user["role_code"], "user role_code should be updated to ADMIN")
	require.Equal(t, float64(existingUserID), user["id"], "user ID should match")
}

