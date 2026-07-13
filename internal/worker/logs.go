package worker

import (
	"context"
	"log"
	"time"

	"github.com/freebies-telegram-bot/internal/db"
	"github.com/go-co-op/gocron/v2"
	"github.com/go-faster/errors"
)

var FetchRetention = 30 * 24

type LogsCleaner struct {
	scheduler gocron.Scheduler
	db        *db.Storage
}

func NewLogsCleaner(db *db.Storage) (LogsCleaner, error) {
	s, err := gocron.NewScheduler()
	if err != nil {
		return LogsCleaner{}, errors.Wrap(err, "failed to create cron scheduler")
	}
	return LogsCleaner{s, db}, nil
}

func (lc LogsCleaner) Start(cronSchedule string) error {
	_, err := lc.scheduler.NewJob(
		gocron.CronJob(cronSchedule, true),
		gocron.NewTask(lc.clean),
	)
	if err != nil {
		return errors.Wrap(err, "failed to create job ")
	}

	lc.scheduler.Start()

	return nil
}

func (lc LogsCleaner) Stop(ctx context.Context) error {
	err := lc.scheduler.ShutdownWithContext(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to shutdown cron scheduler")
	}
	return nil
}

func (lc LogsCleaner) clean() {
	deadline := time.Now().Add(-time.Duration(FetchRetention) * time.Hour)
	log.Printf("[LogsCleaner] cleaning fetch logs older than %s\n", deadline)
	fetchLogsCleaned, err := lc.db.DeleteFetchesOlderThan(deadline)
	if err != nil {
		log.Printf("failed clean fetches: %s", err.Error())
	}
	log.Printf("[LogsCleaner] cleaned %d fetch logs\n", fetchLogsCleaned)

	log.Printf("[LogsCleaner] cleaning posts older than %s\n", deadline)
	postsCleaned, err := lc.db.DeletePostsOlderThan(deadline)
	if err != nil {
		log.Printf("failed clean posts: %s", err.Error())
	}
	log.Printf("[LogsCleaner] cleaned %d posts logs\n", postsCleaned)

	log.Printf("[LogsCleaner] cleaning posts older than %s\n", deadline)
	deliveredPostsCleaned, err := lc.db.DeleteDeliveredPostsOlderThan(deadline)
	if err != nil {
		log.Printf("failed clean delivered posts: %s", err.Error())
	}
	log.Printf("[LogsCleaner] cleaned %d delivered posts logs\n", deliveredPostsCleaned)

}
