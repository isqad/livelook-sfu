package sfu

import "github.com/jmoiron/sqlx"

// User is central subject of the application
type User struct {
	ID       string `json:"id" db:"id"`
	Email    string `json:"email" db:"email"`
	Password string `json:"-" db:"password"`
}

// NewUser creates new user subject
func NewUser(id string) *User {
	return &User{ID: id}
}

// Save saves the user to DB
func (u *User) Save(db *sqlx.DB) (*User, error) {
	_, err := db.Exec(`INSERT INTO users (id, created_at) VALUES ($1, NOW())`, u.ID)
	if err != nil {
		return nil, err
	}

	return u, nil
}
