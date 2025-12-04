package user

import (
	"errors"
	"regexp"
	"strings"
)

// RoleCode là "enum" đại diện cho các quyền trong hệ thống
type RoleCode string

const (
	RoleCodeSuperAdmin RoleCode = "SUPER_ADMIN"
	RoleCodeAdmin      RoleCode = "ADMIN"
	RoleCodeCustomer   RoleCode = "CUSTOMER"
)

var roleCodeRegexp = regexp.MustCompile(`^[A-Z0-9_]{3,64}$`)

// Kiểm tra xem role có hợp lệ không
func (c RoleCode) IsValid() bool {
	return roleCodeRegexp.MatchString(string(c))
}

// Helper: có phải SUPER_ADMIN không
func (c RoleCode) IsSuperAdmin() bool {
	return c == RoleCodeSuperAdmin
}

// Helper: có phải ADMIN không
func (c RoleCode) IsAdmin() bool {
	return c == RoleCodeAdmin
}

// Error dùng khi parse role string bị sai
var ErrInvalidRoleCode = errors.New("invalid role code")

// ParseRoleCode: convert từ string (từ request / DB) sang RoleCode (có validate)
func ParseRoleCode(s string) (RoleCode, error) {
	c := RoleCode(strings.ToUpper(strings.TrimSpace(s)))
	if !c.IsValid() {
		return "", ErrInvalidRoleCode
	}
	return c, nil
}

// =====================
// Policy phân quyền cho việc gán role
// =====================

// CanAssignRole: kiểm tra executorRole có được phép gán targetRole hay không
//
// Rule theo yêu cầu của bạn:
// - Chỉ SUPER_ADMIN mới được tạo/assign user có role ADMIN
// - ADMIN không được tạo/assign user role ADMIN
func CanAssignRole(executorRole RoleCode, targetRole RoleCode) bool {
	// Nếu target là ADMIN thì chỉ SUPER_ADMIN mới được phép
	if targetRole == RoleCodeAdmin {
		return executorRole == RoleCodeSuperAdmin
	}

	// Các role khác (ví dụ CUSTOMER) tạm cho phép
	return true
}
