package scheduler

import (
	"context"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/player-monitor/backend/internal/crawler"
	"github.com/player-monitor/backend/pkg/logger"
)

type Scheduler struct {
	scheduler *gocron.Scheduler
	crawler   *crawler.Crawler
	interval  time.Duration
}

func New(crawler *crawler.Crawler, intervalHours int) (*Scheduler, error) {
	s := gocron.NewScheduler(time.UTC)
	s.SingletonModeAll()

	interval := time.Duration(intervalHours) * time.Hour
	if interval < 1*time.Hour {
		interval = 24 * time.Hour
	}

	return &Scheduler{
		scheduler: s,
		crawler:   crawler,
		interval:  interval,
	}, nil
}

func (s *Scheduler) Start(ctx context.Context) error {
	log := logger.Log

	s.scheduler.Every(s.interval).Do(func() {
		log.Info().Msg("starting scheduled scan")
		s.crawler.ScanAllSites(ctx)
	})

	s.scheduler.StartAsync()
	log.Info().Dur("interval", s.interval).Msg("scheduler started")

	return nil
}

func (s *Scheduler) Stop() {
	s.scheduler.Stop()
	logger.Log.Info().Msg("scheduler stopped")
}
