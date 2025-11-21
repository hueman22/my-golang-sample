package userrole

import (
	domuser "example.com/my-golang-sample/app/internal/domain/user"
)

type UserRole struct {
	ID          int64
	Code        domuser.RoleCode
	Name        string
	Description string
	IsSystem    bool
}

type ListFilter struct {
	Query *string
}

