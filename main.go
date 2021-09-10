package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const (
	WEB_URL   = "https://pikabu.ru/tag/%D0%A5%D0%B0%D0%BB%D1%8F%D0%B2%D0%B0/hot?cl=steam"
)

type PikabuFetcher struct{

}

func (pf PikabuFetcher) Fetch(sinceTime time.Time) ([]string, error) {
	res, err := http.Get(WEB_URL)
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

func setupServer(bot *Bot, storage *Storage) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Pinged")
		fmt.Fprintf(w, "Hehehe")
	})

	http.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		message := r.FormValue("message")
		subscribers, err := storage.ReadSubscribers()
		if err != nil {
			log.Println(err)
			w.Write([]byte(err.Error()))
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for _, s := range subscribers {
			log.Println("Message sent to ", s.ChatID)
			bot.SendMsg(s.ChatID, message)
		}
	})

	port, ok := os.LookupEnv("PORT")
	if !ok {
		port = "8080"
	}

	fmt.Printf("Running server at port %s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	client, err := getGoogleClient()
	if err != nil {
		log.Panic(err)
	}

	storage := NewStorage(client)

	bot, err := NewBot(storage, PikabuFetcher{})
	if err != nil {
		log.Panic(err)
	}

	go setupServer(bot, storage)
	go bot.WatchNewPosts()

	bot.Run()
}
