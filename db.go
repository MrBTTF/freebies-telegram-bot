package main

import (
	"fmt"
	"time"

	"database/sql"

	_ "github.com/mattn/go-sqlite3"
)

type Scanable interface {
	Scan(dest ...any) error
}

type Subscriber struct {
	ChatID   int64
	LastPost time.Time
}

type Storage struct {
	db *sql.DB
}

func NewStorage(db *sql.DB) *Storage {
	return &Storage{
		db: db,
	}
}

const InsertSubscriberQuery = `
INSERT OR IGNORE INTO subscribers(chat_id, last_post) values(?,?)
`

func (s *Storage) StoreSubscriber(chatId int64, sinceTime time.Time) error {
	sinceTimeStr := sinceTime.Format(time.RFC3339)
	_, err := s.db.Exec(InsertSubscriberQuery, chatId, sinceTimeStr)

	if err != nil {
		return fmt.Errorf("Unable to store last post %s for chat_id %d: %w", sinceTimeStr, chatId, err)
	}

	return nil
}

const UpdateLastPostQuery = `
UPDATE subscribers SET last_post = ? where chat_id = ?
`

func (s *Storage) UpdateLastPost(chatId int64, sinceTime time.Time) error {
	sinceTimeStr := sinceTime.Format(time.RFC3339)

	_, err := s.db.Exec(UpdateLastPostQuery, sinceTimeStr, chatId)

	if err != nil {
		return fmt.Errorf("Unable to store last post %s for chat_id %d: %w", sinceTimeStr, chatId, err)
	}

	return nil
}

const DeleteSubscriberQuery = `
DELETE FROM subscribers WHERE chat_id = ?
`

func (s *Storage) DeleteSubscriber(chatId int) error {
	_, err := s.db.Exec(DeleteSubscriberQuery, chatId)
	if err != nil {
		return fmt.Errorf("Unable to delete subscriber for chat_id %d: %w", chatId, err)
	}

	return nil
}

const SelectSubscribersQuery = `
SELECT chat_id, last_post FROM subscribers
`

func (s *Storage) ReadSubscribers() ([]Subscriber, error) {
	rows, err := s.db.Query(SelectSubscribersQuery)
	if err != nil {
		return nil, fmt.Errorf("Unable to read subscribers: %w", err)
	}
	defer rows.Close()

	var subscribers []Subscriber
	for rows.Next() {
		subscriber, err := scanSubscriber(rows)
		if err != nil {
			return nil, fmt.Errorf("Unable to read subscribers: %w", err)
		}

		subscribers = append(subscribers, subscriber)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("Unable to read subscribers: %w", err)
	}

	return subscribers, nil
}

const SelectSubscriberQuery = `
SELECT chat_id, last_post FROM subscribers
WHERE chat_id = ?
`

func (s *Storage) GetSubscriber(chatId int) (Subscriber, error) {
	row := s.db.QueryRow(SelectSubscriberQuery, chatId)
	subscriber, err := scanSubscriber(row)
	if err != nil {
		return Subscriber{}, fmt.Errorf("Unable to get subscriber for chat_id %d: %w", chatId, err)
	}

	return subscriber, nil
}

func scanSubscriber(row Scanable) (Subscriber, error) {
	var subscriber Subscriber
	var lastPostStr string

	err := row.Scan(
		&subscriber.ChatID,
		&lastPostStr,
	)
	if err != nil {
		return Subscriber{}, fmt.Errorf("Unable to scan subscriber: %w", err)
	}

	date, err := time.Parse(time.RFC3339, lastPostStr)
	if err != nil {
		return Subscriber{}, fmt.Errorf("Unable to scan subscriber: %w", err)
	}
	subscriber.LastPost = date

	if lastPostStr == "" {
		subscriber.LastPost = time.Now()
	}
	return subscriber, nil
}
