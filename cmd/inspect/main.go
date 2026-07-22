package main

import (
	"fmt"
	"log"
	"net/http"
	"time"

	cloudflarebp "github.com/DaRealFreak/cloudflare-bp-go"
	"github.com/alexflint/go-arg"
	"github.com/freebies-telegram-bot/internal/fetchers"
)

type SinceDateTime struct {
	time.Time
}

func (sdt *SinceDateTime) UnmarshalText(text []byte) error {
	input := string(text)

	// Define allowed formats from most specific to least specific
	layouts := []string{
		time.RFC3339, // "2006-01-02T15:04:05Z07:00"
		"2006-01-02", // "2006-01-02" (date-only)
	}

	var err error
	for _, layout := range layouts {
		var parsed time.Time
		parsed, err = time.Parse(layout, input)
		if err == nil {
			sdt.Time = parsed
			return nil // Success! Exit early
		}
	}

	// If all layouts fail, return a helpful consolidated error
	return fmt.Errorf("invalid time format %q: must match YYYY-MM-DD or RFC3339", input)
}

var args struct {
	Source string        `arg:"positional"`
	Since  SinceDateTime `arg:"-s,--since"`
}

type InspectStorage struct {
}

func (is *InspectStorage) StoreFetch() (int64, error) {
	return 0, nil
}

func (is *InspectStorage) StoreBody(fetchId int64, body string) error {
	return nil
}

func (is *InspectStorage) StoreError(fetchId int64, errorStr string) error {
	return nil
}

func (is *InspectStorage) DeleteFetch(id int64) error {
	return nil
}

func (is *InspectStorage) DeleteFetchesOlderThan(deadline time.Time) (int64, error) {
	return 0, nil
}

func (is *InspectStorage) StorePost(fetch_id int64, link string, title string, postedAt time.Time) error {
	return nil
}

func main() {
	arg.MustParse(&args)

	if args.Source == "" {
		args.Source = fetchers.FREE_GAME_FINDINGS_URL
	}

	httpClient := &http.Client{}
	httpClient.Transport = cloudflarebp.AddCloudFlareByPass(httpClient.Transport)

	storage := &InspectStorage{}

	fetcher := fetchers.NewFreeGameFindingsFetcher(args.Source, httpClient, storage)

	fetch, err := fetcher.Fetch(args.Since.Time)
	if err != nil {
		log.Fatalf("%s", err.Error())
	}

	fmt.Println("time\turl")

	for _, link := range fetch.Links {
		fmt.Printf("%s\t%s\n", link.Date, link.Link)
	}
}
