package sfu

import (
	"database/sql"

	"github.com/jmoiron/sqlx"
)

// User is central subject of the application
type User struct {
	ID       string `json:"id" db:"id"`
	Email    string `json:"email" db:"email"`
	Password string `json:"-" db:"password"`
	Name     string `json:"name" db:"name"`
}

// NewUser creates new user subject
func NewUser(id string) *User {
	return &User{ID: id}
}

// AuthUser authenticate user
// TODO: move to repository
func AuthUser(db *sqlx.DB, email string, password string) (*User, error) {
	u := &User{}
	err := db.Get(u, `SELECT
	id, email, name
	FROM users
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

// Save saves the user to DB
func (u *User) Save(db *sqlx.DB) (*User, error) {
	_, err := db.Exec(`INSERT INTO users (id, created_at) VALUES ($1, NOW())`, u.ID)
	if err != nil {
		return nil, err
	}

	return u, nil
}
