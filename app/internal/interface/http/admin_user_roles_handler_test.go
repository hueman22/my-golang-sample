package http

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	domuser "example.com/my-golang-sample/app/internal/domain/user"
	domrole "example.com/my-golang-sample/app/internal/domain/userrole"
	"example.com/my-golang-sample/app/internal/infra/security"
	userroleuc "example.com/my-golang-sample/app/internal/usecase/userrole"
)

type fakeRoleRepo struct {
	roles  map[int64]*domrole.UserRole
	nextID int64
	inUse  map[int64]bool
}

func newFakeRoleRepo() *fakeRoleRepo {
	return &fakeRoleRepo{
		roles: map[int64]*domrole.UserRole{
			1: {ID: 1, Code: domuser.RoleCodeSuperAdmin, Name: "Super Admin", IsSystem: true},
			2: {ID: 2, Code: domuser.RoleCodeAdmin, Name: "Admin", IsSystem: true},
			3: {ID: 3, Code: domuser.RoleCodeCustomer, Name: "Customer", IsSystem: true},
		},
		nextID: 4,
		inUse:  map[int64]bool{},
	}
}

func (f *fakeRoleRepo) Create(ctx context.Context, role *domrole.UserRole) (*domrole.UserRole, error) {
	for _, existing := range f.roles {
		if existing.Code == role.Code {
			return nil, domrole.ErrRoleCodeExisted
		}
	}
	role.ID = f.nextID
	f.nextID++
	f.roles[role.ID] = role
	return role, nil
}

func (f *fakeRoleRepo) Update(ctx context.Context, role *domrole.UserRole) (*domrole.UserRole, error) {
	if _, ok := f.roles[role.ID]; !ok {
		return nil, domrole.ErrRoleNotFound
	}
	f.roles[role.ID] = role
	return role, nil
}

func (f *fakeRoleRepo) Delete(ctx context.Context, id int64) error {
	role, ok := f.roles[id]
	if !ok {
		return domrole.ErrRoleNotFound
	}
	if role.IsSystem {
		return domrole.ErrRoleImmutable
	}
	if f.inUse[id] {
		return domrole.ErrRoleInUse
	}
	delete(f.roles, id)
	return nil
}

func (f *fakeRoleRepo) GetByID(ctx context.Context, id int64) (*domrole.UserRole, error) {
	role, ok := f.roles[id]
	if !ok {
		return nil, domrole.ErrRoleNotFound
	}
	cloned := *role
	return &cloned, nil
}

func (f *fakeRoleRepo) GetByCode(ctx context.Context, code string) (*domrole.UserRole, error) {
	for _, role := range f.roles {
		if string(role.Code) == code {
			cloned := *role
			return &cloned, nil
		}
	}
	return nil, domrole.ErrRoleNotFound
}

func (f *fakeRoleRepo) List(ctx context.Context, filter domrole.ListFilter) ([]*domrole.UserRole, error) {
	results := make([]*domrole.UserRole, 0, len(f.roles))
	var query string
	if filter.Query != nil {
		query = strings.ToLower(*filter.Query)
	}

	for _, role := range f.roles {
		if query != "" {
			if !strings.Contains(strings.ToLower(string(role.Code)), query) &&
				!strings.Contains(strings.ToLower(role.Name), query) {
				continue
			}
		}
		cloned := *role
		results = append(results, &cloned)
	}
	return results, nil
}

func newTestAPIForRoles(repo domrole.Repository) (*API, string) {
	roleSvc := userroleuc.NewService(repo)
	tokenSvc := security.NewJWTService("secret", time.Hour)

	api := NewAPI(Dependencies{
		UserRoleService: roleSvc,
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

func TestAdminUserRoleCRUDFlow(t *testing.T) {
	repo := newFakeRoleRepo()
	api, token := newTestAPIForRoles(repo)
	router := api.Router()

	// Create role
	createPayload := map[string]any{
		"code":        "content_manager",
		"name":        "Content Manager",
		"description": "Manage content",
	}
	payloadBytes, _ := json.Marshal(createPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/user-roles", bytes.NewReader(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var created map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &created))
	roleID := int(created["id"].(float64))
	roleIDStr := strconv.Itoa(roleID)

	// List with query filter
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/user-roles?q=content", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var listResp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &listResp))
	data := listResp["data"].([]any)
	require.Len(t, data, 1)

	// Get detail
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/user-roles/"+roleIDStr, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Update role
	updatePayload := map[string]any{
		"name":        "Content Lead",
		"description": "Updated",
	}
	updateBytes, _ := json.Marshal(updatePayload)
	req = httptest.NewRequest(http.MethodPut, "/api/v1/admin/user-roles/"+roleIDStr, bytes.NewReader(updateBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())

	// Delete role
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/user-roles/"+roleIDStr, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())

	// Verify deleted
	req = httptest.NewRequest(http.MethodGet, "/api/v1/admin/user-roles/"+roleIDStr, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminUserRole_DeleteInUseReturns422(t *testing.T) {
	repo := newFakeRoleRepo()
	role, err := domuser.ParseRoleCode("SUPPORT")
	require.NoError(t, err)
	repo.roles[10] = &domrole.UserRole{ID: 10, Code: role, Name: "Support"}
	repo.inUse[10] = true

	api, token := newTestAPIForRoles(repo)
	router := api.Router()

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/admin/user-roles/10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code, rec.Body.String())
}

func TestAdminUserRole_CreateDuplicateReturns409(t *testing.T) {
	repo := newFakeRoleRepo()
	api, token := newTestAPIForRoles(repo)
	router := api.Router()

	payload := map[string]any{
		"code": "ADMIN",
		"name": "Duplicate",
	}
	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/user-roles", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusConflict, rec.Code, rec.Body.String())
}
