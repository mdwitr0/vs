package crawler

import (
	"context"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/player-monitor/backend/internal/config"
	"github.com/player-monitor/backend/internal/repo"
	"github.com/player-monitor/backend/pkg/logger"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Crawler struct {
	siteRepo     *repo.SiteRepo
	pageRepo     *repo.PageRepo
	settingsRepo *repo.SettingsRepo
	parser       *Parser
	detector     *Detector
	client       *http.Client
	cfg          *config.Config
}

func NewCrawler(
	siteRepo *repo.SiteRepo,
	pageRepo *repo.PageRepo,
	settingsRepo *repo.SettingsRepo,
	cfg *config.Config,
) *Crawler {
	client := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	return &Crawler{
		siteRepo:     siteRepo,
		pageRepo:     pageRepo,
		settingsRepo: settingsRepo,
		parser:       NewParser(client),
		detector:     NewDetector(),
		client:       client,
		cfg:          cfg,
	}
}

type crawlState struct {
	seen      map[string]bool
	queue     []string
	mu        sync.Mutex
	processed int
}

func (c *Crawler) ScanSite(ctx context.Context, site *repo.Site) {
	log := logger.Log

	if err := c.siteRepo.UpdateStatus(ctx, site.ID.Hex(), "scanning"); err != nil {
		log.Error().Err(err).Str("site_id", site.ID.Hex()).Msg("failed to update site status")
		return
	}

	settings, err := c.settingsRepo.GetByUserID(ctx, site.UserID)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch settings")
		c.siteRepo.UpdateStatus(ctx, site.ID.Hex(), "error")
		return
	}

	playerRegex, err := regexp.Compile(settings.PlayerPattern)
	if err != nil {
		log.Error().Err(err).Msg("invalid player pattern")
		c.siteRepo.UpdateStatus(ctx, site.ID.Hex(), "error")
		return
	}

	state := &crawlState{
		seen:  make(map[string]bool),
		queue: []string{},
	}

	sitemapURLs, err := c.parser.ParseSitemap(ctx, site.Domain)
	if err != nil {
		log.Warn().Err(err).Str("domain", site.Domain).Msg("failed to parse sitemap, starting from homepage")
		sitemapURLs = []string{"https://" + site.Domain}
	}

	for _, url := range sitemapURLs {
		if !state.seen[url] {
			state.seen[url] = true
			state.queue = append(state.queue, url)
		}
	}

	log.Info().Str("domain", site.Domain).Int("initial_urls", len(state.queue)).Msg("starting crawl")

	maxPages := c.cfg.CrawlMaxPages
	if maxPages == 0 {
		maxPages = 3000000
	}
	maxDepth := c.cfg.CrawlMaxDepth
	if maxDepth == 0 {
		maxDepth = 5
	}
	rateLimit := c.cfg.CrawlRateLimit
	if rateLimit == 0 {
		rateLimit = 500 * time.Millisecond
	}

	depth := 0
	for len(state.queue) > 0 && state.processed < maxPages && depth < maxDepth {
		currentBatch := state.queue
		state.queue = nil

		for _, url := range currentBatch {
			if ctx.Err() != nil {
				log.Info().Str("domain", site.Domain).Msg("scan cancelled")
				c.siteRepo.UpdateStatus(ctx, site.ID.Hex(), "error")
				return
			}

			if state.processed >= maxPages {
				break
			}

			html, err := c.parser.FetchPage(ctx, url)
			if err != nil {
				log.Debug().Err(err).Str("url", url).Msg("failed to fetch page")
				continue
			}

			hasPlayer := playerRegex.MatchString(html)
			pageType := c.detector.DetectPageType(html, url)

			page := &repo.Page{
				UserID:            site.UserID,
				SiteID:            site.ID,
				URL:               url,
				HasPlayer:         hasPlayer,
				PageType:          pageType,
				ExcludeFromReport: pageType != "content",
			}

			if err := c.pageRepo.Upsert(ctx, page); err != nil {
				log.Error().Err(err).Str("url", url).Msg("failed to save page")
				continue
			}

			state.processed++

			if state.processed%50 == 0 {
				total, withPlayer, withoutPlayer, _ := c.pageRepo.CountBySiteID(ctx, site.ID)
				c.siteRepo.UpdateStats(ctx, site.ID, total, withPlayer, withoutPlayer)
			}

			if depth < maxDepth-1 {
				links, err := c.parser.ExtractLinks(html, url)
				if err == nil {
					for _, link := range links {
						if !state.seen[link] {
							state.seen[link] = true
							state.queue = append(state.queue, link)
						}
					}
				}
			}

			time.Sleep(rateLimit)
		}

		depth++
		log.Debug().Str("domain", site.Domain).Int("depth", depth).Int("processed", state.processed).Int("queue", len(state.queue)).Msg("crawl progress")
	}

	total, withPlayer, withoutPlayer, _ := c.pageRepo.CountBySiteID(ctx, site.ID)
	c.siteRepo.UpdateStats(ctx, site.ID, total, withPlayer, withoutPlayer)

	if err := c.siteRepo.UpdateStatus(ctx, site.ID.Hex(), "active"); err != nil {
		log.Error().Err(err).Str("site_id", site.ID.Hex()).Msg("failed to update site status")
	}

	log.Info().
		Str("domain", site.Domain).
		Int("scanned", state.processed).
		Msg("scan completed")
}

func (c *Crawler) ScanAllSites(ctx context.Context) {
	log := logger.Log

	sites, err := c.siteRepo.GetActiveSites(ctx)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch active sites")
		return
	}

	log.Info().Int("count", len(sites)).Msg("starting scheduled scan of all sites")

	for _, site := range sites {
		if ctx.Err() != nil {
			log.Info().Msg("scheduled scan cancelled")
			return
		}

		c.ScanSite(ctx, &site)
		time.Sleep(5 * time.Second)
	}

	log.Info().Msg("scheduled scan completed")
}

func (c *Crawler) UpdateSiteStats(ctx context.Context, siteID primitive.ObjectID) error {
	total, withPlayer, withoutPlayer, err := c.pageRepo.CountBySiteID(ctx, siteID)
	if err != nil {
		return err
	}

	return c.siteRepo.UpdateStats(ctx, siteID, total, withPlayer, withoutPlayer)
}
