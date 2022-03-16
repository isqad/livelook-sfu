package core

import (
	"database/sql"

	"github.com/jmoiron/sqlx"
)

type UserStorer interface {
	FindByUID(uid string) (*User, error)
	Find(id string) (*User, error)
	AuthAdminUser(email string, password string) (*User, error)
}

type UserRepository struct {
	db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) *UserRepository {
	return &UserRepository{
		db: db,
	}
}

func (r *UserRepository) Find(id string) (*User, error) {
	user := &User{}

	err := r.db.Get(user, `SELECT * FROM users WHERE id = $1 LIMIT 1`, id)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) FindByUID(uid string) (*User, error) {
	user := &User{}

	err := r.db.Get(user, `SELECT * FROM users WHERE uid = $1 LIMIT 1`, uid)
	if err != nil {
		return nil, err
	}

	return user, nil
}

func (r *UserRepository) AuthAdminUser(email string, password string) (*User, error) {
	u := &User{}
	err := r.db.Get(u, `SELECT * FROM users
		WHERE password = crypt($1, password) AND lower(email) = lower($2) AND is_admin LIMIT 1`,
		password,
		email,
	)
	if err != nil {
		if err != sql.ErrNoRows {
			return nil, err
		}
		return nil, nil
	}

	return u, nil
}
