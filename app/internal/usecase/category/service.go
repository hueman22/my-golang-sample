package category

import (
	"context"
	"strings"

	dom "example.com/my-golang-sample/app/internal/domain/category"
)

const maxSlugLength = 64

type Service struct {
	repo dom.Repository
}

func NewService(repo dom.Repository) *Service {
	return &Service{repo: repo}
}

type CreateInput struct {
	Name        string
	Slug        *string
	Description string
	IsActive    *bool
}

type UpdateInput struct {
	ID          int64
	Name        *string
	Slug        *string
	Description *string
	IsActive    *bool
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*dom.Category, error) {
	name, err := sanitizeName(in.Name)
	if err != nil {
		return nil, err
	}
	slug, err := buildSlug(name, in.Slug)
	if err != nil {
		return nil, err
	}

	isActive := true
	if in.IsActive != nil {
		isActive = *in.IsActive
	}

	category := &dom.Category{
		Name:        name,
		Slug:        slug,
		Description: strings.TrimSpace(in.Description),
		IsActive:    isActive,
	}
	return s.repo.Create(ctx, category)
}

func (s *Service) Update(ctx context.Context, in UpdateInput) (*dom.Category, error) {
	existing, err := s.repo.GetByID(ctx, in.ID)
	if err != nil {
		return nil, err
	}

	nameChanged := false
	if in.Name != nil {
		name, err := sanitizeName(*in.Name)
		if err != nil {
			return nil, err
		}
		if existing.Name != name {
			nameChanged = true
		}
		existing.Name = name
	}

	if in.Description != nil {
		existing.Description = strings.TrimSpace(*in.Description)
	}

	if in.IsActive != nil {
		existing.IsActive = *in.IsActive
	}

	switch {
	case in.Slug != nil:
		slug, err := buildSlug(existing.Name, in.Slug)
		if err != nil {
			return nil, err
		}
		existing.Slug = slug
	case nameChanged || existing.Slug == "":
		slug, err := buildSlug(existing.Name, nil)
		if err != nil {
			return nil, err
		}
		existing.Slug = slug
	}

	return s.repo.Update(ctx, existing)
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

func sanitizeName(name string) (string, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return "", dom.ErrCategoryInvalidName
	}
	return trimmed, nil
}

func buildSlug(name string, slugInput *string) (string, error) {
	source := name
	if slugInput != nil {
		source = *slugInput
	}
	slug := slugify(source)
	if slug == "" {
		return "", dom.ErrCategoryInvalidSlug
	}
	if len(slug) > maxSlugLength {
		return "", dom.ErrCategoryInvalidSlug
	}
	return slug, nil
}

func slugify(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	var b strings.Builder
	b.Grow(len(input))

	previousDash := false
	for _, r := range input {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			previousDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			previousDash = false
		default:
			if !previousDash {
				b.WriteByte('-')
				previousDash = true
			}
		}
	}
	slug := b.String()
	slug = strings.Trim(slug, "-")
	slug = strings.ReplaceAll(slug, "--", "-")
	return slug
}
