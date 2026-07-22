package bot

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/freebies-telegram-bot/internal/db"
	"github.com/freebies-telegram-bot/internal/fetchers"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	linksRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "game_freebies_links_requests",
		Help: "The number of latest requests to fetch links",
	})
	fetchedRequests = promauto.NewCounter(prometheus.CounterOpts{
		Name: "game_freebies_links_fetched",
		Help: "The number of fetched links",
	})
	freebieDeliveries = promauto.NewCounter(prometheus.CounterOpts{
		Name: "game_freebies_freebie_deliveries",
		Help: "The number of latest freebie links delivered to users",
	})
	currentSubscribers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "game_freebies_current_subscribers",
		Help: "The current number of subscribers",
	})
)

var (
	//go:embed keys/telegram_token.txt
	ApiToken string

	rnd = rand.New(rand.NewSource(time.Now().UnixNano()))
)

type LinksFetcher interface {
	Fetch(time.Time) (fetchers.Fetch, error)
}

type BotStorage interface {
	GetPostByLink(link string) (db.Post, error)
	StoreDeliveredPost(postId, receiver int64) error
	GetDeliveredPost(postId, receiver int64) (db.DeliveredPost, error)
	StoreSubscriber(chatId int64, sinceTime time.Time) error
	UpdateLastPost(chatId int64, sinceTime time.Time) error
	DeleteSubscriber(chatId int) error
	ReadSubscribers() ([]db.Subscriber, error)
	GetSubscriber(chatId int) (db.Subscriber, error)
}

type Bot struct {
	botApi  *tgbotapi.BotAPI
	storage BotStorage
	links   LinksFetcher
}

func NewBot(storage BotStorage, links LinksFetcher) (*Bot, error) {
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
	u.Timeout = 60 * 5

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

func (b *Bot) WatchNewPosts(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		subscribers, err := b.storage.ReadSubscribers()
		if err != nil {
			log.Println(err)
			continue
		}
		currentSubscribers.Set(float64(len(subscribers)))

		now := time.Now().UTC()

		earlierstLastPost := now
		for _, s := range subscribers {
			if earlierstLastPost.After(s.LastPost) {
				earlierstLastPost = s.LastPost
			}
		}

		fetch, err := b.fetchLinks(earlierstLastPost)
		if err != nil {
			log.Println(err)
			continue
		}

		if len(fetch.Links) == 0 {
			time.Sleep(time.Duration(rnd.Intn(60*4)+60) * time.Second)
			continue
		}

		links := filterLinks(fetch.Links)

		var wg sync.WaitGroup
		for _, s := range subscribers {
			wg.Go(func() {

				links = getLinksAfter(links, s.LastPost)
				if err != nil {
					log.Println(err)
					return
				}
				links = b.filterDeliveredLinks(s.ChatID, fetch.Id, links)

				if len(links) == 0 {
					return
				}
				b.SendMsg(s.ChatID, "Just found some new freebies for you 😉")
				b.sendLinks(s.ChatID, links)
				err = b.storage.UpdateLastPost(s.ChatID, now)
				if err != nil {
					log.Println(err)
					return
				}

			})
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
	fetch, err := b.fetchLinks(sinceTime)
	if err != nil {
		log.Println(err)
	}
	if len(fetch.Links) == 0 {
		if sinceDays == 0 {
			b.SendMsg(chatID, "No freebies for today 😕")
		} else {
			b.SendMsg(chatID, "No freebies so far 😕")
		}
	} else {
		b.SendMsg(chatID, "Here are some freebies for you 😉")
		b.sendLinks(chatID, fetch.Links)
	}
}

func (b *Bot) fetchLinks(sinceTime time.Time) (fetchers.Fetch, error) {
	fetch, err := b.links.Fetch(sinceTime)
	if err != nil {
		return fetchers.Fetch{}, err
	}
	linksRequests.Add(1)
	if len(fetch.Links) != 0 {
		log.Printf("Fetched %d posts in total for %s", len(fetch.Links), sinceTime.String())
		fetchedRequests.Add(float64(len(fetch.Links)))
	}
	return fetch, nil
}

func (b *Bot) sendLinks(chatId int64, links []fetchers.Link) {
	for _, link := range links {
		b.SendMsg(chatId, link.Link)
	}
	log.Printf("%d posts send to subscriber: %d", len(links), chatId)
	freebieDeliveries.Add(float64(len(links)))
}

func (b *Bot) filterDeliveredLinks(chatId, fetchId int64, links []fetchers.Link) []fetchers.Link {
	filteredLinks := []fetchers.Link{}
	for _, link := range links {
		delivered, err := b.checkIfPostDelivered(chatId, link.Link, fetchId)
		if err != nil {
			log.Printf("Failed to check post for delivery for chatId '%d', link '%s': %s", fetchId, link.Link, err.Error())
		}
		if delivered {
			continue
		}

		filteredLinks = append(filteredLinks, link)
	}
	return filteredLinks
}

func (b *Bot) checkIfPostDelivered(chatId int64, link string, fetchId int64) (bool, error) {
	post, err := b.storage.GetPostByLink(link)
	if err != nil {
		return false, fmt.Errorf("Failed to get post post for fetch id '%d', link '%s': %w", fetchId, link, err)
	}

	deliveredPost, err := b.storage.GetDeliveredPost(post.Id, chatId)
	if err == nil {
		log.Printf("Skipping post, already delivered for post id '%d', chat id '%d', link '%s' on delivery date %s", post.Id, chatId, link, deliveredPost.DeliveryDate.String())
		return true, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return false, fmt.Errorf("Failed to get post delivered post for fetch id '%d', chatId '%d': %w", fetchId, chatId, err)
	}

	err = b.storage.StoreDeliveredPost(post.Id, chatId)
	if err != nil {
		return false, fmt.Errorf("Failed to store delivered post for fetch id '%d', chatId '%d': %w", fetchId, chatId, err)
	}

	return false, nil
}

func getLinksAfter(links []fetchers.Link, date time.Time) []fetchers.Link {
	result := make([]fetchers.Link, 0, len(links))
	for _, link := range links {
		if link.Date.After(date) {
			result = append(result, link)
		}
	}

	return result
}

func filterLinks(links []fetchers.Link) []fetchers.Link {
	filteredLinks := []fetchers.Link{}
	for _, link := range links {
		if isLinkAllowed(link.Link) {
			filteredLinks = append(filteredLinks, link)
		}
	}
	return filteredLinks
}

var rules = map[string]func(link string) bool{
	"skip_amazon": func(link string) bool {
		return !strings.Contains(link, "amazon.com")
	},
	"skip_reddit": func(link string) bool {
		return !strings.HasPrefix(link, "/r/")
	},
	"skip_x_com": func(link string) bool {
		return !strings.HasPrefix(link, "https://x.com")
	},
}

func isLinkAllowed(link string) bool {
	for name, isAllowed := range rules {
		if !isAllowed(link) {
			fmt.Printf("Link %s is filtered by rule %s\n", link, name)
			return false
		}
	}
	return true
}
