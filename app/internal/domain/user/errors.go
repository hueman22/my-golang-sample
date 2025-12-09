package user

import "errors"

var (
	ErrUserNotFound            = errors.New("user not found")
	ErrCannotAssignRole        = errors.New("cannot assign role")
	ErrEmailAlreadyUsed        = errors.New("email already used")
	ErrUnauthorized            = errors.New("unauthorized")
	ErrInvalidCredential       = errors.New("invalid credential")
	ErrAdminCannotCreateAdmin  = errors.New("admin cannot create admin")
	ErrAdminCannotPromoteAdmin = errors.New("admin cannot promote admin")
)
