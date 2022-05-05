package core

import "github.com/jmoiron/sqlx"

type streamImageDbStore struct {
	db *sqlx.DB
}

func NewStreamImageDbStore(db *sqlx.DB) streamImageDbStore {
	return streamImageDbStore{db}
}

func (s streamImageDbStore) Save(img *StreamImage) error {
	_, err := s.db.Exec(`
		UPDATE sessions
			SET
		image_filename = $1,
		title = $2,
		updated_at = NOW()
		WHERE id = $3`,
		img.Filename,
		img.Description,
		img.Session.ID,
	)
	if err != nil {
		return err
	}

	return nil
}
