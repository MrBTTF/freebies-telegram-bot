package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"syscall"

	cloudflarebp "github.com/DaRealFreak/cloudflare-bp-go"
	"github.com/go-faster/errors"

	"github.com/freebies-telegram-bot/internal/bot"
	"github.com/freebies-telegram-bot/internal/db"
	"github.com/freebies-telegram-bot/internal/fetchers"
	"github.com/freebies-telegram-bot/internal/worker"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "modernc.org/sqlite"
)

const WeeklyCron = "0 0 0 * * 1"

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

func setupDB(dbPath string) (*sql.DB, error) {
	conn, err := sql.Open("sqlite", "file:"+dbPath+"/db.sqlite3?_pragma=journal_mode(wal)&_pragma=busy_timeout(10000)")
	if err != nil {
		return nil, errors.Wrap(err, "failed to open db connection")
	}
	conn.SetMaxOpenConns(4)
	conn.SetMaxIdleConns(1)
	log.Println("Using db at " + dbPath)
	return conn, nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	dbPath, ok := os.LookupEnv("DB_PATH")
	if !ok {
		dbPath = "./db"
	}

	conn, err := setupDB(dbPath)
	if err != nil {
		log.Panic(err)
	}
	defer conn.Close()

	httpClient := &http.Client{}
	httpClient.Transport = cloudflarebp.AddCloudFlareByPass(httpClient.Transport)

	storage := db.NewStorage(conn)

	bot, err := bot.NewBot(storage, fetchers.NewFreeGameFindingsFetcher(fetchers.FREE_GAME_FINDINGS_URL, httpClient, storage))
	if err != nil {
		log.Panic(err)
	}

	logsCleaner, err := worker.NewLogsCleaner(storage)
	if err != nil {
		log.Panic(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go setupServer(bot, storage)
	go bot.WatchNewPosts(ctx)

	if err = logsCleaner.Start(WeeklyCron); err != nil {
		log.Panic(err)
	}

	bot.Run()

	logsCleaner.Stop(ctx)

}
