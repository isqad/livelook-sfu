package sfu

import (
	"github.com/jmoiron/sqlx"
)

const (
	pageDefault    int = 1
	perPageDefault int = 50
)

type BroadcastsDBStorer interface {
	Save(*Broadcast) error
	SetStopped(*Broadcast) error
}

type BroadcastsRepository struct {
	db *sqlx.DB
}

func NewBroadcastsRepository(db *sqlx.DB) *BroadcastsRepository {
	return &BroadcastsRepository{
		db: db,
	}
}

func (r *BroadcastsRepository) GetAll(page int, perPage int) ([]*Broadcast, error) {
	if page == 0 {
		page = pageDefault
	}
	if perPage == 0 {
		perPage = perPageDefault
	}

	bs := []*Broadcast{}
	err := r.db.Select(&bs,
		`SELECT id, title, user_id FROM broadcasts ORDER BY created_at DESC LIMIT $1 OFFSET $2`,
		perPage, (page-1)*perPage,
	)
	if err != nil {
		return nil, err
	}

	return bs, nil
}

func (r *BroadcastsRepository) Save(broadcast *Broadcast) error {
	_, err := r.db.Exec(
		`INSERT INTO broadcasts (id, user_id, title, created_at) VALUES ($1, $2, $3, NOW())`,
		broadcast.ID,
		broadcast.UserID,
		broadcast.Title,
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *BroadcastsRepository) SetStopped(broadcast *Broadcast) error {
	_, err := r.db.Exec(
		`UPDATE broadcasts SET "state" = 'stopped' WHERE id = $1`,
		broadcast.ID,
	)
	if err != nil {
		return err
	}

	return nil
}
