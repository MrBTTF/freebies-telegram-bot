package main

import (
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"

	cloudflarebp "github.com/DaRealFreak/cloudflare-bp-go"
	"github.com/PuerkitoBio/goquery"
)

const (
	PIKABU_URL             = "https://pikabu.ru/tag/%D0%A5%D0%B0%D0%BB%D1%8F%D0%B2%D0%B0/hot?cl=steam"
	EPIC_GAMES_URL         = "https://beebom.com/epic-games-free-games-list/"
	FREE_GAME_FINDINGS_URL = "https://old.reddit.com/r/FreeGameFindings/new/"
)

type FreeGameFindingsFetcher struct {
}

func (f FreeGameFindingsFetcher) Fetch(sinceTime time.Time) ([]Link, error) {
	client := &http.Client{}
	client.Transport = cloudflarebp.AddCloudFlareByPass(client.Transport)

	res, err := client.Get(FREE_GAME_FINDINGS_URL)
	if err != nil {
		return []Link{}, fmt.Errorf("Error making request to Free Game Findings: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		return []Link{}, fmt.Errorf("Error reading response body from Free Game Findings: %w", err)
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

			links = append(links, Link{href, date})

			return true
		})
	return links, nil
}

type EpicGamesFetcher struct {
}

func (f EpicGamesFetcher) Fetch(sinceTime time.Time) ([]string, error) {
	client := &http.Client{}
	client.Transport = cloudflarebp.AddCloudFlareByPass(client.Transport)
	res, err := client.Get(EPIC_GAMES_URL)
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
}

func (pf PikabuFetcher) Fetch(sinceTime time.Time) ([]string, error) {
	client := &http.Client{}
	client.Transport = cloudflarebp.AddCloudFlareByPass(client.Transport)
	res, err := client.Get(PIKABU_URL)
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
