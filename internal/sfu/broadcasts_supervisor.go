package sfu

import (
	"fmt"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

type BroadcastsSupervisor struct {
	Broadcasts map[string]*Broadcast
	mutex      sync.RWMutex
	db         *sqlx.DB
	rdb        *redis.Client
}

func NewBroadcastsSupervisor(db *sqlx.DB, rdb *redis.Client) *BroadcastsSupervisor {
	return &BroadcastsSupervisor{
		Broadcasts: make(map[string]*Broadcast),
		db:         db,
		rdb:        rdb,
	}
}

func (s *BroadcastsSupervisor) CreateBroadcast(req *BroadcastRequest) error {
	broadcast, err := NewBroadcast(
		uuid.NewString(),
		req.UserID,
		req.Title,
		req.Sdp,
	)
	if err != nil {
		return err
	}
	// FIXME: init broadcast in background
	if err := broadcast.Start(s.db, s.rdb); err != nil {
		return err
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	s.Broadcasts[broadcast.ID] = broadcast

	return nil
}

func (s *BroadcastsSupervisor) AddViewer(broadcastID string, req *ViewerRequest) error {
	viewer, err := NewViewer(
		uuid.NewString(),
		req.UserID,
		req.Sdp,
	)
	if err != nil {
		return err
	}
	broadcast, err := s.getBroadcast(broadcastID)
	if err != nil {
		return err
	}
	if err := broadcast.addViewer(viewer); err != nil {
		return err
	}

	if err := viewer.Start(s.rdb); err != nil {
		return err
	}

	return nil
}

func (s *BroadcastsSupervisor) getBroadcast(broadcastID string) (*Broadcast, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	broadcast, ok := s.Broadcasts[broadcastID]
	if !ok {
		return nil, fmt.Errorf("Not found broadcast via ID: %v", broadcastID)
	}
	return broadcast, nil
}
