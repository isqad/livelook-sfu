package core

import "github.com/jmoiron/sqlx"

type streamImageDbStore struct {
	db *sqlx.DB
}

func NewStreamImageDbStore(db *sqlx.DB) streamImageDbStore {
	return streamImageDbStore{db}
}

func (s streamImageDbStore) Save(img *StreamImage) error {

	return nil
}
