package user

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	domuser "example.com/my-golang-sample/app/internal/domain/user"
	domrole "example.com/my-golang-sample/app/internal/domain/userrole"
)

// --- Enhanced Mock Repository ---

type enhancedMockUserRepository struct {
	users            map[int64]*domuser.User
	emailIndex       map[string]int64
	nextID           int64
	createErr        error
	updateErr        error
	getByIDErr       error
	getRoleIDErr     error
	roleIDs          map[domuser.RoleCode]int64
	createCalled     bool
	updateCalled     bool
	getRoleIDCalled  bool
}

func newEnhancedMockUserRepository() *enhancedMockUserRepository {
	return &enhancedMockUserRepository{
		users:      make(map[int64]*domuser.User),
		emailIndex: make(map[string]int64),
		nextID:     1,
		roleIDs: map[domuser.RoleCode]int64{
			domuser.RoleCodeSuperAdmin: 1,
			domuser.RoleCodeAdmin:      2,
			domuser.RoleCodeCustomer:   3,
			domuser.RoleCode("GUEST"):  4,
		},
	}
}

func (m *enhancedMockUserRepository) Create(ctx context.Context, u *domuser.User) (*domuser.User, error) {
	m.createCalled = true
	if m.createErr != nil {
		return nil, m.createErr
	}
	if _, exists := m.emailIndex[u.Email]; exists {
		return nil, domuser.ErrEmailAlreadyUsed
	}
	u.ID = m.nextID
	m.nextID++
	m.users[u.ID] = m.clone(u)
	m.emailIndex[u.Email] = u.ID
	return m.clone(u), nil
}

func (m *enhancedMockUserRepository) GetByID(ctx context.Context, id int64) (*domuser.User, error) {
	if m.getByIDErr != nil {
		return nil, m.getByIDErr
	}
	if user, ok := m.users[id]; ok {
		return m.clone(user), nil
	}
	return nil, domuser.ErrUserNotFound
}

func (m *enhancedMockUserRepository) GetByEmail(ctx context.Context, email string) (*domuser.User, error) {
	if id, ok := m.emailIndex[email]; ok {
		return m.clone(m.users[id]), nil
	}
	return nil, domuser.ErrUserNotFound
}

func (m *enhancedMockUserRepository) List(ctx context.Context, filter domuser.ListUsersFilter) ([]*domuser.User, error) {
	var result []*domuser.User
	for _, user := range m.users {
		if filter.RoleCode != nil && user.RoleCode != *filter.RoleCode {
			continue
		}
		result = append(result, m.clone(user))
	}
	return result, nil
}

func (m *enhancedMockUserRepository) Update(ctx context.Context, u *domuser.User) (*domuser.User, error) {
	m.updateCalled = true
	if m.updateErr != nil {
		return nil, m.updateErr
	}
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

func (m *enhancedMockUserRepository) Delete(ctx context.Context, id int64) error {
	if user, ok := m.users[id]; ok {
		delete(m.emailIndex, user.Email)
		delete(m.users, id)
		return nil
	}
	return domuser.ErrUserNotFound
}

func (m *enhancedMockUserRepository) GetRoleIDByCode(ctx context.Context, code domuser.RoleCode) (int64, error) {
	m.getRoleIDCalled = true
	if m.getRoleIDErr != nil {
		return 0, m.getRoleIDErr
	}
	if id, ok := m.roleIDs[code]; ok {
		return id, nil
	}
	return 0, domrole.ErrRoleNotFound
}

func (m *enhancedMockUserRepository) clone(u *domuser.User) *domuser.User {
	if u == nil {
		return nil
	}
	c := *u
	return &c
}

// --- Enhanced Mock Hasher ---

type enhancedMockHasher struct {
	hashErr error
}

func (m *enhancedMockHasher) Hash(password string) (string, error) {
	if m.hashErr != nil {
		return "", m.hashErr
	}
	return "hashed:" + password, nil
}

// --- Test Cases ---

func TestCreateUser_EmptyPassword_ReturnsError(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	_, err := svc.CreateUser(context.Background(), CreateUserInput{
		ExecutorRole: domuser.RoleCodeSuperAdmin,
		Name:         "Test User",
		Email:        "test@example.com",
		Password:     "", // Empty password
		RoleCode:     domuser.RoleCodeCustomer,
	})

	require.ErrorIs(t, err, domuser.ErrInvalidCredential)
	require.False(t, repo.createCalled, "repo.Create should not be called")
	require.False(t, repo.getRoleIDCalled, "GetRoleIDByCode should not be called")
}

func TestCreateUser_InvalidRole_ReturnsError(t *testing.T) {
	tests := []struct {
		name          string
		roleCode      domuser.RoleCode
		expectedError error
		shouldCallGetRoleID bool
	}{
		{
			name:          "Empty role code",
			roleCode:      domuser.RoleCode(""),
			expectedError: domuser.ErrInvalidRoleCode,
			shouldCallGetRoleID: false,
		},
		{
			name:          "Lowercase role",
			roleCode:      domuser.RoleCode("admin"),
			expectedError: domuser.ErrInvalidRoleCode,
			shouldCallGetRoleID: false,
		},
		{
			name:          "Role with spaces",
			roleCode:      domuser.RoleCode("ADMIN USER"),
			expectedError: domuser.ErrInvalidRoleCode,
			shouldCallGetRoleID: false,
		},
		{
			name:          "Role with special chars",
			roleCode:      domuser.RoleCode("ADMIN@123"),
			expectedError: domuser.ErrInvalidRoleCode,
			shouldCallGetRoleID: false,
		},
		{
			name:          "Valid format but non-existent role",
			roleCode:      domuser.RoleCode("INVALID_ROLE"), // Passes regex but doesn't exist
			expectedError: domrole.ErrRoleNotFound, // Will be returned by GetRoleIDByCode
			shouldCallGetRoleID: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newEnhancedMockUserRepository()
			hasher := &enhancedMockHasher{}
			svc := NewService(repo, hasher)

			_, err := svc.CreateUser(context.Background(), CreateUserInput{
				ExecutorRole: domuser.RoleCodeSuperAdmin,
				Name:         "Test User",
				Email:        "test@example.com",
				Password:     "password123",
				RoleCode:     tt.roleCode,
			})

			require.ErrorIs(t, err, tt.expectedError)
			require.False(t, repo.createCalled, "repo.Create should not be called")
			require.Equal(t, tt.shouldCallGetRoleID, repo.getRoleIDCalled, "GetRoleIDByCode call expectation mismatch")
		})
	}
}

func TestCreateUser_DuplicateEmail_ReturnsError(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	// Pre-populate with a user
	existingUser := &domuser.User{
		ID:           1,
		Name:         "Existing User",
		Email:        "existing@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	repo.users[1] = existingUser
	repo.emailIndex["existing@example.com"] = 1

	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	_, err := svc.CreateUser(context.Background(), CreateUserInput{
		ExecutorRole: domuser.RoleCodeSuperAdmin,
		Name:         "New User",
		Email:        "existing@example.com", // Duplicate email
		Password:     "password123",
		RoleCode:     domuser.RoleCodeCustomer,
	})

	require.ErrorIs(t, err, domuser.ErrEmailAlreadyUsed)
	require.True(t, repo.createCalled, "repo.Create should be called to detect duplicate")
	require.Len(t, repo.users, 1, "no new user should be created")
}

func TestCreateUser_AsAdmin_CannotCreateAdmin(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	_, err := svc.CreateUser(context.Background(), CreateUserInput{
		ExecutorRole: domuser.RoleCodeAdmin, // Admin executor
		Name:         "New Admin",
		Email:        "newadmin@example.com",
		Password:     "password123",
		RoleCode:     domuser.RoleCodeAdmin, // Trying to create ADMIN
	})

	require.ErrorIs(t, err, domuser.ErrCannotAssignRole)
	require.False(t, repo.createCalled, "repo.Create should not be called")
	require.False(t, repo.getRoleIDCalled, "GetRoleIDByCode should not be called")
}

func TestCreateUser_AsSuperAdmin_CanCreateAdmin(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	user, err := svc.CreateUser(context.Background(), CreateUserInput{
		ExecutorRole: domuser.RoleCodeSuperAdmin,
		Name:         "New Admin",
		Email:        "newadmin@example.com",
		Password:     "password123",
		RoleCode:     domuser.RoleCodeAdmin,
	})

	require.NoError(t, err)
	require.True(t, repo.createCalled, "repo.Create should be called")
	require.True(t, repo.getRoleIDCalled, "GetRoleIDByCode should be called")
	require.Equal(t, domuser.RoleCodeAdmin, user.RoleCode)
	require.Equal(t, int64(2), user.UserRoleID, "should have correct role ID")
	require.Equal(t, "hashed:password123", user.PasswordHash)
}

func TestCreateUser_GetRoleIDByCodeError_ReturnsError(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	repo.getRoleIDErr = domrole.ErrRoleNotFound
	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	_, err := svc.CreateUser(context.Background(), CreateUserInput{
		ExecutorRole: domuser.RoleCodeSuperAdmin,
		Name:         "Test User",
		Email:        "test@example.com",
		Password:     "password123",
		RoleCode:     domuser.RoleCodeCustomer,
	})

	require.ErrorIs(t, err, domrole.ErrRoleNotFound)
	require.False(t, repo.createCalled, "repo.Create should not be called")
	require.True(t, repo.getRoleIDCalled, "GetRoleIDByCode should be called")
}

func TestCreateUser_HasherError_ReturnsError(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	hasher := &enhancedMockHasher{
		hashErr: errors.New("hashing failed"),
	}
	svc := NewService(repo, hasher)

	_, err := svc.CreateUser(context.Background(), CreateUserInput{
		ExecutorRole: domuser.RoleCodeSuperAdmin,
		Name:         "Test User",
		Email:        "test@example.com",
		Password:     "password123",
		RoleCode:     domuser.RoleCodeCustomer,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "hashing failed")
	require.False(t, repo.createCalled, "repo.Create should not be called")
}

func TestCreateUser_RepositoryCreateError_PropagatesError(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	repo.createErr = errors.New("database error")
	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	_, err := svc.CreateUser(context.Background(), CreateUserInput{
		ExecutorRole: domuser.RoleCodeSuperAdmin,
		Name:         "Test User",
		Email:        "test@example.com",
		Password:     "password123",
		RoleCode:     domuser.RoleCodeCustomer,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "database error")
	require.True(t, repo.createCalled, "repo.Create should be called")
}

func TestUpdateUser_NotFound_ReturnsError(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	// No users in repo
	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	targetRole := domuser.RoleCodeAdmin
	_, err := svc.UpdateUser(context.Background(), UpdateUserInput{
		ExecutorRole: domuser.RoleCodeSuperAdmin,
		ID:           999, // Non-existent user
		RoleCode:     &targetRole,
	})

	require.ErrorIs(t, err, domuser.ErrUserNotFound)
	require.False(t, repo.updateCalled, "repo.Update should not be called")
}

func TestUpdateUser_InvalidRole_ReturnsError(t *testing.T) {
	tests := []struct {
		name          string
		roleCode      domuser.RoleCode
		expectedError error
		shouldCallGetRoleID bool
	}{
		{
			name:          "Empty role code",
			roleCode:      domuser.RoleCode(""),
			expectedError: domuser.ErrInvalidRoleCode,
			shouldCallGetRoleID: false,
		},
		{
			name:          "Lowercase role",
			roleCode:      domuser.RoleCode("admin"),
			expectedError: domuser.ErrInvalidRoleCode,
			shouldCallGetRoleID: false,
		},
		{
			name:          "Valid format but non-existent role",
			roleCode:      domuser.RoleCode("INVALID_ROLE"), // Passes regex but doesn't exist
			expectedError: domrole.ErrRoleNotFound, // Will be returned by GetRoleIDByCode
			shouldCallGetRoleID: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newEnhancedMockUserRepository()
			existingUser := &domuser.User{
				ID:           1,
				Name:         "Test User",
				Email:        "test@example.com",
				PasswordHash: "hash",
				UserRoleID:   3,
				RoleCode:     domuser.RoleCodeCustomer,
			}
			repo.users[1] = existingUser
			repo.emailIndex["test@example.com"] = 1

			hasher := &enhancedMockHasher{}
			svc := NewService(repo, hasher)

			_, err := svc.UpdateUser(context.Background(), UpdateUserInput{
				ExecutorRole: domuser.RoleCodeSuperAdmin,
				ID:           1,
				RoleCode:     &tt.roleCode,
			})

			require.ErrorIs(t, err, tt.expectedError)
			require.False(t, repo.updateCalled, "repo.Update should not be called")
			require.Equal(t, tt.shouldCallGetRoleID, repo.getRoleIDCalled, "GetRoleIDByCode call expectation mismatch")
		})
	}
}

func TestUpdateUser_AsAdmin_CannotPromoteToAdmin(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	existingUser := &domuser.User{
		ID:           1,
		Name:         "Customer User",
		Email:        "customer@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	repo.users[1] = existingUser
	repo.emailIndex["customer@example.com"] = 1

	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	targetRole := domuser.RoleCodeAdmin
	_, err := svc.UpdateUser(context.Background(), UpdateUserInput{
		ExecutorRole: domuser.RoleCodeAdmin, // Admin executor
		ID:           1,
		RoleCode:     &targetRole, // Trying to promote to ADMIN
	})

	require.ErrorIs(t, err, domuser.ErrCannotAssignRole)
	require.False(t, repo.updateCalled, "repo.Update should not be called")
	require.False(t, repo.getRoleIDCalled, "GetRoleIDByCode should not be called")
	// Verify role was not changed
	require.Equal(t, domuser.RoleCodeCustomer, repo.users[1].RoleCode)
}

func TestUpdateUser_AsSuperAdmin_CanPromoteToAdmin(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	existingUser := &domuser.User{
		ID:           1,
		Name:         "Customer User",
		Email:        "customer@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	repo.users[1] = existingUser
	repo.emailIndex["customer@example.com"] = 1

	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	targetRole := domuser.RoleCodeAdmin
	updatedUser, err := svc.UpdateUser(context.Background(), UpdateUserInput{
		ExecutorRole: domuser.RoleCodeSuperAdmin,
		ID:           1,
		RoleCode:     &targetRole,
	})

	require.NoError(t, err)
	require.True(t, repo.updateCalled, "repo.Update should be called")
	require.True(t, repo.getRoleIDCalled, "GetRoleIDByCode should be called")
	require.Equal(t, domuser.RoleCodeAdmin, updatedUser.RoleCode)
	require.Equal(t, int64(2), updatedUser.UserRoleID, "should have ADMIN role ID")
}

func TestUpdateUser_GetRoleIDByCodeError_ReturnsError(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	existingUser := &domuser.User{
		ID:           1,
		Name:         "Test User",
		Email:        "test@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	repo.users[1] = existingUser
	repo.emailIndex["test@example.com"] = 1
	repo.getRoleIDErr = domrole.ErrRoleNotFound

	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	targetRole := domuser.RoleCodeAdmin
	_, err := svc.UpdateUser(context.Background(), UpdateUserInput{
		ExecutorRole: domuser.RoleCodeSuperAdmin,
		ID:           1,
		RoleCode:     &targetRole,
	})

	require.ErrorIs(t, err, domrole.ErrRoleNotFound)
	require.False(t, repo.updateCalled, "repo.Update should not be called")
}

func TestUpdateUser_HasherError_ReturnsError(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	existingUser := &domuser.User{
		ID:           1,
		Name:         "Test User",
		Email:        "test@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	repo.users[1] = existingUser
	repo.emailIndex["test@example.com"] = 1

	hasher := &enhancedMockHasher{
		hashErr: errors.New("hashing failed"),
	}
	svc := NewService(repo, hasher)

	newPassword := "newpassword123"
	_, err := svc.UpdateUser(context.Background(), UpdateUserInput{
		ExecutorRole: domuser.RoleCodeSuperAdmin,
		ID:           1,
		Password:     &newPassword,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "hashing failed")
	require.False(t, repo.updateCalled, "repo.Update should not be called")
}

func TestUpdateUser_RepositoryUpdateError_PropagatesError(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	existingUser := &domuser.User{
		ID:           1,
		Name:         "Test User",
		Email:        "test@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	repo.users[1] = existingUser
	repo.emailIndex["test@example.com"] = 1
	repo.updateErr = errors.New("database update error")

	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	newName := "Updated Name"
	_, err := svc.UpdateUser(context.Background(), UpdateUserInput{
		ExecutorRole: domuser.RoleCodeSuperAdmin,
		ID:           1,
		Name:         &newName,
	})

	require.Error(t, err)
	require.Contains(t, err.Error(), "database update error")
	require.True(t, repo.updateCalled, "repo.Update should be called")
}

func TestUpdateUser_DuplicateEmail_ReturnsError(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	existingUser1 := &domuser.User{
		ID:           1,
		Name:         "User 1",
		Email:        "user1@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	existingUser2 := &domuser.User{
		ID:           2,
		Name:         "User 2",
		Email:        "user2@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	repo.users[1] = existingUser1
	repo.users[2] = existingUser2
	repo.emailIndex["user1@example.com"] = 1
	repo.emailIndex["user2@example.com"] = 2

	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	// Try to update user1's email to user2's email
	newEmail := "user2@example.com"
	_, err := svc.UpdateUser(context.Background(), UpdateUserInput{
		ExecutorRole: domuser.RoleCodeSuperAdmin,
		ID:           1,
		Email:        &newEmail,
	})

	require.ErrorIs(t, err, domuser.ErrEmailAlreadyUsed)
	require.True(t, repo.updateCalled, "repo.Update should be called to detect duplicate")
}

func TestUpdateUser_UpdateName_Success(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	existingUser := &domuser.User{
		ID:           1,
		Name:         "Old Name",
		Email:        "test@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	repo.users[1] = existingUser
	repo.emailIndex["test@example.com"] = 1

	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	newName := "New Name"
	updatedUser, err := svc.UpdateUser(context.Background(), UpdateUserInput{
		ExecutorRole: domuser.RoleCodeSuperAdmin,
		ID:           1,
		Name:         &newName,
	})

	require.NoError(t, err)
	require.True(t, repo.updateCalled)
	require.Equal(t, "New Name", updatedUser.Name)
	require.Equal(t, "test@example.com", updatedUser.Email, "email should remain unchanged")
}

func TestUpdateUser_UpdatePassword_Success(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	existingUser := &domuser.User{
		ID:           1,
		Name:         "Test User",
		Email:        "test@example.com",
		PasswordHash: "old_hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	repo.users[1] = existingUser
	repo.emailIndex["test@example.com"] = 1

	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	newPassword := "newpassword123"
	updatedUser, err := svc.UpdateUser(context.Background(), UpdateUserInput{
		ExecutorRole: domuser.RoleCodeSuperAdmin,
		ID:           1,
		Password:     &newPassword,
	})

	require.NoError(t, err)
	require.True(t, repo.updateCalled)
	require.Equal(t, "hashed:newpassword123", updatedUser.PasswordHash)
}

func TestUpdateUser_UpdateEmail_Success(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	existingUser := &domuser.User{
		ID:           1,
		Name:         "Test User",
		Email:        "old@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	repo.users[1] = existingUser
	repo.emailIndex["old@example.com"] = 1

	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	newEmail := "new@example.com"
	updatedUser, err := svc.UpdateUser(context.Background(), UpdateUserInput{
		ExecutorRole: domuser.RoleCodeSuperAdmin,
		ID:           1,
		Email:        &newEmail,
	})

	require.NoError(t, err)
	require.True(t, repo.updateCalled)
	require.Equal(t, "new@example.com", updatedUser.Email)
	require.Equal(t, "old@example.com", existingUser.Email, "original should not be modified")
}

func TestUpdateUser_NoChanges_Success(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	existingUser := &domuser.User{
		ID:           1,
		Name:         "Test User",
		Email:        "test@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	repo.users[1] = existingUser
	repo.emailIndex["test@example.com"] = 1

	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	// Update with all nil fields (no changes)
	updatedUser, err := svc.UpdateUser(context.Background(), UpdateUserInput{
		ExecutorRole: domuser.RoleCodeSuperAdmin,
		ID:           1,
	})

	require.NoError(t, err)
	require.True(t, repo.updateCalled)
	require.Equal(t, "Test User", updatedUser.Name)
	require.Equal(t, "test@example.com", updatedUser.Email)
}

func TestGetUser_Success(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	existingUser := &domuser.User{
		ID:           1,
		Name:         "Test User",
		Email:        "test@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	repo.users[1] = existingUser

	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	user, err := svc.GetUser(context.Background(), 1)

	require.NoError(t, err)
	require.Equal(t, int64(1), user.ID)
	require.Equal(t, "Test User", user.Name)
	require.Equal(t, "test@example.com", user.Email)
}

func TestGetUser_NotFound_ReturnsError(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	_, err := svc.GetUser(context.Background(), 999)

	require.ErrorIs(t, err, domuser.ErrUserNotFound)
}

func TestGetUser_RepositoryError_PropagatesError(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	repo.getByIDErr = errors.New("database error")
	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	_, err := svc.GetUser(context.Background(), 1)

	require.Error(t, err)
	require.Contains(t, err.Error(), "database error")
}

func TestListUsers_Success(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	user1 := &domuser.User{
		ID:           1,
		Name:         "User 1",
		Email:        "user1@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	user2 := &domuser.User{
		ID:           2,
		Name:         "User 2",
		Email:        "user2@example.com",
		PasswordHash: "hash",
		UserRoleID:   2,
		RoleCode:     domuser.RoleCodeAdmin,
	}
	repo.users[1] = user1
	repo.users[2] = user2
	repo.emailIndex["user1@example.com"] = 1
	repo.emailIndex["user2@example.com"] = 2

	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	users, err := svc.ListUsers(context.Background(), domuser.ListUsersFilter{})

	require.NoError(t, err)
	require.Len(t, users, 2)
}

func TestListUsers_WithRoleFilter_Success(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	user1 := &domuser.User{
		ID:           1,
		Name:         "Customer 1",
		Email:        "customer1@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	user2 := &domuser.User{
		ID:           2,
		Name:         "Admin 1",
		Email:        "admin1@example.com",
		PasswordHash: "hash",
		UserRoleID:   2,
		RoleCode:     domuser.RoleCodeAdmin,
	}
	repo.users[1] = user1
	repo.users[2] = user2
	repo.emailIndex["customer1@example.com"] = 1
	repo.emailIndex["admin1@example.com"] = 2

	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	roleCode := domuser.RoleCodeCustomer
	users, err := svc.ListUsers(context.Background(), domuser.ListUsersFilter{
		RoleCode: &roleCode,
	})

	require.NoError(t, err)
	require.Len(t, users, 1)
	require.Equal(t, domuser.RoleCodeCustomer, users[0].RoleCode)
}

func TestDeleteUser_Success(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	existingUser := &domuser.User{
		ID:           1,
		Name:         "Test User",
		Email:        "test@example.com",
		PasswordHash: "hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	repo.users[1] = existingUser
	repo.emailIndex["test@example.com"] = 1

	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	err := svc.DeleteUser(context.Background(), 1)

	require.NoError(t, err)
	require.Len(t, repo.users, 0, "user should be deleted")
	require.Len(t, repo.emailIndex, 0, "email index should be cleared")
}

func TestDeleteUser_NotFound_ReturnsError(t *testing.T) {
	repo := newEnhancedMockUserRepository()
	hasher := &enhancedMockHasher{}
	svc := NewService(repo, hasher)

	err := svc.DeleteUser(context.Background(), 999)

	require.ErrorIs(t, err, domuser.ErrUserNotFound)
}

