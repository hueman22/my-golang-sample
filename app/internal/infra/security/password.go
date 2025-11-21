package security

import "golang.org/x/crypto/bcrypt"

type BcryptService struct {
	cost int
}

func NewBcryptService(cost int) *BcryptService {
	if cost == 0 {
		cost = bcrypt.DefaultCost
	}
	return &BcryptService{cost: cost}
}

func (s *BcryptService) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), s.cost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func (s *BcryptService) Compare(hash string, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

