package auth

import (
	"context"
	"strings"

	domuser "example.com/my-golang-sample/app/internal/domain/user"
)

type PasswordComparer interface {
	Compare(hash string, password string) error
}

type Claims struct {
	UserID   int64
	RoleCode domuser.RoleCode
	Email    string
	Name     string
}

type TokenService interface {
	GenerateToken(u *domuser.User) (string, error)
	ParseToken(token string) (*Claims, error)
}

type Service struct {
	userRepo domuser.Repository
	checker  PasswordComparer
	tokens   TokenService
}

func NewService(
	userRepo domuser.Repository,
	checker PasswordComparer,
	tokens TokenService,
) *Service {
	return &Service{
		userRepo: userRepo,
		checker:  checker,
		tokens:   tokens,
	}
}

type LoginInput struct {
	Email    string
	Password string
}

type LoginResult struct {
	Token string
	User  *domuser.User
}

func (s *Service) Login(ctx context.Context, in LoginInput) (*LoginResult, error) {
	email := strings.TrimSpace(strings.ToLower(in.Email))
	if email == "" || in.Password == "" {
		return nil, domuser.ErrInvalidCredential
	}

	u, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, domuser.ErrUnauthorized
	}

	if err := s.checker.Compare(u.PasswordHash, in.Password); err != nil {
		return nil, domuser.ErrUnauthorized
	}

	token, err := s.tokens.GenerateToken(u)
	if err != nil {
		return nil, err
	}

	return &LoginResult{
		Token: token,
		User:  u,
	}, nil
}
