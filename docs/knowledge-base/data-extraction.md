# Извлечение данных со страниц фильмов

## Обзор

Документ описывает паттерны извлечения информации о фильмах с пиратских сайтов:
- Название и год
- Внешние ID (Kinopoisk, IMDb, etc)
- URL плеера
- Метаданные

---

## 1. Извлечение названия и года

### Паттерны в title/h1

```
Формат 1: "Название (YYYY)"
Формат 2: "Название (YYYY) смотреть онлайн"
Формат 3: "Название / Original Title (YYYY)"
```

### Regex для извлечения

```go
// Название и год из заголовка
var titleYearRegex = regexp.MustCompile(`^(.+?)\s*\((\d{4})\)`)

// Пример: "Хатико: Самый верный друг (2009)"
// Группы: [1] = "Хатико: Самый верный друг", [2] = "2009"
```

### Селекторы по CMS

| CMS | Селектор названия | Селектор года |
|-----|-------------------|---------------|
| DLE | `h1`, `.short-title`, `#dle-content h1` | В title или отдельное поле |
| WordPress | `h1.entry-title`, `.wp-block-heading` | В title |
| Custom | `h1`, `[itemprop="name"]` | `[itemprop="datePublished"]` |

### Meta теги

```html
<meta property="og:title" content="Хатико: Самый верный друг (2009)">
<meta name="title" content="...">
```

---

## 2. Извлечение внешних ID

### 2.1 Kinopoisk ID

**Источники:**

1. **Ссылки на kinopoisk.ru:**
```html
<a href="https://www.kinopoisk.ru/film/252188/">КиноПоиск</a>
<a href="https://kinopoisk.ru/film/252188">
```
```go
var kpLinkRegex = regexp.MustCompile(`kinopoisk\.ru/film/(\d+)`)
```

2. **Data атрибуты:**
```html
<div data-kp="252188">
<div data-kinopoisk-id="252188">
<div data-kpid="252188">
```

3. **JavaScript переменные:**
```javascript
var kp_id = 252188;
var kinopoisk_id = "252188";
window.kpId = 252188;
```

4. **Player API URLs:**
```
https://api.provider.com/embed?kp=252188
https://player.com/?kinopoisk_id=252188
```

5. **JSON-LD Schema:**
```json
{
  "@type": "Movie",
  "identifier": "kp252188"
}
```

### 2.2 IMDb ID

**Формат:** `tt` + 7-8 цифр (tt0266543)

**Источники:**

1. **Ссылки:**
```html
<a href="https://www.imdb.com/title/tt0266543/">IMDb</a>
```
```go
var imdbLinkRegex = regexp.MustCompile(`imdb\.com/title/(tt\d+)`)
```

2. **Data атрибуты:**
```html
<div data-imdb="tt0266543">
<div data-imdb-id="tt0266543">
```

3. **JavaScript:**
```javascript
var imdb_id = "tt0266543";
```

4. **Player URLs:**
```
https://player.com/?imdb=tt0266543
```

### 2.3 Другие ID

| Источник | Формат | Regex |
|----------|--------|-------|
| TMDB | число | `themoviedb\.org/movie/(\d+)` |
| MAL (аниме) | число | `myanimelist\.net/anime/(\d+)` |
| Shikimori | число | `shikimori\.(one\|org)/animes/(\d+)` |
| MDL (дорамы) | число | `mydramalist\.com/(\d+)` |

---

## 3. Извлечение Player/Iframe URL

### Паттерны iframe

```html
<!-- Прямой iframe -->
<iframe src="https://player.example.com/embed/12345" allowfullscreen></iframe>

<!-- Lazy-loaded -->
<iframe data-src="https://player.example.com/embed/12345"></iframe>

<!-- С параметрами -->
<iframe src="https://api.videocdn.tv/get_video?kp_id=252188&token=..."></iframe>
```

### Селекторы

```go
// Общие селекторы для iframe плеера
playerSelectors := []string{
    "iframe[src*='player']",
    "iframe[src*='embed']",
    "iframe[data-src]",
    ".player iframe",
    ".video-player iframe",
    "#player iframe",
    ".kinobalancer iframe",
}
```

### JavaScript players

Некоторые плееры загружаются через JS:

```javascript
// Playerjs
new Playerjs({id:"player", file:"..."});

// Kinobalancer
window.kinobalancer = { ... };

// Custom
loadPlayer('https://api.example.com/video/12345');
```

**Поиск в скриптах:**
```go
var playerJsRegex = regexp.MustCompile(`file:\s*["']([^"']+)["']`)
var embedRegex = regexp.MustCompile(`(https?://[^\s"']+(?:embed|player|video)[^\s"']*)`)
```

---

## 4. Schema.org / JSON-LD

### Movie Schema

```html
<script type="application/ld+json">
{
  "@context": "https://schema.org",
  "@type": "Movie",
  "name": "Хатико: Самый верный друг",
  "alternativeHeadline": "Hachi: A Dog's Tale",
  "datePublished": "2009",
  "duration": "PT1H33M",
  "director": {
    "@type": "Person",
    "name": "Лассе Халльстрём"
  },
  "aggregateRating": {
    "@type": "AggregateRating",
    "ratingValue": "8.1",
    "ratingCount": "280213"
  }
}
</script>
```

### Извлечение JSON-LD

```go
func extractJSONLD(html string) ([]map[string]interface{}, error) {
    doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

    var results []map[string]interface{}

    doc.Find(`script[type="application/ld+json"]`).Each(func(i int, s *goquery.Selection) {
        var data map[string]interface{}
        if err := json.Unmarshal([]byte(s.Text()), &data); err == nil {
            results = append(results, data)
        }
    })

    return results, nil
}
```

---

## 5. Next.js / SPA данные

### __NEXT_DATA__

```html
<script id="__NEXT_DATA__" type="application/json">
{
  "props": {
    "pageProps": {
      "movie": {
        "id": 1013343,
        "externalIds": {
          "imdb": "tt6166392",
          "tmdb": 787699,
          "kpHD": null
        }
      }
    }
  }
}
</script>
```

### Извлечение

```go
func extractNextData(html string) (map[string]interface{}, error) {
    doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))

    script := doc.Find(`script#__NEXT_DATA__`).Text()

    var data map[string]interface{}
    err := json.Unmarshal([]byte(script), &data)

    return data, err
}
```

---

## 6. Паттерны по CMS

### DLE

```go
type DLEExtractor struct{}

func (e *DLEExtractor) Extract(doc *goquery.Document) *MovieData {
    data := &MovieData{}

    // Название из h1
    h1 := doc.Find("h1").First().Text()
    data.Title, data.Year = parseTitle(h1)

    // KP ID из ссылок
    doc.Find("a[href*='kinopoisk.ru/film/']").Each(func(i int, s *goquery.Selection) {
        href, _ := s.Attr("href")
        if matches := kpLinkRegex.FindStringSubmatch(href); len(matches) > 1 {
            data.KinopoiskID = matches[1]
        }
    })

    // IMDb из ссылок
    doc.Find("a[href*='imdb.com/title/']").Each(func(i int, s *goquery.Selection) {
        href, _ := s.Attr("href")
        if matches := imdbLinkRegex.FindStringSubmatch(href); len(matches) > 1 {
            data.IMDbID = matches[1]
        }
    })

    // Player iframe
    if iframe := doc.Find(".player iframe, #player iframe").First(); iframe.Length() > 0 {
        data.PlayerURL, _ = iframe.Attr("src")
        if data.PlayerURL == "" {
            data.PlayerURL, _ = iframe.Attr("data-src")
        }
    }

    return data
}
```

### WordPress

```go
type WordPressExtractor struct{}

func (e *WordPressExtractor) Extract(doc *goquery.Document) *MovieData {
    data := &MovieData{}

    // OG meta
    if ogTitle, exists := doc.Find(`meta[property="og:title"]`).Attr("content"); exists {
        data.Title, data.Year = parseTitle(ogTitle)
    }

    // JSON-LD
    doc.Find(`script[type="application/ld+json"]`).Each(func(i int, s *goquery.Selection) {
        // parse schema.org Movie
    })

    // Kinobalancer player
    if iframe := doc.Find(".kinobalancer iframe, .kinobalancer-frame iframe").First(); iframe.Length() > 0 {
        data.PlayerURL, _ = iframe.Attr("src")
    }

    return data
}
```

---

## 7. Fallback стратегия

Если специфичные селекторы не сработали:

```go
func extractFallback(doc *goquery.Document, html string) *MovieData {
    data := &MovieData{}

    data.Title = doc.Find("h1").First().Text()

    if data.Title == "" {
        data.Title = doc.Find("title").Text()
    }

    yearRegex := regexp.MustCompile(`\b(19|20)\d{2}\b`)
    if matches := yearRegex.FindString(html); matches != "" {
        data.Year = matches
    }

    if matches := kpLinkRegex.FindStringSubmatch(html); len(matches) > 1 {
        data.KinopoiskID = matches[1]
    }

    if matches := imdbLinkRegex.FindStringSubmatch(html); len(matches) > 1 {
        data.IMDbID = matches[1]
    }

    doc.Find("iframe").Each(func(i int, s *goquery.Selection) {
        src, _ := s.Attr("src")
        if src == "" {
            src, _ = s.Attr("data-src")
        }
        if strings.Contains(src, "player") || strings.Contains(src, "embed") || strings.Contains(src, "video") {
            data.PlayerURL = src
        }
    })

    return data
}
```

---

## 8. Примеры реальных данных

### zona-films.video (DLE)

```
URL: /1535-hatiko-samyj-vernyj-drug-2009.html
Title: Хатико: Самый верный друг (2009)
English: Hachi: A Dog's Tale
Year: 2009
KP Rating: 9.3
IMDb Rating: 8.1 (280213 votes)
Director: Лассе Халльстрём
Runtime: 89 минут
Players: 3 варианта + трейлер
```

### mi.anwap.today (Custom)

```
URL: /films/12486
Internal ID: 12486
Title: Флюк
Year: 1995
Player: https://api.namy.ws/embed/movie/922
Quality: DVDRip
```

### kinomore.netlify.app (Next.js)

```
URL: /film/1013343
Data source: __NEXT_DATA__.props.pageProps
IMDb: tt6166392
TMDb: 787699
KP HD: null
SSR: true (gssp)
```

---

## 9. Структура результата

```go
type MovieData struct {
    // Основное
    Title       string `json:"title"`
    OriginalTitle string `json:"original_title,omitempty"`
    Year        int    `json:"year,omitempty"`

    // Внешние ID
    KinopoiskID string `json:"kp_id,omitempty"`
    IMDbID      string `json:"imdb_id,omitempty"`
    TMDbID      string `json:"tmdb_id,omitempty"`
    MALID       string `json:"mal_id,omitempty"`
    ShikimoriID string `json:"shikimori_id,omitempty"`

    // Плеер
    PlayerURL   string `json:"player_url,omitempty"`
    PlayerType  string `json:"player_type,omitempty"` // iframe, playerjs, etc

    // Метаданные
    Description string `json:"description,omitempty"`
    Director    string `json:"director,omitempty"`
    Duration    string `json:"duration,omitempty"`

    // Schema.org
    SchemaOrg   map[string]interface{} `json:"schema_org,omitempty"`

    // Источник
    SourceURL   string `json:"source_url"`
    ExtractedAt time.Time `json:"extracted_at"`
}
```
