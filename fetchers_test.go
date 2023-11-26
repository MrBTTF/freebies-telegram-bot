package main

import (
	"fmt"
	"log"
	"testing"
	"time"
)

func Test_EpicGamesFetcher(t *testing.T) {
	fetcher := EpicGamesFetcher{}
	sinceTime := time.Now().Add(24 * 3 * time.Hour)
	links, err := fetcher.Fetch(sinceTime)
	if err != nil {
		log.Fatalf("status code error: %s", err.Error())
	}

	_ = links
}

func Test_FreeGameFindingsFetcher(t *testing.T) {
	fetcher := FreeGameFindingsFetcher{}
	sinceTime := time.Now().UTC().Add(-24 * 3 * time.Hour)
	links, err := fetcher.Fetch(sinceTime)
	if err != nil {
		log.Fatalf("status code error: %s", err.Error())
	}
	fmt.Println(links)
	_ = links
}
