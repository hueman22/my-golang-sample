package user

type User struct {
	ID           int64
	Name         string
	Email        string
	PasswordHash string
	UserRoleID   int64
	RoleCode     RoleCode
}

type ListUsersFilter struct {
	RoleCode *RoleCode
}
