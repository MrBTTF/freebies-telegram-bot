package db

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type Scanable interface {
	Scan(dest ...any) error
}

type SqliteStorage struct {
	db *sql.DB
}

func NewStorage(db *sql.DB) *SqliteStorage {
	return &SqliteStorage{
		db: db,
	}
}
