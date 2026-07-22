package fetchers

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	PIKABU_URL             = "https://pikabu.ru/tag/%D0%A5%D0%B0%D0%BB%D1%8F%D0%B2%D0%B0/hot?cl=steam"
	EPIC_GAMES_URL         = "https://beebom.com/epic-games-free-games-list/"
	FREE_GAME_FINDINGS_URL = "https://old.reddit.com/r/FreeGameFindings/new/"
)

type Fetch struct {
	Id    int64
	Links []Link
}

type Link struct {
	Link  string
	Title string
	Date  time.Time
}

type FetchStorage interface {
	StoreFetch() (int64, error)
	StoreBody(fetchId int64, body string) error
	StoreError(fetchId int64, errorStr string) error
	DeleteFetch(id int64) error
	DeleteFetchesOlderThan(deadline time.Time) (int64, error)
	StorePost(fetch_id int64, link, title string, postedAt time.Time) error
}

type FreeGameFindingsFetcher struct {
	url        string
	httpClient *http.Client
	storage    FetchStorage
}

func NewFreeGameFindingsFetcher(url string, httpClient *http.Client, storage FetchStorage) FreeGameFindingsFetcher {
	return FreeGameFindingsFetcher{
		url,
		httpClient,
		storage,
	}
}

func (f FreeGameFindingsFetcher) Fetch(sinceTime time.Time) (Fetch, error) {
	fetchId, err := f.storage.StoreFetch()
	if err != nil {
		return Fetch{}, fmt.Errorf("Error storing a fetch: %w", err)
	}

	req, err := http.NewRequest("GET", f.url, nil)
	if err != nil {
		if err := f.storage.StoreError(fetchId, err.Error()); err != nil {
			return Fetch{}, fmt.Errorf("Error storing error for fetch '%d': %w", fetchId, err)
		}
		return Fetch{}, fmt.Errorf("Error making request: %w", err)
	}

	// Emulate a standard Chrome browser request
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Cache-Control", "max-age=0")
	req.Header.Set("Sec-Ch-Ua", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	res, err := f.httpClient.Do(req)
	if err != nil {
		if err := f.storage.StoreError(fetchId, err.Error()); err != nil {
			return Fetch{}, fmt.Errorf("Error storing error for fetch '%d': %w", fetchId, err)
		}
		return Fetch{}, fmt.Errorf("Error making request to Free Game Findings: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return Fetch{}, fmt.Errorf("status code error: %d %s", res.StatusCode, res.Status)

	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return Fetch{}, fmt.Errorf("Error reading body: %w", err)
	}

	err = f.storage.StoreBody(fetchId, string(body))
	if err != nil {
		if err := f.storage.StoreError(fetchId, err.Error()); err != nil {
			return Fetch{}, fmt.Errorf("Error storing error for fetch '%d': %w", fetchId, err)
		}
		return Fetch{}, fmt.Errorf("Error storing body for fetch '%d': %w", fetchId, err)
	}

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		if err := f.storage.StoreError(fetchId, err.Error()); err != nil {
			return Fetch{}, fmt.Errorf("Error storing error for fetch '%d': %w", fetchId, err)
		}
		return Fetch{}, fmt.Errorf("Error reading response body from Free Game Findings: %w", err)
	}

	links := []Link{}
	doc.
		Find("div#siteTable > :not(.promotedlink, .linkflair-modpost, .linkflair-Expired)").
		Children().
		Find(".top-matter").
		EachWithBreak(func(i int, div *goquery.Selection) bool {
			tagline := div.Find("p.tagline")
			datetime, _ := tagline.Find("time").Attr("datetime")

			date, err := time.Parse(time.RFC3339, datetime)
			if err != nil {
				log.Println(err)
				return false
			}
			if !date.UTC().After(sinceTime) {
				return false
			}
			// fmt.Println("Game:")
			// fmt.Println(date)

			title := div.Find("p.title")
			href, _ := title.Find("a").Attr("href")

			link := Link{href, "", date}
			links = append(links, link)

			err = f.storage.StorePost(fetchId, link.Link, link.Title, link.Date)
			if err != nil {
				log.Println(err)
			}

			return true
		})

	if len(links) == 0 {
		err := f.storage.DeleteFetch(fetchId)
		if err != nil {
			log.Println(fmt.Errorf("Error deleting fetch id '%d': %w", fetchId, err))
		}
	}
	return Fetch{fetchId, links}, nil
}

type EpicGamesFetcher struct {
	httpClient *http.Client
}

func (f EpicGamesFetcher) Fetch(sinceTime time.Time) ([]string, error) {
	res, err := f.httpClient.Get(EPIC_GAMES_URL)
	if err != nil {
		return []string{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return []string{}, err
	}

	links := []string{}
	doc.Find("figure.wp-block-table").Children().Find("tbody").Children().Each(func(i int, tr *goquery.Selection) {
		td_games := tr.Children().First()
		// td_dates := td_games.Next().Text()
		// start := strings.Split(td_dates, "-")[0]
		// game_start := time.

		hrefs := td_games.Find("a").Map(func(i int, link *goquery.Selection) string {
			result, _ := link.Attr("href")
			return result
		})
		links = append(links, hrefs...)
	})
	return links[:5], nil
}

type PikabuFetcher struct {
	httpClient *http.Client
}

func (pf PikabuFetcher) Fetch(sinceTime time.Time) ([]string, error) {
	res, err := pf.httpClient.Get(PIKABU_URL)
	if err != nil {
		return []string{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return []string{}, err
	}

	links := []string{}
	doc.Find("article").Each(func(i int, s *goquery.Selection) {
		datetime, exists := s.Find(".caption.story__datetime.hint").Attr("datetime")
		if !exists {
			log.Println("attribute datetime not found")
			return
		}

		date, err := time.Parse(time.RFC3339, datetime)
		if err != nil {
			log.Println(err)
			return
		}
		if !date.After(sinceTime) {
			return
		}

		title := s.Find(".story__title")
		link, exists := title.Find("a").Attr("href")
		log.Println(link)
		if !exists {
			log.Println("attribute href not found")
			return
		}

		// TODO: pagination
		links = append(links, link)
	})

	sort.Sort(sort.Reverse(sort.StringSlice(links)))
	return links, nil
}
