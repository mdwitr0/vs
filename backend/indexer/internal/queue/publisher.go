package queue

import (
	"context"
	"os"
	"time"

	"github.com/video-analitics/backend/pkg/nats"
	"github.com/video-analitics/backend/pkg/queue"
	"github.com/video-analitics/indexer/internal/repo"
)

type Publisher struct {
	np *nats.Publisher
}

func NewPublisher(client *nats.Client) *Publisher {
	return &Publisher{np: nats.NewPublisher(client)}
}

type TaskInfo struct {
	TaskID       string
	Site         *repo.Site
	AutoContinue bool // для sitemap задач: автоматически запустить page crawl
}

func (p *Publisher) PublishCrawlTask(ctx context.Context, info TaskInfo) error {
	var cookies []queue.CookieData
	for _, c := range info.Site.Cookies {
		cookies = append(cookies, queue.CookieData{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  c.Expires,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
		})
	}

	task := queue.CrawlTask{
		ID:            info.TaskID,
		SiteID:        info.Site.ID.Hex(),
		Domain:        info.Site.Domain,
		HasSitemap:    info.Site.HasSitemap,
		SitemapURLs:   info.Site.SitemapURLs,
		CMS:           info.Site.CMS,
		ScannerType:   string(info.Site.ScannerType),
		CaptchaType:   info.Site.CaptchaType,
		Cookies:       cookies,
		ScanIntervalH: info.Site.ScanIntervalH,
		CreatedAt:     time.Now(),
	}

	return p.np.PublishCrawlTask(ctx, task)
}

func (p *Publisher) PublishCrawlTasks(ctx context.Context, tasks []TaskInfo) error {
	for _, info := range tasks {
		if err := p.PublishCrawlTask(ctx, info); err != nil {
			return err
		}
	}
	return nil
}

func (p *Publisher) PublishTasks(ctx context.Context, tasks []queue.CrawlTask) error {
	for _, task := range tasks {
		if err := p.np.PublishCrawlTask(ctx, task); err != nil {
			return err
		}
	}
	return nil
}

func (p *Publisher) PublishTask(ctx context.Context, task *queue.CrawlTask) error {
	return p.np.PublishCrawlTask(ctx, task)
}

func (p *Publisher) PublishDetectTask(ctx context.Context, taskID, siteID, domain string) error {
	task := queue.DetectTask{
		ID:        taskID,
		SiteID:    siteID,
		Domain:    domain,
		CreatedAt: time.Now(),
	}
	return p.np.PublishDetectTask(ctx, task)
}

func (p *Publisher) PublishSitemapCrawlTask(ctx context.Context, info TaskInfo) error {
	var cookies []queue.CookieData
	for _, c := range info.Site.Cookies {
		cookies = append(cookies, queue.CookieData{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  c.Expires,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
		})
	}

	indexerAPIURL := "http://localhost:8080"
	if env := os.Getenv("INDEXER_API_URL"); env != "" {
		indexerAPIURL = env
	}

	task := queue.SitemapCrawlTask{
		ID:            info.TaskID,
		SiteID:        info.Site.ID.Hex(),
		Domain:        info.Site.Domain,
		SitemapURLs:   info.Site.SitemapURLs,
		ScannerType:   string(info.Site.ScannerType),
		CaptchaType:   info.Site.CaptchaType,
		Cookies:       cookies,
		AutoContinue:  info.AutoContinue,
		IndexerAPIURL: indexerAPIURL,
		CreatedAt:     time.Now(),
	}

	return p.np.PublishSitemapCrawlTask(ctx, task)
}

func (p *Publisher) PublishPageCrawlTask(ctx context.Context, info TaskInfo, indexerAPIURL string, batchSize int) error {
	var cookies []queue.CookieData
	for _, c := range info.Site.Cookies {
		cookies = append(cookies, queue.CookieData{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  c.Expires,
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
		})
	}

	task := queue.PageCrawlTask{
		ID:            info.TaskID,
		SiteID:        info.Site.ID.Hex(),
		Domain:        info.Site.Domain,
		ScannerType:   string(info.Site.ScannerType),
		CaptchaType:   info.Site.CaptchaType,
		Cookies:       cookies,
		BatchSize:     batchSize,
		IndexerAPIURL: indexerAPIURL,
		CreatedAt:     time.Now(),
	}

	return p.np.PublishPageCrawlTask(ctx, task)
}

// PublishPageCrawlTaskSimple - упрощённая версия с дефолтными значениями
func (p *Publisher) PublishPageCrawlTaskSimple(ctx context.Context, info TaskInfo) error {
	indexerAPIURL := "http://localhost:8080"
	if env := os.Getenv("INDEXER_API_URL"); env != "" {
		indexerAPIURL = env
	}
	return p.PublishPageCrawlTask(ctx, info, indexerAPIURL, 50)
}
