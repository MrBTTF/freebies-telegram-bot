package db

import (
	"database/sql"

	_ "modernc.org/sqlite"
)

type Scanable interface {
	Scan(dest ...any) error
}

type Storage struct {
	db *sql.DB
}

func NewStorage(db *sql.DB) *Storage {
	return &Storage{
		db: db,
	}
}
