package admin

import (
	"database/sql"

	"github.com/jmoiron/sqlx"
)

// User is admin
type User struct {
	ID       string `json:"id" db:"id"`
	Email    string `json:"email" db:"email"`
	Password string `json:"-" db:"password"`
	Name     string `json:"name" db:"name"`
}

// AuthAdminUser authenticate admin
// TODO: move to repository
func AuthAdminUser(db *sqlx.DB, email string, password string) (*User, error) {
	u := &User{}
	err := db.Get(u, `SELECT
	id, email, name
	FROM admin_users
	WHERE password = crypt($1, password) AND lower(email) = lower($2) LIMIT 1`,
		password, email,
	)
	if err != nil {
		if err != sql.ErrNoRows {
			return nil, err
		}
		return nil, nil
	}

	return u, nil
}
