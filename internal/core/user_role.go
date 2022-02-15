package core

// UserRoleName is type of user role
type UserRoleName string

const (
	// RoleUser is user
	RoleUser UserRoleName = "user"
	// RoleAdmin is admin
	RoleAdmin = "admin"
)

// UserRole determines the role of user
type UserRole struct {
	ID     string       `db:"id"`
	Name   UserRoleName `db:"name"`
	UserID string       `db:"user_id"`
}
