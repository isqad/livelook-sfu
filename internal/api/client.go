package api

import (
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type Client struct {
	ID        string    `json:"id,omitempty" db:"id"`
	UserID    string    `json:"user_id" db:"user_id"`
	UserAgent string    `json:"user_agent" db:"user_agent"`
	IP        string    `json:"ip" db:"ip"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

func NewClient(userID string) *Client {
	return &Client{
		ID:     uuid.New().String(),
		UserID: userID,
	}
}

func (c *Client) Save(db *sqlx.DB) error {
	return nil
}
