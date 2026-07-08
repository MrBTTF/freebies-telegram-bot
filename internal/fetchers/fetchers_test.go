package fetchers

import (
	"fmt"
	"log"
	"testing"
	"time"
)

// TODO: Setup tests
// Add tests/reddit.html with snapshot of reddit page
// Add tests/db.sqlite3 with test data: subscribers, links, posts
// or create migrations in tests/ and create a fresh db for each test

func Test_FreeGameFindingsFetcher(t *testing.T) {
	fetcher := FreeGameFindingsFetcher{}
	sinceTime := time.Now().UTC().Add(-24 * 3 * time.Hour)
	fetch, err := fetcher.Fetch(sinceTime)
	if err != nil {
		log.Fatalf("status code error: %s", err.Error())
	}
	fmt.Println(fetch.Links)
}
