package user

import (
	"context"

	dom "example.com/my-golang-sample/app/internal/domain/user"
)

type Service struct {
	repo dom.Repository
}

func NewService(repo dom.Repository) *Service {
	return &Service{repo: repo}
}

type CreateUserInput struct {
	ExecutorRole dom.RoleCode
	Name         string
	Email        string
	RoleCode     dom.RoleCode
}

type UpdateUserInput struct {
	ExecutorRole dom.RoleCode
	ID           int64
	Name         *string
	Email        *string
	RoleCode     *dom.RoleCode
}

func (s *Service) CreateUser(ctx context.Context, in CreateUserInput) (*dom.User, error) {
	if !in.RoleCode.IsValid() {
		return nil, dom.ErrInvalidRoleCode
	}

	if !dom.CanAssignRole(in.ExecutorRole, in.RoleCode) {
		return nil, dom.ErrCannotAssignRole
	}

	roleID, err := s.repo.GetRoleIDByCode(ctx, in.RoleCode)
	if err != nil {
		return nil, err
	}

	u := &dom.User{
		Name:       in.Name,
		Email:      in.Email,
		Password:   "$2y$12$SOME_FAKE_HASH_FOR_DEMO", // TODO: thay bằng hash thực
		UserRoleID: roleID,
		RoleCode:   in.RoleCode,
	}

	return s.repo.Create(ctx, u)
}

func (s *Service) GetUser(ctx context.Context, id int64) (*dom.User, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) UpdateUser(ctx context.Context, in UpdateUserInput) (*dom.User, error) {
	u, err := s.repo.GetByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}

	if in.RoleCode != nil {
		if !in.RoleCode.IsValid() {
			return nil, dom.ErrInvalidRoleCode
		}
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

	return s.repo.Update(ctx, u)
}

func (s *Service) DeleteUser(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
