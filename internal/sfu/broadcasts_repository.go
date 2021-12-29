package sfu

import (
	"github.com/jmoiron/sqlx"
)

const (
	pageDefault    int = 1
	perPageDefault int = 50
)

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
