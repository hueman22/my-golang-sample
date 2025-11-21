package userrole

import "errors"

var (
	ErrRoleNotFound    = errors.New("role not found")
	ErrRoleCodeExisted = errors.New("role code already exists")
)

