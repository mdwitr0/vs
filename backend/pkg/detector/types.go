package detector

import "time"

type CMS string

const (
	CMSUnknown     CMS = "unknown"
	CMSDLE         CMS = "dle"
	CMSWordPress   CMS = "wordpress"
	CMSCinemaPress CMS = "cinemapress"
	CMSUCoz        CMS = "ucoz"
	CMSCustom      CMS = "custom"
)

type RenderType string

const (
	RenderSSR    RenderType = "ssr"
	RenderCSR    RenderType = "csr"
	RenderHybrid RenderType = "hybrid"
)

type Framework string

const (
	FrameworkNone   Framework = ""
	FrameworkReact  Framework = "react"
	FrameworkVue    Framework = "vue"
	FrameworkNextJS Framework = "nextjs"
	FrameworkNuxt   Framework = "nuxt"
)

type SitemapStatus string

const (
	SitemapNone    SitemapStatus = "none"    // sitemap не найден
	SitemapValid   SitemapStatus = "valid"   // sitemap работает и содержит URLs
	SitemapInvalid SitemapStatus = "invalid" // sitemap указан но недоступен (404, ошибка)
	SitemapEmpty   SitemapStatus = "empty"   // sitemap работает но пустой
)

type CrawlStrategy string

const (
	// CrawlStrategySitemap - единственная стратегия: sitemap если есть + рекурсивный сбор ссылок
	CrawlStrategySitemap CrawlStrategy = "sitemap"
)

type CaptchaType string

const (
	CaptchaNone       CaptchaType = "none"
	CaptchaReCAPTCHA  CaptchaType = "recaptcha"
	CaptchaHCaptcha   CaptchaType = "hcaptcha"
	CaptchaCloudflare CaptchaType = "cloudflare"
	CaptchaDDoSGuard  CaptchaType = "ddos-guard"
	CaptchaDLEAntibot CaptchaType = "dle-antibot"
	CaptchaUCoz       CaptchaType = "ucoz"
	CaptchaPirate     CaptchaType = "pirate"
	CaptchaCustom     CaptchaType = "custom"
)

type Result struct {
	CMS        CMS    `json:"cms"`
	CMSVersion string `json:"cms_version,omitempty"`

	RenderType   RenderType `json:"render_type"`
	NeedsBrowser bool       `json:"needs_browser"`
	Framework    Framework  `json:"framework,omitempty"`

	HasSitemap    bool          `json:"has_sitemap"`
	SitemapStatus SitemapStatus `json:"sitemap_status"`
	SitemapURLs   []string      `json:"sitemap_urls,omitempty"`
	CrawlStrategy CrawlStrategy `json:"crawl_strategy"`

	CaptchaType CaptchaType  `json:"captcha_type"`
	Cookies     []CookieData `json:"cookies,omitempty"`

	SubdomainPerMovie bool `json:"subdomain_per_movie"`

	FinalURL     string `json:"final_url,omitempty"`
	HasRedirects bool   `json:"has_redirects"`

	Confidence float64   `json:"confidence"`
	DetectedBy []string  `json:"detected_by"`
	DetectedAt time.Time `json:"detected_at"`

	StatusCode  int               `json:"status_code"`
	Headers     map[string]string `json:"headers,omitempty"`
	ContentType string            `json:"content_type,omitempty"`
}

type Marker struct {
	Type       string
	Name       string
	Value      string
	Confidence float64
}

type FetchResult struct {
	URL         string
	FinalURL    string
	StatusCode  int
	Headers     map[string]string
	Body        []byte
	ContentType string
	Error       error
}
