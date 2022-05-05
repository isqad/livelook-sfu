package core

import (
	"database/sql"
	"math"

	"github.com/jmoiron/sqlx"
	"github.com/spf13/viper"
)

const (
	sessionsPageDefault    int = 1
	sessionsPerPageDefault int = 50
)

type SessionsDBStorer interface {
	Save(*Session) (*Session, error)
	SetOnline(userID UserSessionID) error
	SetOffline(userID UserSessionID) error
	StartPublish(userID UserSessionID) error
	StopPublish(userID UserSessionID) error
	FindByUserID(userID UserSessionID) (*Session, error)
}

type StreamsRepository interface {
	GetAll(page int, perPage int) (*OnlineStreams, error)
}

type OnlineStreams struct {
	Streams    []*Session
	TotalPages int
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

func (r *SessionsRepository) GetAll(page int, perPage int) (*OnlineStreams, error) {
	if page == 0 {
		page = sessionsPageDefault
	}
	if perPage == 0 {
		perPage = sessionsPerPageDefault
	}

	streams := &OnlineStreams{}

	var total int
	err := r.db.Get(&total, `SELECT COUNT(*) FROM sessions`)
	if err != nil {
		return nil, err
	}
	streams.TotalPages = int(math.Ceil(float64(total) / float64(perPage)))

	sessions := []*Session{}
	err = r.db.Select(&sessions,
		`SELECT
			id,
			title,
			user_id,
			is_online,
			image_filename,
			viewers_count,
			updated_at,
			created_at
		FROM sessions
		ORDER BY updated_at DESC LIMIT $1 OFFSET $2`,
		perPage, (page-1)*perPage,
	)
	if err != nil {
		return nil, err
	}

	for _, s := range sessions {
		img := NewStreamImage(s, viper.GetString("app.upload_root"))
		s.ImageURL, err = img.URL()
		if err != nil {
			return nil, err
		}
	}

	streams.Streams = sessions

	return streams, nil
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
		string(session.UserID),
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

func (r *SessionsRepository) SetOnline(userID UserSessionID) error {
	_, err := r.db.Exec(`UPDATE sessions SET is_online = true, updated_at = NOW() WHERE user_id = $1`,
		string(userID),
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *SessionsRepository) SetOffline(userID UserSessionID) error {
	_, err := r.db.Exec(
		`UPDATE sessions SET
			updated_at = NOW(),
			is_online = false,
			state = $1,
			media_type = NULL
		WHERE user_id = $2`,
		string(SessionIdle),
		string(userID),
	)
	return err
}

func (r *SessionsRepository) StartPublish(userID UserSessionID) error {
	_, err := r.db.Exec(
		`UPDATE sessions SET
			updated_at = NOW(),
			state = $1,
			media_type = $2,
			is_online = true
		WHERE user_id = $3`,
		string(SingleBroadcast),
		string(VideoSession),
		string(userID),
	)
	return err
}

func (r *SessionsRepository) StopPublish(userID UserSessionID) error {
	_, err := r.db.Exec(
		`UPDATE sessions SET
			updated_at = NOW(),
			state = $1,
			media_type = NULL
		WHERE user_id = $2`,
		string(SessionIdle),
		string(userID),
	)
	return err
}

func (r *SessionsRepository) FindByUserID(userID UserSessionID) (*Session, error) {
	session := &Session{}

	err := r.db.Get(session,
		`SELECT
			*
			FROM sessions
			WHERE user_id = $1 LIMIT 1`,
		string(userID),
	)

	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	return session, nil
}
