package category

import (
	"context"

	dom "example.com/my-golang-sample/app/internal/domain/category"
)

type Service struct {
	repo dom.Repository
}

func NewService(repo dom.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, c *dom.Category) (*dom.Category, error) {
	if !c.IsActive {
		c.IsActive = true
	}
	return s.repo.Create(ctx, c)
}

func (s *Service) Update(ctx context.Context, c *dom.Category) (*dom.Category, error) {
	existed, err := s.repo.GetByID(ctx, c.ID)
	if err != nil {
		return nil, err
	}

	if c.Name != "" {
		existed.Name = c.Name
	}
	if c.Description != "" {
		existed.Description = c.Description
	}
	existed.IsActive = c.IsActive

	return s.repo.Update(ctx, existed)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) GetByID(ctx context.Context, id int64) (*dom.Category, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context, filter dom.ListFilter) ([]*dom.Category, error) {
	return s.repo.List(ctx, filter)
}

