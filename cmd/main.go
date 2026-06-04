package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"

	"github.com/freebies-telegram-bot/internal/bot"
	"github.com/freebies-telegram-bot/internal/db"
	"github.com/freebies-telegram-bot/internal/fetchers"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var markdownRe = regexp.MustCompile(`([!\(\).])`)

func sendMessage(bot *bot.Bot, chatId int, message string) error {
	log.Println("Message sent to ", chatId)
	message = markdownRe.ReplaceAllString(message, `\$1`)
	return bot.SendMsgWithMarkdown(int64(chatId), message)
}

func setupServer(bot *bot.Bot, storage *db.Storage) {
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "OK")
	})

	http.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		message := r.FormValue("message")
		chatIDStr := r.FormValue("chat_id")
		if chatIDStr != "" {
			chatID, err := strconv.Atoi(chatIDStr)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(err.Error()))
				return
			}
			if chatID == -1 {
				subscribers, err := storage.ReadSubscribers()
				if err != nil {
					w.WriteHeader(http.StatusBadRequest)
					w.Write([]byte(err.Error()))
					return
				}
				for _, s := range subscribers {
					err = sendMessage(bot, int(s.ChatID), message)
					if err != nil {
						log.Printf("Unable to send message to %d:%s\n", s.ChatID, err.Error())
					}
				}
			} else {
				err = sendMessage(bot, int(chatID), message)
				if err != nil {
					log.Printf("Unable to send message to %d:%s\n", chatID, err.Error())
				}
			}
			return
		}

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
	dbPath, ok := os.LookupEnv("DB_PATH")
	if !ok {
		dbPath = "./db"
	}

	conn, err := sql.Open("sqlite3", dbPath+"/db.sqlite3")
	if err != nil {
		log.Panic(err)
	}
	log.Println("Using db at " + dbPath)

	storage := db.NewStorage(conn)

	bot, err := bot.NewBot(storage, fetchers.FreeGameFindingsFetcher{})
	if err != nil {
		log.Panic(err)
	}

	go setupServer(bot, storage)
	go bot.WatchNewPosts()

	bot.Run()
}
