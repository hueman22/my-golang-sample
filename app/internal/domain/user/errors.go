package user

import "errors"

var (
	ErrUserNotFound     = errors.New("user not found")
	ErrCannotAssignRole = errors.New("cannot assign role")
)
