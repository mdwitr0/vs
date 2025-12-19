# Структуры данных

## Обзор

Модели данных для indexer сервиса. Все JSON поля используют **snake_case**.

---

## 1. Site (отслеживаемый сайт)

```go
type Site struct {
    ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
    Domain        string             `bson:"domain" json:"domain"`

    // Статус
    Status        SiteStatus         `bson:"status" json:"status"`
    LastSuccess   *time.Time         `bson:"last_success,omitempty" json:"last_success,omitempty"`
    LastCheck     *time.Time         `bson:"last_check,omitempty" json:"last_check,omitempty"`
    FailureCount  int                `bson:"failure_count" json:"failure_count"`

    // Детектированные технологии
    DetectedTech  *TechDetection     `bson:"detected_tech,omitempty" json:"detected_tech,omitempty"`

    // Конфигурация сканирования
    ScanConfig    ScanConfig         `bson:"scan_config" json:"scan_config"`

    // Статистика
    PagesCount    int                `bson:"pages_count" json:"pages_count"`

    // Метаданные
    CreatedAt     time.Time          `bson:"created_at" json:"created_at"`
    UpdatedAt     time.Time          `bson:"updated_at" json:"updated_at"`
}

type SiteStatus string

const (
    SiteStatusActive  SiteStatus = "active"
    SiteStatusDown    SiteStatus = "down"
    SiteStatusBlocked SiteStatus = "blocked"
)
```

### TechDetection (результат детекции)

```go
type TechDetection struct {
    // CMS
    CMS           string   `bson:"cms" json:"cms"`                       // DLE, WordPress, CinemaPress, uCoz, Custom, Unknown
    CMSVersion    string   `bson:"cms_version,omitempty" json:"cms_version,omitempty"`

    // Рендеринг
    RenderType    string   `bson:"render_type" json:"render_type"`       // SSR, CSR, Hybrid
    NeedsBrowser  bool     `bson:"needs_browser" json:"needs_browser"`

    // SPA Framework
    Framework     string   `bson:"framework,omitempty" json:"framework,omitempty"` // React, Vue, Next.js, Nuxt

    // Sitemap
    HasSitemap    bool     `bson:"has_sitemap" json:"has_sitemap"`
    SitemapURLs   []string `bson:"sitemap_urls,omitempty" json:"sitemap_urls,omitempty"`

    // Защита
    CaptchaType   string   `bson:"captcha_type" json:"captcha_type"`     // none, recaptcha, cloudflare, dle-antibot

    // Особенности
    SubdomainPerMovie bool `bson:"subdomain_per_movie" json:"subdomain_per_movie"`

    // Метаданные детекции
    Confidence    float64  `bson:"confidence" json:"confidence"`
    DetectedBy    []string `bson:"detected_by" json:"detected_by"`       // список сработавших маркеров
    DetectedAt    time.Time `bson:"detected_at" json:"detected_at"`
}
```

### ScanConfig (конфигурация сканирования)

```go
type ScanConfig struct {
    // Тип сканера
    ScannerType   string        `bson:"scanner_type" json:"scanner_type"`     // universal, dle, wordpress, spa

    // Настройки
    ScanInterval  time.Duration `bson:"scan_interval" json:"scan_interval"`   // интервал между сканами
    MaxPages      int           `bson:"max_pages" json:"max_pages"`           // лимит страниц за скан
    RateLimit     float64       `bson:"rate_limit" json:"rate_limit"`         // requests per second

    // Флаги
    UseSitemap    bool          `bson:"use_sitemap" json:"use_sitemap"`
    UseBrowser    bool          `bson:"use_browser" json:"use_browser"`
    Enabled       bool          `bson:"enabled" json:"enabled"`
}
```

### MongoDB индексы

```javascript
// Уникальный домен
db.sites.createIndex({ "domain": 1 }, { unique: true })

// Для выборки активных сайтов
db.sites.createIndex({ "status": 1, "scan_config.enabled": 1 })

// Для шедулера
db.sites.createIndex({ "last_check": 1, "status": 1 })
```

---

## 2. Page (проиндексированная страница)

```go
type Page struct {
    ID            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
    SiteID        primitive.ObjectID `bson:"site_id" json:"site_id"`

    // URL
    URL           string             `bson:"url" json:"url"`
    NormalizedURL string             `bson:"normalized_url" json:"normalized_url"`

    // Извлечённые данные
    Title         string             `bson:"title" json:"title"`
    OriginalTitle string             `bson:"original_title,omitempty" json:"original_title,omitempty"`
    Year          int                `bson:"year,omitempty" json:"year,omitempty"`

    // Внешние ID
    ExternalIDs   ExternalIDs        `bson:"external_ids" json:"external_ids"`

    // Плеер
    PlayerInfo    *PlayerInfo        `bson:"player_info,omitempty" json:"player_info,omitempty"`

    // Метаданные страницы
    MetaTags      map[string]string  `bson:"meta_tags,omitempty" json:"meta_tags,omitempty"`
    SchemaOrg     map[string]interface{} `bson:"schema_org,omitempty" json:"schema_org,omitempty"`

    // Timestamps
    IndexedAt     time.Time          `bson:"indexed_at" json:"indexed_at"`
    LastCheckedAt time.Time          `bson:"last_checked_at" json:"last_checked_at"`

    // Статус
    HTTPStatus    int                `bson:"http_status" json:"http_status"`
    IsAvailable   bool               `bson:"is_available" json:"is_available"`
}

type ExternalIDs struct {
    KinopoiskID   string `bson:"kp_id,omitempty" json:"kp_id,omitempty"`
    IMDbID        string `bson:"imdb_id,omitempty" json:"imdb_id,omitempty"`
    TMDbID        string `bson:"tmdb_id,omitempty" json:"tmdb_id,omitempty"`
    MALID         string `bson:"mal_id,omitempty" json:"mal_id,omitempty"`
    ShikimoriID   string `bson:"shikimori_id,omitempty" json:"shikimori_id,omitempty"`
    MyDramaListID string `bson:"mydramalist_id,omitempty" json:"mydramalist_id,omitempty"`
}

type PlayerInfo struct {
    IframeURL   string `bson:"iframe_url" json:"iframe_url"`
    Provider    string `bson:"provider" json:"provider"`       // bazon, videocdn, kodik, etc
    VideoID     string `bson:"video_id,omitempty" json:"video_id,omitempty"`
}
```

### MongoDB индексы

```javascript
// Уникальный URL в рамках сайта
db.pages.createIndex({ "site_id": 1, "normalized_url": 1 }, { unique: true })

// Поиск по внешним ID
db.pages.createIndex({ "external_ids.kp_id": 1 })
db.pages.createIndex({ "external_ids.imdb_id": 1 })

// Составной для поиска нарушений
db.pages.createIndex({ "external_ids.kp_id": 1, "site_id": 1 })

// Для обновления
db.pages.createIndex({ "last_checked_at": 1 })

// Полнотекстовый поиск по названию
db.pages.createIndex({ "title": "text", "original_title": "text" })
```

---

## 3. ScanTask (задача сканирования)

```go
type ScanTask struct {
    ID          primitive.ObjectID   `bson:"_id,omitempty" json:"id"`

    // Связи
    SiteIDs     []primitive.ObjectID `bson:"site_ids" json:"site_ids"`

    // Статус
    Status      TaskStatus           `bson:"status" json:"status"`
    Progress    TaskProgress         `bson:"progress" json:"progress"`

    // Результат
    Result      *TaskResult          `bson:"result,omitempty" json:"result,omitempty"`
    Error       string               `bson:"error,omitempty" json:"error,omitempty"`

    // Timestamps
    CreatedAt   time.Time            `bson:"created_at" json:"created_at"`
    StartedAt   *time.Time           `bson:"started_at,omitempty" json:"started_at,omitempty"`
    FinishedAt  *time.Time           `bson:"finished_at,omitempty" json:"finished_at,omitempty"`
}

type TaskStatus string

const (
    TaskStatusPending    TaskStatus = "pending"
    TaskStatusRunning    TaskStatus = "running"
    TaskStatusCompleted  TaskStatus = "completed"
    TaskStatusFailed     TaskStatus = "failed"
    TaskStatusCancelled  TaskStatus = "cancelled"
)

type TaskProgress struct {
    TotalSites     int `bson:"total_sites" json:"total_sites"`
    CompletedSites int `bson:"completed_sites" json:"completed_sites"`
    TotalPages     int `bson:"total_pages" json:"total_pages"`
    ProcessedPages int `bson:"processed_pages" json:"processed_pages"`
}

type TaskResult struct {
    SitesScanned  int `bson:"sites_scanned" json:"sites_scanned"`
    PagesFound    int `bson:"pages_found" json:"pages_found"`
    PagesUpdated  int `bson:"pages_updated" json:"pages_updated"`
    Errors        int `bson:"errors" json:"errors"`
}
```

### MongoDB индексы и TTL

```javascript
// Поиск по статусу
db.scan_tasks.createIndex({ "status": 1 })

// TTL - удалять через 30 дней
db.scan_tasks.createIndex({ "created_at": 1 }, { expireAfterSeconds: 2592000 })
```

---

## 4. API DTO

### Запросы

```go
// POST /api/sites
type CreateSiteRequest struct {
    Domain string `json:"domain" validate:"required,hostname"`
}

// POST /api/sites/import
type ImportSitesRequest struct {
    Domains []string `json:"domains" validate:"required,min=1"`
}

// POST /api/sites/scan
type ScanRequest struct {
    SiteIDs []string `json:"site_ids" validate:"required,min=1"`
}

// GET /api/sites?status=active&page=1&limit=20
type ListSitesQuery struct {
    Status string `query:"status"`
    Search string `query:"search"`
    Page   int    `query:"page" validate:"min=1"`
    Limit  int    `query:"limit" validate:"min=1,max=100"`
}
```

### Ответы

```go
// Сайт
type SiteResponse struct {
    ID           string         `json:"id"`
    Domain       string         `json:"domain"`
    Status       string         `json:"status"`
    LastCheck    *time.Time     `json:"last_check,omitempty"`
    DetectedTech *TechDetection `json:"detected_tech,omitempty"`
    PagesCount   int            `json:"pages_count"`
    CreatedAt    time.Time      `json:"created_at"`
}

// Список с пагинацией
type PaginatedResponse[T any] struct {
    Data       []T `json:"data"`
    Pagination struct {
        Page       int `json:"page"`
        Limit      int `json:"limit"`
        Total      int `json:"total"`
        TotalPages int `json:"total_pages"`
    } `json:"pagination"`
}

// Задача сканирования
type ScanTaskResponse struct {
    ID         string        `json:"id"`
    Status     string        `json:"status"`
    Progress   TaskProgress  `json:"progress"`
    Result     *TaskResult   `json:"result,omitempty"`
    Error      string        `json:"error,omitempty"`
    CreatedAt  time.Time     `json:"created_at"`
    StartedAt  *time.Time    `json:"started_at,omitempty"`
    FinishedAt *time.Time    `json:"finished_at,omitempty"`
}
```

---

## 5. Внутренние структуры

### HTTP клиент

```go
type FetchResult struct {
    URL         string
    StatusCode  int
    Headers     http.Header
    Body        []byte
    ContentType string
    FinalURL    string // после редиректов
    Duration    time.Duration
    Error       error
}
```

### Результат парсинга

```go
type ParseResult struct {
    // Основные данные
    Title         string
    OriginalTitle string
    Year          int

    // ID
    ExternalIDs   ExternalIDs

    // Плеер
    PlayerInfo    *PlayerInfo

    // Мета
    MetaTags      map[string]string
    SchemaOrg     map[string]interface{}

    // Ссылки для краулинга
    Links         []string

    // Качество парсинга
    Confidence    float64
    ParsedFields  []string // какие поля удалось извлечь
}
```

---

## 6. Конфигурация

```go
type Config struct {
    Server   ServerConfig   `yaml:"server"`
    MongoDB  MongoDBConfig  `yaml:"mongodb"`
    Scanner  ScannerConfig  `yaml:"scanner"`
    Detector DetectorConfig `yaml:"detector"`
}

type ServerConfig struct {
    Host string `yaml:"host" env:"SERVER_HOST" env-default:"0.0.0.0"`
    Port int    `yaml:"port" env:"SERVER_PORT" env-default:"8080"`
}

type MongoDBConfig struct {
    URI      string `yaml:"uri" env:"MONGODB_URI" env-default:"mongodb://localhost:27017"`
    Database string `yaml:"database" env:"MONGODB_DATABASE" env-default:"indexer"`
}

type ScannerConfig struct {
    DefaultInterval time.Duration `yaml:"default_interval" env-default:"24h"`
    MaxConcurrent   int           `yaml:"max_concurrent" env-default:"5"`
    RequestTimeout  time.Duration `yaml:"request_timeout" env-default:"30s"`
    UserAgent       string        `yaml:"user_agent"`
}

type DetectorConfig struct {
    Timeout          time.Duration `yaml:"timeout" env-default:"10s"`
    CheckSitemap     bool          `yaml:"check_sitemap" env-default:"true"`
    CheckRobots      bool          `yaml:"check_robots" env-default:"true"`
}
```
