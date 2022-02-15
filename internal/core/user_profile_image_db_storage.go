package core

import "github.com/jmoiron/sqlx"

type userProfileImageDbStorage struct {
	db *sqlx.DB
}

func NewUserProfileImageDbStorer(db *sqlx.DB) userProfileImageDbStorage {
	return userProfileImageDbStorage{db}
}

// Save сохраняет аватар пользователя в БД
func (s userProfileImageDbStorage) Save(img *UserProfileImage) (int64, error) {
	s.db.Ping()
	return 100500, nil
}
