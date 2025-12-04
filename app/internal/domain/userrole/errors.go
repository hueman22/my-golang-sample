package userrole

import "errors"

var (
	ErrRoleNotFound    = errors.New("role not found")
	ErrRoleCodeExisted = errors.New("role code already exists")
	ErrRoleInUse       = errors.New("role is in use")
	ErrRoleImmutable   = errors.New("system role cannot be modified")
)
