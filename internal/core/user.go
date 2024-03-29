package core

import (
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const (
	AdminSessionNameKey = "_livelook_admin_session"
)

type UserSessionID string

// User is central subject of the application
type User struct {
	ID        UserSessionID `json:"id,omitempty" db:"id"`
	UID       string        `json:"uid" db:"uid"`
	Name      string        `json:"name" db:"name"`
	CreatedAt *time.Time    `json:"-" db:"created_at"`
	IsAdmin   bool          `json:"-" db:"is_admin"`
	Email     *string       `json:"-" db:"email"`
	Password  *string       `json:"-" db:"password"`
}

// NewUser creates new user subject
func NewUser() *User {
	return &User{ID: UserSessionID(uuid.New().String())}
}

func FindUserByUID(db *sqlx.DB, uid string) (*User, error) {
	user := &User{}

	err := db.Get(user, `SELECT * FROM users WHERE uid = $1 LIMIT 1`, uid)
	if err != nil {
		return nil, err
	}

	return user, nil
}

// Save saves the user to DB
func (u *User) Save(db *sqlx.DB) error {
	var id string
	err := db.Get(&id,
		`INSERT INTO users (id, uid, name, created_at) VALUES ($1, $2, $3, NOW())
		  ON CONFLICT ON CONSTRAINT uniq_users_uid DO UPDATE
		  SET
			name = EXCLUDED.name
		  RETURNING id`,
		u.ID,
		u.UID,
		u.Name,
	)
	if err != nil {
		return err
	}
	u.ID = UserSessionID(id)

	return nil
}
