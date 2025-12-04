package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	domuser "example.com/my-golang-sample/app/internal/domain/user"
)

type mockUserRepository struct {
	usersByEmail map[string]*domuser.User
	getByEmailErr error
}

func newMockUserRepository() *mockUserRepository {
	return &mockUserRepository{
		usersByEmail: make(map[string]*domuser.User),
	}
}

func (m *mockUserRepository) Create(ctx context.Context, u *domuser.User) (*domuser.User, error) {
	return nil, nil
}

func (m *mockUserRepository) GetByID(ctx context.Context, id int64) (*domuser.User, error) {
	return nil, nil
}

func (m *mockUserRepository) GetByEmail(ctx context.Context, email string) (*domuser.User, error) {
	if m.getByEmailErr != nil {
		return nil, m.getByEmailErr
	}
	if user, ok := m.usersByEmail[email]; ok {
		cloned := *user
		return &cloned, nil
	}
	return nil, domuser.ErrUserNotFound
}

func (m *mockUserRepository) List(ctx context.Context, filter domuser.ListUsersFilter) ([]*domuser.User, error) {
	return nil, nil
}

func (m *mockUserRepository) Update(ctx context.Context, u *domuser.User) (*domuser.User, error) {
	return nil, nil
}

func (m *mockUserRepository) Delete(ctx context.Context, id int64) error {
	return nil
}

func (m *mockUserRepository) GetRoleIDByCode(ctx context.Context, code domuser.RoleCode) (int64, error) {
	return 0, nil
}

type mockPasswordComparer struct {
	compareErr error
}

func (m *mockPasswordComparer) Compare(hash string, password string) error {
	return m.compareErr
}

type mockTokenService struct {
	token      string
	generateErr error
}

func (m *mockTokenService) GenerateToken(u *domuser.User) (string, error) {
	if m.generateErr != nil {
		return "", m.generateErr
	}
	if m.token != "" {
		return m.token, nil
	}
	return "mock-token-" + u.Email, nil
}

func (m *mockTokenService) ParseToken(token string) (*Claims, error) {
	return nil, nil
}

func TestLogin_Success(t *testing.T) {
	repo := newMockUserRepository()
	user := &domuser.User{
		ID:           1,
		Name:         "John Doe",
		Email:        "john@example.com",
		PasswordHash: "hashed_password",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	repo.usersByEmail["john@example.com"] = user

	checker := &mockPasswordComparer{compareErr: nil}
	tokenSvc := &mockTokenService{token: "valid-jwt-token"}

	svc := NewService(repo, checker, tokenSvc)

	result, err := svc.Login(context.Background(), LoginInput{
		Email:    "john@example.com",
		Password: "correctpassword",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "valid-jwt-token", result.Token)
	require.NotNil(t, result.User)
	require.Equal(t, int64(1), result.User.ID)
	require.Equal(t, "John Doe", result.User.Name)
	require.Equal(t, "john@example.com", result.User.Email)
	require.Equal(t, domuser.RoleCodeCustomer, result.User.RoleCode)
}

func TestLogin_EmailNormalization(t *testing.T) {
	tests := []struct {
		name     string
		inputEmail string
		expectedEmail string
	}{
		{
			name:          "Uppercase email is lowercased",
			inputEmail:    "JOHN@EXAMPLE.COM",
			expectedEmail: "john@example.com",
		},
		{
			name:          "Email with spaces is trimmed",
			inputEmail:    "  john@example.com  ",
			expectedEmail: "john@example.com",
		},
		{
			name:          "Mixed case with spaces",
			inputEmail:    "  John@Example.COM  ",
			expectedEmail: "john@example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockUserRepository()
			user := &domuser.User{
				ID:           1,
				Name:         "John Doe",
				Email:        tt.expectedEmail,
				PasswordHash: "hashed_password",
				UserRoleID:   3,
				RoleCode:     domuser.RoleCodeCustomer,
			}
			repo.usersByEmail[tt.expectedEmail] = user

			checker := &mockPasswordComparer{compareErr: nil}
			tokenSvc := &mockTokenService{token: "valid-token"}

			svc := NewService(repo, checker, tokenSvc)

			result, err := svc.Login(context.Background(), LoginInput{
				Email:    tt.inputEmail,
				Password: "password123",
			})

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, tt.expectedEmail, result.User.Email)
		})
	}
}

func TestLogin_UserNotFound(t *testing.T) {
	repo := newMockUserRepository()
	checker := &mockPasswordComparer{}
	tokenSvc := &mockTokenService{}

	svc := NewService(repo, checker, tokenSvc)

	result, err := svc.Login(context.Background(), LoginInput{
		Email:    "nonexistent@example.com",
		Password: "anypassword",
	})

	require.ErrorIs(t, err, domuser.ErrUnauthorized)
	require.Nil(t, result)
}

func TestLogin_WrongPassword(t *testing.T) {
	repo := newMockUserRepository()
	user := &domuser.User{
		ID:           1,
		Name:         "John Doe",
		Email:        "john@example.com",
		PasswordHash: "correct_hash",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	repo.usersByEmail["john@example.com"] = user

	// Simulate password mismatch
	checker := &mockPasswordComparer{compareErr: errors.New("password mismatch")}
	tokenSvc := &mockTokenService{}

	svc := NewService(repo, checker, tokenSvc)

	result, err := svc.Login(context.Background(), LoginInput{
		Email:    "john@example.com",
		Password: "wrongpassword",
	})

	require.ErrorIs(t, err, domuser.ErrUnauthorized)
	require.Nil(t, result)
}

func TestLogin_EmptyEmailOrPassword(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		password string
	}{
		{
			name:     "Empty email",
			email:    "",
			password: "password123",
		},
		{
			name:     "Empty password",
			email:    "john@example.com",
			password: "",
		},
		{
			name:     "Both empty",
			email:    "",
			password: "",
		},
		{
			name:     "Email with only spaces",
			email:    "   ",
			password: "password123",
		},
		{
			name:     "Email with tabs",
			email:    "\t\t",
			password: "password123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockUserRepository()
			checker := &mockPasswordComparer{}
			tokenSvc := &mockTokenService{}

			svc := NewService(repo, checker, tokenSvc)

			result, err := svc.Login(context.Background(), LoginInput{
				Email:    tt.email,
				Password: tt.password,
			})

			require.ErrorIs(t, err, domuser.ErrInvalidCredential)
			require.Nil(t, result)
		})
	}
}

func TestLogin_TokenGenerationError(t *testing.T) {
	repo := newMockUserRepository()
	user := &domuser.User{
		ID:           1,
		Name:         "John Doe",
		Email:        "john@example.com",
		PasswordHash: "hashed_password",
		UserRoleID:   3,
		RoleCode:     domuser.RoleCodeCustomer,
	}
	repo.usersByEmail["john@example.com"] = user

	checker := &mockPasswordComparer{compareErr: nil}
	tokenSvc := &mockTokenService{
		generateErr: errors.New("token generation failed"),
	}

	svc := NewService(repo, checker, tokenSvc)

	result, err := svc.Login(context.Background(), LoginInput{
		Email:    "john@example.com",
		Password: "correctpassword",
	})

	require.Error(t, err)
	require.Equal(t, "token generation failed", err.Error())
	require.Nil(t, result)
}

func TestLogin_DifferentUserRoles(t *testing.T) {
	tests := []struct {
		name     string
		roleCode domuser.RoleCode
	}{
		{
			name:     "Login as SUPER_ADMIN",
			roleCode: domuser.RoleCodeSuperAdmin,
		},
		{
			name:     "Login as ADMIN",
			roleCode: domuser.RoleCodeAdmin,
		},
		{
			name:     "Login as CUSTOMER",
			roleCode: domuser.RoleCodeCustomer,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := newMockUserRepository()
			user := &domuser.User{
				ID:           1,
				Name:         "Test User",
				Email:        "user@example.com",
				PasswordHash: "hashed_password",
				UserRoleID:   1,
				RoleCode:     tt.roleCode,
			}
			repo.usersByEmail["user@example.com"] = user

			checker := &mockPasswordComparer{compareErr: nil}
			tokenSvc := &mockTokenService{token: "valid-token"}

			svc := NewService(repo, checker, tokenSvc)

			result, err := svc.Login(context.Background(), LoginInput{
				Email:    "user@example.com",
				Password: "password123",
			})

			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, tt.roleCode, result.User.RoleCode)
		})
	}
}

