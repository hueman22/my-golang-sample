package order

import (
	"context"

	domorder "example.com/my-golang-sample/app/internal/domain/order"
)

type Service struct {
	repo domorder.Repository
}

func NewService(repo domorder.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context) ([]*domorder.Order, error) {
	return s.repo.List(ctx)
}

func (s *Service) GetByID(ctx context.Context, id int64) (*domorder.Order, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *Service) UpdateStatus(ctx context.Context, id int64, status domorder.Status) (*domorder.Order, error) {
	if !status.IsValid() {
		return nil, domorder.ErrInvalidStatus
	}
	return s.repo.UpdateStatus(ctx, id, status)
}

