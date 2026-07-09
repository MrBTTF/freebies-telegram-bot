package db

import (
	"fmt"
	"time"
)

type Post struct {
	Id        int64
	FetchId   int64
	Link      string
	Title     string
	PostedAt  time.Time
	CreatedAt time.Time
}

const InsertPostQuery = `
INSERT INTO posts(fetch_id, link, title, posted_at) values(?,?,?,?)
ON CONFLICT(link) DO NOTHING
`

func (s *Storage) StorePost(fetch_id int64, link, title string, postedAt time.Time) error {
	_, err := s.db.Exec(InsertPostQuery, fetch_id, link, title, postedAt)
	if err != nil {
		return fmt.Errorf("Unable to store post for fetch id '%d', link '%s': %w", fetch_id, link, err)
	}
	return nil
}

const SelectPostByLinkQuery = `
SELECT id, fetch_id, link, title, posted_at, created_at FROM posts
WHERE link = ?
`

func (s *Storage) GetPostByLink(link string) (Post, error) {
	row := s.db.QueryRow(SelectPostByLinkQuery, link)

	var post Post
	err := row.Scan(
		&post.Id,
		&post.FetchId,
		&post.Link,
		&post.Title,
		&post.PostedAt,
		&post.CreatedAt,
	)
	if err != nil {
		return Post{}, fmt.Errorf("Unable to scan post for link '%s': %w", link, err)
	}

	return post, nil
}

type DeliveredPost struct {
	PostId       int64
	Link         string
	Receiver     string
	DeliveryDate time.Time
}

const InsertDeliveriedPostQuery = `
INSERT INTO delivered_posts(post_id, receiver) values(?,?)
`

func (s *Storage) StoreDeliveredPost(postId, receiver int64) error {
	_, err := s.db.Exec(InsertDeliveriedPostQuery, postId, receiver)
	if err != nil {
		return fmt.Errorf("Unable to store delivered post for post id '%d', receiver '%d': %w", postId, receiver, err)
	}
	return nil
}

const SelectDeliveredPostQuery = `
SELECT post_id, receiver, delivery_date FROM delivered_posts
WHERE post_id = ? AND receiver = ?
`

func (s *Storage) GetDeliveredPost(postId, receiver int64) (DeliveredPost, error) {
	row := s.db.QueryRow(SelectDeliveredPostQuery, postId, receiver)

	var deliveredPost DeliveredPost
	err := row.Scan(
		&deliveredPost.PostId,
		&deliveredPost.Receiver,
		&deliveredPost.DeliveryDate,
	)
	if err != nil {
		return DeliveredPost{}, fmt.Errorf("Unable to scan delivered post for post id '%d': %w", postId, err)
	}

	return deliveredPost, nil
}
