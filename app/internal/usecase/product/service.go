package product

import (
	"context"

	dom "example.com/my-golang-sample/app/internal/domain/product"
)

type Service struct {
	repo dom.Repository
}

func NewService(repo dom.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, p *dom.Product) (*dom.Product, error) {
	return s.repo.Create(ctx, p)
}

func (s *Service) Update(ctx context.Context, p *dom.Product) (*dom.Product, error) {
	existed, err := s.repo.GetByID(ctx, p.ID)
	if err != nil {
		return nil, err
	}

	if p.Name != "" {
		existed.Name = p.Name
	}
	if p.Description != "" {
		existed.Description = p.Description
	}
	if p.Price > 0 {
		existed.Price = p.Price
	}
	if p.Stock >= 0 {
		existed.Stock = p.Stock
	}
	if p.CategoryID > 0 {
		existed.CategoryID = p.CategoryID
	}
	existed.IsActive = p.IsActive

	return s.repo.Update(ctx, existed)
}

func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *Service) GetByID(ctx context.Context, id int64) (*dom.Product, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) List(ctx context.Context, filter dom.ListFilter) ([]*dom.Product, error) {
	return s.repo.List(ctx, filter)
}

