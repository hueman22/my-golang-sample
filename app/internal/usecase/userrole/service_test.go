package userrole

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	domuser "example.com/my-golang-sample/app/internal/domain/user"
	domrole "example.com/my-golang-sample/app/internal/domain/userrole"
)

type mockRoleRepository struct {
	roles     map[int64]*domrole.UserRole
	nextID    int64
	created   *domrole.UserRole
	updated   *domrole.UserRole
	deletedID int64
}

func newMockRoleRepository() *mockRoleRepository {
	return &mockRoleRepository{
		roles: map[int64]*domrole.UserRole{
			1: {ID: 1, Code: domuser.RoleCodeSuperAdmin, Name: "Super Admin", IsSystem: true},
			2: {ID: 2, Code: domuser.RoleCodeAdmin, Name: "Admin", IsSystem: true},
			3: {ID: 3, Code: domuser.RoleCodeCustomer, Name: "Customer", IsSystem: true},
		},
		nextID: 4,
	}
}

func (m *mockRoleRepository) Create(ctx context.Context, role *domrole.UserRole) (*domrole.UserRole, error) {
	for _, existing := range m.roles {
		if existing.Code == role.Code {
			return nil, domrole.ErrRoleCodeExisted
		}
	}
	role.ID = m.nextID
	m.nextID++
	m.roles[role.ID] = role
	m.created = role
	return role, nil
}

func (m *mockRoleRepository) Update(ctx context.Context, role *domrole.UserRole) (*domrole.UserRole, error) {
	if _, ok := m.roles[role.ID]; !ok {
		return nil, domrole.ErrRoleNotFound
	}
	m.roles[role.ID] = role
	m.updated = role
	return role, nil
}

func (m *mockRoleRepository) Delete(ctx context.Context, id int64) error {
	if _, ok := m.roles[id]; !ok {
		return domrole.ErrRoleNotFound
	}
	delete(m.roles, id)
	m.deletedID = id
	return nil
}

func (m *mockRoleRepository) GetByID(ctx context.Context, id int64) (*domrole.UserRole, error) {
	role, ok := m.roles[id]
	if !ok {
		return nil, domrole.ErrRoleNotFound
	}
	cloned := *role
	return &cloned, nil
}

func (m *mockRoleRepository) GetByCode(ctx context.Context, code string) (*domrole.UserRole, error) {
	for _, role := range m.roles {
		if string(role.Code) == code {
			cloned := *role
			return &cloned, nil
		}
	}
	return nil, domrole.ErrRoleNotFound
}

func (m *mockRoleRepository) List(ctx context.Context, filter domrole.ListFilter) ([]*domrole.UserRole, error) {
	result := make([]*domrole.UserRole, 0, len(m.roles))
	for _, role := range m.roles {
		cloned := *role
		result = append(result, &cloned)
	}
	return result, nil
}

func TestService_Create_NormalizesCode(t *testing.T) {
	repo := newMockRoleRepository()
	svc := NewService(repo)

	role, err := svc.Create(context.Background(), CreateInput{
		Code:        "  support_agent ",
		Name:        "Support Agent",
		Description: "Handle tickets",
	})

	require.NoError(t, err)
	require.Equal(t, domuser.RoleCode("SUPPORT_AGENT"), role.Code)
	require.Equal(t, repo.created, role)
}

func TestService_Create_InvalidCode(t *testing.T) {
	repo := newMockRoleRepository()
	svc := NewService(repo)

	_, err := svc.Create(context.Background(), CreateInput{
		Code: "bad code!",
		Name: "Broken",
	})

	require.ErrorIs(t, err, domuser.ErrInvalidRoleCode)
	require.Nil(t, repo.created)
}

func TestService_Update_MergesFields(t *testing.T) {
	repo := newMockRoleRepository()
	svc := NewService(repo)

	newName := "Renamed Admin"
	newDesc := "Updated description"
	role, err := svc.Update(context.Background(), UpdateInput{
		ID:          2,
		Name:        &newName,
		Description: &newDesc,
	})

	require.NoError(t, err)
	require.Equal(t, newName, role.Name)
	require.Equal(t, newDesc, role.Description)
	require.Equal(t, repo.updated, role)
}

func TestService_Delete_ForwardsToRepo(t *testing.T) {
	repo := newMockRoleRepository()
	svc := NewService(repo)

	err := svc.Delete(context.Background(), 3)

	require.NoError(t, err)
	require.Equal(t, int64(3), repo.deletedID)
}


