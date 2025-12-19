package queue

import "time"

type CookieData struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  int64  `json:"expires,omitempty"`
	HTTPOnly bool   `json:"http_only"`
	Secure   bool   `json:"secure"`
}

type CrawlTask struct {
	ID            string       `json:"id"`
	SiteID        string       `json:"site_id"`
	Domain        string       `json:"domain"`
	HasSitemap    bool         `json:"has_sitemap"`
	CrawlStrategy string       `json:"crawl_strategy"`           // sitemap или recursive
	SitemapURLs   []string     `json:"sitemap_urls,omitempty"`
	CMS           string       `json:"cms,omitempty"`
	ScannerType   string       `json:"scanner_type,omitempty"`   // "http" или "spa"
	CaptchaType   string       `json:"captcha_type,omitempty"`   // тип капчи для обхода
	Cookies       []CookieData `json:"cookies,omitempty"`        // сохранённые cookies для обхода капчи
	ScanIntervalH int          `json:"scan_interval_h,omitempty"`
	CreatedAt     time.Time    `json:"created_at"`
}

type SitemapStat struct {
	URL       string `json:"url"`
	URLsFound int    `json:"urls_found"`
	Error     string `json:"error,omitempty"`
}

type ParsedURLData struct {
	URL        string     `json:"url"`
	LastMod    *time.Time `json:"lastmod,omitempty"`
	ChangeFreq string     `json:"changefreq,omitempty"`
	Priority   float64    `json:"priority,omitempty"`
	Source     string     `json:"source"` // sitemap URL откуда взят
	Depth      int        `json:"depth"`  // 0 = из sitemap, 1-3 = найдены при парсинге
}

type CrawlResult struct {
	TaskID          string          `json:"task_id"`
	SiteID          string          `json:"site_id"`
	Success         bool            `json:"success"`
	PagesFound      int             `json:"pages_found"`
	PagesSaved      int             `json:"pages_saved"`
	Error           string          `json:"error,omitempty"`
	IsDomainExpired bool            `json:"is_domain_expired,omitempty"`
	IsBlocked       bool            `json:"is_blocked,omitempty"`
	BlockedCount    int             `json:"blocked_count,omitempty"`
	SitemapStats    []SitemapStat   `json:"sitemap_stats,omitempty"`
	ParsedURLs      []ParsedURLData `json:"parsed_urls,omitempty"` // URL из sitemap с метаданными
	NewCookies      []CookieData    `json:"new_cookies,omitempty"` // обновлённые cookies после решения капчи
	ScanIntervalH   int             `json:"scan_interval_h,omitempty"`
	FinishedAt      time.Time       `json:"finished_at"`
}

type CrawlProgress struct {
	TaskID         string `json:"task_id"`
	PagesFound     int    `json:"pages_found"`     // всего URL для обработки
	PagesProcessed int    `json:"pages_processed"` // обработано URL
	PagesSaved     int    `json:"pages_saved"`     // сохранено страниц с контентом
}

type DetectTask struct {
	ID        string    `json:"id"`
	SiteID    string    `json:"site_id"`
	Domain    string    `json:"domain"`
	CreatedAt time.Time `json:"created_at"`
}

type DetectResultMsg struct {
	TaskID            string       `json:"task_id"`
	SiteID            string       `json:"site_id"`
	Success           bool         `json:"success"`
	Error             string       `json:"error,omitempty"`
	CMS               string       `json:"cms"`
	CMSVersion        string       `json:"cms_version,omitempty"`
	HasSitemap        bool         `json:"has_sitemap"`
	SitemapStatus     string       `json:"sitemap_status"` // none, valid, invalid, empty
	CrawlStrategy     string       `json:"crawl_strategy"` // sitemap, recursive
	SitemapURLs       []string     `json:"sitemap_urls,omitempty"`
	NeedsSPA          bool         `json:"needs_spa"`
	CaptchaType       string       `json:"captcha_type,omitempty"`
	Cookies           []CookieData `json:"cookies,omitempty"`
	HasDomainRedirect bool         `json:"has_domain_redirect,omitempty"`
	RedirectToDomain  string       `json:"redirect_to_domain,omitempty"`
	FinishedAt        time.Time    `json:"finished_at"`
}

// ============================================
// Двухэтапный парсинг: SitemapCrawl + PageCrawl
// ============================================

type SitemapCrawlTask struct {
	ID            string       `json:"id"`
	SiteID        string       `json:"site_id"`
	Domain        string       `json:"domain"`
	SitemapURLs   []string     `json:"sitemap_urls"`
	ScannerType   string       `json:"scanner_type"`
	CaptchaType   string       `json:"captcha_type,omitempty"`
	Cookies       []CookieData `json:"cookies,omitempty"`
	AutoContinue  bool         `json:"auto_continue"`   // если true, автоматически запустить page crawl после завершения
	IndexerAPIURL string       `json:"indexer_api_url"` // URL indexer API для получения уже известных URL
	CreatedAt     time.Time    `json:"created_at"`
}

type SitemapURLBatch struct {
	TaskID        string          `json:"task_id"`
	SiteID        string          `json:"site_id"`
	URLs          []ParsedURLData `json:"urls"`
	BatchNumber   int             `json:"batch_number"`
	SitemapSource string          `json:"sitemap_source"`
}

type SitemapCrawlResult struct {
	TaskID       string        `json:"task_id"`
	SiteID       string        `json:"site_id"`
	Success      bool          `json:"success"`
	TotalURLs    int           `json:"total_urls"`
	NewURLs      int           `json:"new_urls"`
	SitemapStats []SitemapStat `json:"sitemap_stats,omitempty"`
	Error        string        `json:"error,omitempty"`
	NewCookies   []CookieData  `json:"new_cookies,omitempty"`
	AutoContinue bool          `json:"auto_continue"` // передаётся из SitemapCrawlTask
	FinishedAt   time.Time     `json:"finished_at"`
}

type PageCrawlTask struct {
	ID            string       `json:"id"`
	SiteID        string       `json:"site_id"`
	Domain        string       `json:"domain"`
	ScannerType   string       `json:"scanner_type"`
	CaptchaType   string       `json:"captcha_type,omitempty"`
	Cookies       []CookieData `json:"cookies,omitempty"`
	BatchSize     int          `json:"batch_size"`
	IndexerAPIURL string       `json:"indexer_api_url"`
	CreatedAt     time.Time    `json:"created_at"`
}

type PageResult struct {
	URL       string    `json:"url"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	Page      *PageData `json:"page,omitempty"`
	IPBlocked bool      `json:"ip_blocked,omitempty"`
}

type PageData struct {
	URL         string            `json:"url"`
	Title       string            `json:"title"`
	Description string            `json:"description,omitempty"`
	MainText    string            `json:"main_text,omitempty"`
	Year        int               `json:"year,omitempty"`
	PlayerURL   string            `json:"player_url,omitempty"`
	LinksText   string            `json:"links_text,omitempty"`
	ExternalIDs map[string]string `json:"external_ids,omitempty"`
}

type PageBatchResult struct {
	TaskID      string       `json:"task_id"`
	SiteID      string       `json:"site_id"`
	Pages       []PageResult `json:"pages"`
	BatchNumber int          `json:"batch_number"`
	NewCookies  []CookieData `json:"new_cookies,omitempty"`
}

type PageCrawlResult struct {
	TaskID          string       `json:"task_id"`
	SiteID          string       `json:"site_id"`
	Success         bool         `json:"success"`
	PagesTotal      int          `json:"pages_total"`
	PagesSuccess    int          `json:"pages_success"`
	PagesFailed     int          `json:"pages_failed"`
	Error           string       `json:"error,omitempty"`
	NewCookies      []CookieData `json:"new_cookies,omitempty"`
	FinishedAt      time.Time    `json:"finished_at"`
	NoURLsAvailable bool         `json:"no_urls_available,omitempty"`
	AllIndexed      bool         `json:"all_indexed,omitempty"`
	IndexedCount    int          `json:"indexed_count,omitempty"`
	IPBlocked       bool         `json:"ip_blocked,omitempty"`
	BlockReason     string       `json:"block_reason,omitempty"`
}

// PageSingleResult - результат парсинга одной страницы
// Публикуется сразу после парсинга для мгновенного обновления статуса URL
type PageSingleResult struct {
	TaskID    string    `json:"task_id"`
	SiteID    string    `json:"site_id"`
	URL       string    `json:"url"`
	Success   bool      `json:"success"`
	Error     string    `json:"error,omitempty"`
	Page      *PageData `json:"page,omitempty"`
	IPBlocked bool      `json:"ip_blocked,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}
