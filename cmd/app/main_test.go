package main

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/freebies-telegram-bot/internal/bot"
	"github.com/freebies-telegram-bot/internal/db"
	"github.com/freebies-telegram-bot/internal/fetchers"
	"github.com/freebies-telegram-bot/internal/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed reddit.html
var testRedditPage string

var conn *sql.DB
var storage *db.SqliteStorage

func TestMain(m *testing.M) {
	var err error
	conn, err = setupDB("../../tests/")
	if err != nil {
		log.Fatalf("test setup failed: %s", err.Error())
	}
	defer func() {
		conn.Close()
		os.Remove("../../tests/db.sqlite3")
		os.Remove("../../tests/db.sqlite3-journal")
	}()

	query, err := os.ReadFile("../../db/migrations/create_tables.sql")
	if err != nil {
		log.Fatalf("test setup failed: %s", err.Error())
	}
	_, err = conn.Exec(string(query))
	if err != nil {
		log.Fatalf("test setup failed: %s", err.Error())
	}
	_, err = conn.Exec(`INSERT INTO subscribers(chat_id,last_post) VALUES(356021914,'2026-07-09T15:15:00Z')`)
	if err != nil {
		log.Fatalf("test setup failed: %s", err.Error())
	}

	storage = db.NewStorage(conn)

	code := m.Run()

	err = conn.Close()
	if err != nil {
		log.Fatalf("test teardown failed: %s", err.Error())
	}
	err = os.Remove("../../tests/db.sqlite3")
	if err != nil {
		log.Fatalf("test teardown failed: %s", err.Error())
	}

	os.Exit(code)
}

func TestWatchNewPosts(t *testing.T) {
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, testRedditPage)
	}))
	defer httpServer.Close()

	mockClient := &http.Client{}

	tgBot, err := bot.NewBot(storage, fetchers.NewFreeGameFindingsFetcher(httpServer.URL, mockClient, storage))
	if err != nil {
		log.Fatalf("test setup failed: %s", err.Error())
	}

	ctx, cancelWatchNewPosts := context.WithCancel(context.Background())
	defer cancelWatchNewPosts()

	go tgBot.WatchNewPosts(ctx)

	testCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for {
		select {
		case <-testCtx.Done():
			t.Error("Timeout")
			return
		default:
		}
		row := conn.QueryRowContext(testCtx, `SELECT body, error FROM fetch_logs WHERE id = 1`)

		var body, fetchError *string
		err := row.Scan(&body, &fetchError)
		if err == sql.ErrNoRows ||
			(err != nil && strings.Contains(err.Error(), "database is locked")) ||
			body == nil {
			continue
		}

		cancelWatchNewPosts()

		require.NoError(t, err)
		assert.Equal(t, testRedditPage, *body)
		assert.Empty(t, fetchError)

		post, err := storage.GetPostByLink("testing-link")
		if errors.Is(err, sql.ErrNoRows) ||
			(err != nil && strings.Contains(err.Error(), "database is locked")) {
			continue
		}
		require.NoError(t, err)
		assert.Equal(t, int64(1), post.FetchId)

		deliveredPost, err := storage.GetDeliveredPost(post.Id, 356021914)
		if errors.Is(err, sql.ErrNoRows) ||
			(err != nil && strings.Contains(err.Error(), "database is locked")) {
			continue
		}
		require.NoError(t, err)
		assert.Equal(t, int64(356021914), deliveredPost.Receiver)

		break
	}

}

func TestLogsCleaner(t *testing.T) {
	fetchLogsIds := []int64{}
	postsIds := []int64{}
	for i := 1; i <= 10; i += 1 {
		deadline := time.Now().Add(-time.Duration(30*24+i*24) * time.Hour)
		fetchId := insertValues(t, "fetch_logs(created_at) VALUES(?)", deadline)
		fetchLogsIds = append(fetchLogsIds, fetchId)
		postId := insertValues(t, "posts(created_at,fetch_id,link,posted_at) VALUES(?,0,?,'')", deadline, strconv.Itoa(i))
		postsIds = append(postsIds, postId)
		insertValues(t, "delivered_posts(delivery_date,post_id,receiver) VALUES(?,0,0)", deadline)
	}

	logsCleaner, err := worker.NewLogsCleaner(storage)
	if err != nil {
		log.Panic(err)
	}

	err = logsCleaner.Start("*/1 * * * * *")
	require.NoError(t, err)

	testCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for {
		select {
		case <-testCtx.Done():
			t.Error("Timeout")
			return
		default:
		}
		if rowsFound(testCtx, "SELECT id FROM fetch_logs WHERE id >= ?", fetchLogsIds[0]) {
			continue
		}
		if rowsFound(testCtx, "SELECT id FROM posts WHERE id >= ?", postsIds[0]) {
			continue
		}
		if rowsFound(testCtx, "SELECT post_id FROM delivered_posts WHERE post_id >= ?", postsIds[0]) {
			continue
		}
		break
	}

	err = logsCleaner.Stop(context.Background())
	require.NoError(t, err)

}

func insertValues(t *testing.T, tableRows string, values ...any) int64 {
	result, err := conn.Exec(`INSERT INTO `+tableRows, values...)
	require.NoError(t, err)
	id, err := result.LastInsertId()
	require.NoError(t, err)
	return id
}

func rowsFound(ctx context.Context, query string, oldestId int64) bool {
	row := conn.QueryRowContext(ctx, query, oldestId)
	var id *int64
	err := row.Scan(&id)
	if err != sql.ErrNoRows {
		return true
	}
	return false
}
