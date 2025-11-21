package user

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	domuser "example.com/my-golang-sample/app/internal/domain/user"
)

type mockUserRepository struct {
	created          bool
	updated          bool
	roleLookupCalled bool
	userByID         *domuser.User
}

func (m *mockUserRepository) Create(ctx context.Context, u *domuser.User) (*domuser.User, error) {
	m.created = true
	u.ID = 100
	return u, nil
}

func (m *mockUserRepository) GetByID(ctx context.Context, id int64) (*domuser.User, error) {
	if m.userByID != nil {
		return m.userByID, nil
	}
	return nil, domuser.ErrUserNotFound
}

func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (*domuser.User, error) {
	return nil, domuser.ErrUserNotFound
}

func (m *mockUserRepository) List(ctx context.Context, filter domuser.ListUsersFilter) ([]*domuser.User, error) {
	return nil, nil
}

func (m *mockUserRepository) Update(ctx context.Context, u *domuser.User) (*domuser.User, error) {
	m.updated = true
	return u, nil
}

func (m *mockUserRepository) Delete(ctx context.Context, id int64) error {
	return nil
}

func (m *mockUserRepository) GetRoleIDByCode(ctx context.Context, code domuser.RoleCode) (int64, error) {
	m.roleLookupCalled = true
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

type mockHasher struct{}

func (mockHasher) Hash(password string) (string, error) {
	return "hashed:" + password, nil
}

func TestService_CreateUser_AdminCannotAssignAdmin(t *testing.T) {
	repo := &mockUserRepository{}
	svc := NewService(repo, mockHasher{})

	_, err := svc.CreateUser(context.Background(), CreateUserInput{
		ExecutorRole: domuser.RoleCodeAdmin,
		Name:         "Admin A",
		Email:        "a@example.com",
		Password:     "secret123",
		RoleCode:     domuser.RoleCodeAdmin,
	})

	require.ErrorIs(t, err, domuser.ErrCannotAssignRole)
	require.False(t, repo.created)
	require.False(t, repo.roleLookupCalled)
}

func TestService_UpdateUser_AdminCannotPromoteToAdmin(t *testing.T) {
	repo := &mockUserRepository{
		userByID: &domuser.User{
			ID:           10,
			Name:         "User",
			Email:        "user@example.com",
			PasswordHash: "hash",
			UserRoleID:   3,
			RoleCode:     domuser.RoleCodeCustomer,
		},
	}
	svc := NewService(repo, mockHasher{})
	targetRole := domuser.RoleCodeAdmin

	_, err := svc.UpdateUser(context.Background(), UpdateUserInput{
		ExecutorRole: domuser.RoleCodeAdmin,
		ID:           10,
		RoleCode:     &targetRole,
	})

	require.ErrorIs(t, err, domuser.ErrCannotAssignRole)
	require.False(t, repo.updated)
	require.False(t, repo.roleLookupCalled)
}

func TestService_CreateUser_SuperAdminCanCreateAdmin(t *testing.T) {
	repo := &mockUserRepository{}
	svc := NewService(repo, mockHasher{})

	user, err := svc.CreateUser(context.Background(), CreateUserInput{
		ExecutorRole: domuser.RoleCodeSuperAdmin,
		Name:         "New Admin",
		Email:        "newadmin@example.com",
		Password:     "strongpass",
		RoleCode:     domuser.RoleCodeAdmin,
	})

	require.NoError(t, err)
	require.True(t, repo.created)
	require.True(t, repo.roleLookupCalled)
	require.Equal(t, domuser.RoleCodeAdmin, user.RoleCode)
	require.Equal(t, "hashed:strongpass", user.PasswordHash)
}

