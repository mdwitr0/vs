package status

// Site represents the status of a site in the system
// @Description Site status
// @enum pending,active,down,dead,frozen,moved
type Site string

const (
	SitePending Site = "pending" // awaiting detection
	SiteActive  Site = "active"  // ready for scanning
	SiteDown    Site = "down"    // temporarily unavailable (1 failure)
	SiteDead    Site = "dead"    // permanently unavailable (2+ failures)
	SiteFrozen  Site = "frozen"  // blocked (403/captcha), requires SPA scanner
	SiteMoved   Site = "moved"   // domain redirected to another domain
)

// Task represents the status of a scan task
// @Description Task status
// @enum pending,processing,completed,failed,cancelled
type Task string

const (
	TaskPending    Task = "pending"    // waiting in queue
	TaskProcessing Task = "processing" // being processed by worker
	TaskCompleted  Task = "completed"  // finished successfully
	TaskFailed     Task = "failed"     // finished with error
	TaskCancelled  Task = "cancelled"  // cancelled by user
)

// URL represents the status of a sitemap URL
// @Description URL indexing status
// @enum pending,processing,indexed,error,skipped
type URL string

const (
	URLPending    URL = "pending"    // waiting to be indexed
	URLProcessing URL = "processing" // currently being parsed
	URLIndexed    URL = "indexed"    // successfully indexed
	URLError      URL = "error"      // failed after max retries
	URLSkipped    URL = "skipped"    // skipped (e.g., XML sitemap reference)
)

// ScannerType represents the type of scanner used for a site
// @Description Scanner type
// @enum http,spa
type ScannerType string

const (
	ScannerHTTP ScannerType = "http" // standard HTTP fetcher
	ScannerSPA  ScannerType = "spa"  // chromedp for SPA/protected sites
)

// Stage represents the current stage of a scan task
// @Description Task stage
// @enum sitemap,page,done
type Stage string

const (
	StageSitemap Stage = "sitemap" // collecting URLs from sitemap
	StagePage    Stage = "page"    // parsing pages
	StageDone    Stage = "done"    // all stages completed
)

// SitemapStatus represents the status of sitemap detection
// @Description Sitemap detection status
// @enum none,valid,invalid,empty
type SitemapStatus string

const (
	SitemapNone    SitemapStatus = "none"    // sitemap not found
	SitemapValid   SitemapStatus = "valid"   // sitemap works and contains URLs
	SitemapInvalid SitemapStatus = "invalid" // sitemap declared but unavailable (404, parse error)
	SitemapEmpty   SitemapStatus = "empty"   // sitemap works but has no URLs
)

// CrawlStrategy represents the crawling strategy for a site (deprecated - single strategy now)
// @Description Crawling strategy
// @enum sitemap
type CrawlStrategy string

const (
	// CrawlStrategySitemap - единственная стратегия: sitemap если есть + рекурсивный сбор ссылок
	CrawlStrategySitemap CrawlStrategy = "sitemap"
)

// AllSiteStatuses returns all valid site statuses
func AllSiteStatuses() []Site {
	return []Site{SitePending, SiteActive, SiteDown, SiteDead, SiteFrozen, SiteMoved}
}

// AllTaskStatuses returns all valid task statuses
func AllTaskStatuses() []Task {
	return []Task{TaskPending, TaskProcessing, TaskCompleted, TaskFailed, TaskCancelled}
}

// AllURLStatuses returns all valid URL statuses
func AllURLStatuses() []URL {
	return []URL{URLPending, URLProcessing, URLIndexed, URLError, URLSkipped}
}

// IsTerminal returns true if the task status is terminal (no further transitions)
func (t Task) IsTerminal() bool {
	return t == TaskCompleted || t == TaskFailed || t == TaskCancelled
}

// IsActive returns true if the task is still active (pending or processing)
func (t Task) IsActive() bool {
	return t == TaskPending || t == TaskProcessing
}

// CanScan returns true if the site can be scanned
func (s Site) CanScan() bool {
	return s == SiteActive || s == SiteDown
}

// IsTerminal returns true if the site status is terminal (no further transitions)
func (s Site) IsTerminal() bool {
	return s == SiteMoved
}
