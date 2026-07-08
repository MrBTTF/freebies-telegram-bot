package db

import (
	"fmt"
	"time"
)

type Fetch struct {
	Id        int64
	CreatedAt time.Time
	Receivers string
	Payload   string
	Error     string
}

const InsertFetchQuery = `
INSERT INTO fetch_logs DEFAULT VALUES
`

func (s *Storage) StoreFetch() (int64, error) {
	result, err := s.db.Exec(InsertFetchQuery)
	if err != nil {
		return 0, fmt.Errorf("Unable to store fetch: %w", err)
	}

	fetchId, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("Unable to store fetch: %w", err)
	}

	return fetchId, nil
}

const UpdateFetchBodyQuery = `
UPDATE fetch_logs SET body = ? where id = ?
`

func (s *Storage) StoreBody(fetchId int64, body string) error {
	_, err := s.db.Exec(UpdateFetchBodyQuery, body, fetchId)
	if err != nil {
		return fmt.Errorf("Unable to store fetch: %w", err)
	}
	return nil
}

const UpdateFetchErrorQuery = `
UPDATE fetch_logs SET error = ? where id = ?
`

func (s *Storage) StoreError(fetchId int64, errorStr string) error {
	_, err := s.db.Exec(UpdateFetchErrorQuery, errorStr, fetchId)
	if err != nil {
		return fmt.Errorf("Unable to store fetch: %w", err)
	}
	return nil
}
