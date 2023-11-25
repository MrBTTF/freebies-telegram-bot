package main

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

var rnd = rand.New(rand.NewSource(time.Now().UnixNano()))

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
		chatIDStr := r.FormValue("chat_id")
		if chatIDStr != "" {
			chatID, err := strconv.Atoi(chatIDStr)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(err.Error()))
				return
			}
			log.Println("Message sent to ", chatID)
			bot.SendMsg(int64(chatID), message)
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
		log.Println("Using db at " + dbPath)
	}

	db, err := sql.Open("sqlite3", dbPath + "/db.sqlite3")
	if err != nil {
		log.Panic(err)
	}

	storage := NewStorage(db)

	bot, err := NewBot(storage, FreeGameFindingsFetcher{})
	if err != nil {
		log.Panic(err)
	}

	go setupServer(bot, storage)
	go bot.WatchNewPosts()

	bot.Run()
}
