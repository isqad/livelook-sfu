package sfu

import (
	"github.com/jmoiron/sqlx"
)

const (
	sessionsPageDefault    int = 1
	sessionsPerPageDefault int = 50
)

type SessionsDBStorer interface {
	Save(*Session) (*Session, error)
}

type SessionsRepository struct {
	db *sqlx.DB
}

func NewSessionsRepository(db *sqlx.DB) *SessionsRepository {
	return &SessionsRepository{
		db: db,
	}
}

func (r *SessionsRepository) GetAll(page int, perPage int) ([]*Session, error) {
	if page == 0 {
		page = sessionsPageDefault
	}
	if perPage == 0 {
		perPage = sessionsPerPageDefault
	}

	s := []*Session{}
	err := r.db.Select(&s,
		`SELECT id, title, user_id FROM sessions ORDER BY updated_at DESC LIMIT $1 OFFSET $2`,
		perPage, (page-1)*perPage,
	)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (r *SessionsRepository) Save(session *Session) (*Session, error) {
	var id int64

	err := r.db.Get(&id,
		`INSERT INTO sessions
			(user_id, title, image_node, image_filename, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6) ON CONFLICT ON CONSTRAINT uniq_sessions_user_id DO UPDATE
			SET
				updated_at = EXCLUDED.updated_at
		RETURNING id`,
		session.UserID,
		session.Title,
		session.ImageNode,
		session.ImageFilename,
		session.CreatedAt,
		session.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	session.ID = id

	return session, nil
}
