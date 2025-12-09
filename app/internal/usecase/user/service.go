package user

import (
	"context"

	dom "example.com/my-golang-sample/app/internal/domain/user"
)

type PasswordHasher interface {
	Hash(password string) (string, error)
}

type Service struct {
	repo   dom.Repository
	hasher PasswordHasher
}

func NewService(repo dom.Repository, hasher PasswordHasher) *Service {
	return &Service{repo: repo, hasher: hasher}
}

type CreateUserInput struct {
	ExecutorRole dom.RoleCode
	Name         string
	Email        string
	Password     string
	RoleCode     dom.RoleCode
}

type UpdateUserInput struct {
	ExecutorRole dom.RoleCode
	ID           int64
	Name         *string
	Email        *string
	Password     *string
	RoleCode     *dom.RoleCode
}

func (s *Service) CreateUser(ctx context.Context, in CreateUserInput) (*dom.User, error) {
	if in.Password == "" {
		return nil, dom.ErrInvalidCredential
	}

	if !in.RoleCode.IsValid() {
		return nil, dom.ErrInvalidRoleCode
	}

	// Business rule: ADMIN cannot create users with ADMIN role
	if in.ExecutorRole == dom.RoleCodeAdmin && in.RoleCode == dom.RoleCodeAdmin {
		// Rule đặc biệt: admin không được tạo admin
		return nil, dom.ErrCannotAssignRole
	}

	// General RBAC rule: check executor có quyền gán role này không
	if !dom.CanAssignRole(in.ExecutorRole, in.RoleCode) {
		return nil, dom.ErrCannotAssignRole
	}

	roleID, err := s.repo.GetRoleIDByCode(ctx, in.RoleCode)
	if err != nil {
		return nil, err
	}

	hash, err := s.hasher.Hash(in.Password)
	if err != nil {
		return nil, err
	}

	u := &dom.User{
		Name:         in.Name,
		Email:        in.Email,
		PasswordHash: hash,
		UserRoleID:   roleID,
		RoleCode:     in.RoleCode,
	}

	return s.repo.Create(ctx, u)
}

func (s *Service) GetUser(ctx context.Context, id int64) (*dom.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) ListUsers(ctx context.Context, filter dom.ListUsersFilter) ([]*dom.User, error) {
	return s.repo.List(ctx, filter)
}

func (s *Service) UpdateUser(ctx context.Context, in UpdateUserInput) (*dom.User, error) {
	u, err := s.repo.GetByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}

	if in.RoleCode != nil {
		// validate role code
		if !in.RoleCode.IsValid() {
			return nil, dom.ErrInvalidRoleCode
		}

		// Business rule: ADMIN cannot update users to ADMIN role
		if in.ExecutorRole == dom.RoleCodeAdmin && *in.RoleCode == dom.RoleCodeAdmin {
			// Rule đặc biệt: admin không được promote lên admin
			return nil, dom.ErrCannotAssignRole
		}

		// General RBAC rule
		if !dom.CanAssignRole(in.ExecutorRole, *in.RoleCode) {
			return nil, dom.ErrCannotAssignRole
		}

		roleID, err := s.repo.GetRoleIDByCode(ctx, *in.RoleCode)
		if err != nil {
			return nil, err
		}
		u.UserRoleID = roleID
		u.RoleCode = *in.RoleCode
	}

	if in.Name != nil {
		u.Name = *in.Name
	}
	if in.Email != nil {
		u.Email = *in.Email
	}
	if in.Password != nil {
		hash, err := s.hasher.Hash(*in.Password)
		if err != nil {
			return nil, err
		}
		u.PasswordHash = hash
	}

	return s.repo.Update(ctx, u)
}

func (s *Service) DeleteUser(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
