package sfu

import "github.com/jmoiron/sqlx"

type User struct {
	ID string `json:"id" db:"id"`
}

func NewUser(id string) *User {
	return &User{ID: id}
}

func (u *User) Save(db *sqlx.DB) (*User, error) {
	_, err := db.Exec(`INSERT INTO users (id, created_at) VALUES ($1, NOW())`, u.ID)
	if err != nil {
		return nil, err
	}

	return u, nil
}
