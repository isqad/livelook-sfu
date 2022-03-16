package core

import (
	"github.com/jmoiron/sqlx"
)

const (
	sessionsPageDefault    int = 1
	sessionsPerPageDefault int = 50
)

type SessionsDBStorer interface {
	Save(*Session) (*Session, error)
	SetOnline(userID string) error
	SetOffline(userID string) error
}

type StreamsRepository interface {
	GetAll(page int, perPage int) ([]*Session, error)
	Start(session *Session) (*Session, error)
	Stop(session *Session) (*Session, error)
}

type SessionsRepository struct {
	db *sqlx.DB
}

func NewSessionsRepository(db *sqlx.DB) SessionsDBStorer {
	return &SessionsRepository{
		db: db,
	}
}

func NewStreamsRepository(db *sqlx.DB) StreamsRepository {
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
		`SELECT id, title, user_id
			FROM sessions
			WHERE state = 'broadcast_single' AND is_online
			ORDER BY updated_at DESC LIMIT $1 OFFSET $2`,
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
			(user_id, title, image_node, image_filename, created_at, updated_at, is_online)
		VALUES ($1, $2, $3, $4, $5, $6, true) ON CONFLICT ON CONSTRAINT uniq_sessions_user_id DO UPDATE
			SET
				updated_at = EXCLUDED.updated_at,
				title = EXCLUDED.title,
				image_filename = EXCLUDED.image_filename,
				is_online = true
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

func (r *SessionsRepository) SetOnline(userID string) error {
	_, err := r.db.Exec(`UPDATE sessions SET is_online = true, updated_at = NOW() WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *SessionsRepository) SetOffline(userID string) error {
	_, err := r.db.Exec(
		`UPDATE sessions SET
			updated_at = NOW(),
			is_online = false,
			state = $1,
			media_type = NULL
		WHERE user_id = $2`,
		SessionIdle,
		userID,
	)
	return err
}

func (r *SessionsRepository) Start(session *Session) (*Session, error) {
	mediaType := VideoSession
	session.MediaType = &mediaType
	session.State = SingleBroadcast

	_, err := r.db.Exec(
		`UPDATE sessions SET
			updated_at = NOW(),
			state = $1,
			media_type = $2
		WHERE user_id = $3`,
		session.State,
		session.MediaType,
		session.UserID,
	)
	return session, err
}

func (r *SessionsRepository) Stop(session *Session) (*Session, error) {
	session.State = SessionIdle
	session.MediaType = nil

	if err := r.SetOffline(session.UserID); err != nil {
		return nil, err
	}

	return session, nil
}
