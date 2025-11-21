package userrole

import (
	"context"
	"strings"

	domuser "example.com/my-golang-sample/app/internal/domain/user"
	domrole "example.com/my-golang-sample/app/internal/domain/userrole"
)

type Service struct {
	repo domrole.Repository
}

func NewService(repo domrole.Repository) *Service {
	return &Service{repo: repo}
}

type CreateInput struct {
	Code        string
	Name        string
	Description string
}

type UpdateInput struct {
	ID          int64
	Name        *string
	Description *string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*domrole.UserRole, error) {
	code := strings.ToUpper(strings.TrimSpace(in.Code))
	roleCode, err := domuser.ParseRoleCode(code)
	if err != nil {
		return nil, err
	}

	role := &domrole.UserRole{
		Code:        roleCode,
		Name:        in.Name,
		Description: in.Description,
	}

	return s.repo.Create(ctx, role)
}

func (s *Service) Update(ctx context.Context, in UpdateInput) (*domrole.UserRole, error) {
	role, err := s.repo.GetByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}

	if in.Name != nil {
		role.Name = *in.Name
	}
	if in.Description != nil {
		role.Description = *in.Description
	}

	return s.repo.Update(ctx, role)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) GetByID(ctx context.Context, id int64) (*domrole.UserRole, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context, filter domrole.ListFilter) ([]*domrole.UserRole, error) {
	return s.repo.List(ctx, filter)
}

