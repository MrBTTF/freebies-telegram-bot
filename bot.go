package main

import (
	"database/sql"
	_ "embed"
	"errors"
	"log"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var (
	//go:embed keys/telegram_token.txt
	ApiToken string
)

type Link struct {
	link string
	date time.Time
}

type LinksFetcher interface {
	Fetch(time.Time) ([]Link, error)
}

type Bot struct {
	botApi  *tgbotapi.BotAPI
	storage *Storage
	links   LinksFetcher
}

func NewBot(storage *Storage, links LinksFetcher) (*Bot, error) {
	bot, err := tgbotapi.NewBotAPI(ApiToken)
	if err != nil {
		return nil, err
	}

	// bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	return &Bot{
		botApi:  bot,
		storage: storage,
		links:   links,
	}, nil
}

func (b *Bot) Run() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.botApi.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { // ignore any non-Message Updates
			continue
		}
		chatID := int64(update.Message.Chat.ID)

		now := time.Now().UTC().Truncate(24 * time.Hour)

		switch update.Message.Command() {
		case "start":
			err := b.storage.StoreSubscriber(chatID, now)
			if err != nil {
				log.Println(err)
			}

			b.SendMsgWithMarkdown(update.Message.Chat.ID, "Hey\\! I'll be posting new freebies from steam community on pikabu\\.ru\\. Type _*/*_ to see the list of commands\\. 🙂")
			b.SendPostsToUser(update.Message.Chat.ID, 0)

		case "today":
			b.SendPostsToUser(update.Message.Chat.ID, 1)
		case "yesterday":
			b.SendPostsToUser(update.Message.Chat.ID, 2)
		case "week":
			b.SendPostsToUser(update.Message.Chat.ID, 8)
		case "month":
			b.SendPostsToUser(update.Message.Chat.ID, 31)
		case "receive":
			_, err := b.storage.GetSubscriber(int(chatID))
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				log.Println(err)
			} else if !errors.Is(err, sql.ErrNoRows) {
				err = b.storage.DeleteSubscriber(int(chatID))
				if err != nil {
					log.Println(err)
				}
				err = b.SendMsg(update.Message.Chat.ID, "I won't be posting new freebies anymore. 😐")
				if err != nil {
					log.Println(err)
				}
			} else {
				err = b.storage.StoreSubscriber(chatID, now)
				if err != nil {
					log.Println(err)
				}
				err = b.SendMsg(update.Message.Chat.ID, "I'll be posting new freebies from now on as soon as I find some. 😉")
				if err != nil {
					log.Println(err)
				}
			}
		case "":
		default:
			err := b.SendMsgWithMarkdown(update.Message.Chat.ID, "Unknown command 🧐\\. Type _*/*_")
			if err != nil {
				log.Println(err)
			}
		}
	}
}

func (b *Bot) WatchNewPosts() {
	for {
		subscribers, err := b.storage.ReadSubscribers()
		if err != nil {
			log.Println(err)
			continue
		}
		now := time.Now().UTC()

		earlierstLastPost := now
		for _, s := range subscribers {
			if earlierstLastPost.After(s.LastPost) {
				earlierstLastPost = s.LastPost
			}
		}
		allLinks, err := b.links.Fetch(earlierstLastPost)
		if err != nil {
			log.Println(err)
			return
		}
		log.Printf("Fetched %d posts in total", len(allLinks))

		var wg sync.WaitGroup
		for _, s := range subscribers {
			s := s
			wg.Add(1)
			go func() {
				defer wg.Done()

				links := getLinksAfter(allLinks, s.LastPost)
				if err != nil {
					log.Println(err)
					return
				}
				if len(links) != 0 {
					b.SendMsg(s.ChatID, "Just found some new freebies for you 😉")
				}
				for _, link := range links {
					b.SendMsg(s.ChatID, link.link)
				}
				log.Printf("%d posts send to subscriber: %d", len(links), s.ChatID)
				if len(links) != 0 {
					err = b.storage.UpdateLastPost(s.ChatID, now)
					if err != nil {
						log.Println(err)
						return
					}
				}

			}()
		}
		time.Sleep(time.Duration(rnd.Intn(60*4)+60) * time.Second)
		wg.Wait()
	}
}

func (b *Bot) SendMsg(chatId int64, message string) error {
	msg := tgbotapi.NewMessage(chatId, message)
	_, err := b.botApi.Send(msg)
	return err
}

func (b *Bot) SendMsgWithMarkdown(chatId int64, message string) error {
	msg := tgbotapi.NewMessage(chatId, message)
	msg.ParseMode = "MarkdownV2"
	msg.DisableWebPagePreview = true
	_, err := b.botApi.Send(msg)
	return err
}

func (b *Bot) SendPostsToUser(chatID int64, sinceDays int) {
	sinceTime := time.Now().UTC().AddDate(0, 0, -sinceDays)
	links, err := b.links.Fetch(sinceTime)
	if err != nil {
		log.Println(err)
	}
	if len(links) == 0 {
		if sinceDays == 0 {
			b.SendMsg(chatID, "No freebies for today 😕")
		} else {
			b.SendMsg(chatID, "No freebies so far 😕")
		}
	} else {
		b.SendMsg(chatID, "Here are some freebies for you 😉")
	}
	for _, link := range links {
		b.SendMsg(chatID, link.link)
	}
	log.Printf("%d posts send to subscriber: %d", len(links), chatID)
}

func getLinksAfter(links []Link, date time.Time) []Link {
	result := make([]Link, 0, len(links))
	for _, link := range links {
		if link.date.After(date) {
			result = append(result, link)
		}
	}

	return result
}
