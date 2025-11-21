package security

import (
	"time"

	"github.com/golang-jwt/jwt/v5"

	domuser "example.com/my-golang-sample/app/internal/domain/user"
	authuc "example.com/my-golang-sample/app/internal/usecase/auth"
)

type JWTService struct {
	secret     []byte
	expiration time.Duration
}

func NewJWTService(secret string, expiration time.Duration) *JWTService {
	return &JWTService{
		secret:     []byte(secret),
		expiration: expiration,
	}
}

type jwtClaims struct {
	UserID int64  `json:"uid"`
	Role   string `json:"role"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	jwt.RegisteredClaims
}

func (s *JWTService) GenerateToken(u *domuser.User) (string, error) {
	now := time.Now()
	claims := jwtClaims{
		UserID: u.ID,
		Role:   string(u.RoleCode),
		Email:  u.Email,
		Name:   u.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.expiration)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *JWTService) ParseToken(token string) (*authuc.Claims, error) {
	parsed, err := jwt.ParseWithClaims(token, &jwtClaims{}, func(token *jwt.Token) (interface{}, error) {
		return s.secret, nil
	})
	if err != nil {
		return nil, err
	}

	claims, ok := parsed.Claims.(*jwtClaims)
	if !ok || !parsed.Valid {
		return nil, err
	}

	role, err := domuser.ParseRoleCode(claims.Role)
	if err != nil {
		return nil, err
	}

	return &authuc.Claims{
		UserID:   claims.UserID,
		RoleCode: role,
		Email:    claims.Email,
		Name:     claims.Name,
	}, nil
}

