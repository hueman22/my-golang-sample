package user

type User struct {
	ID         int64
	Name       string
	Email      string
	Password   string // password hash
	UserRoleID int64
	RoleCode   RoleCode
}
