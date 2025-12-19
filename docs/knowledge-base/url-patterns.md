# URL паттерны пиратских сайтов

## Обзор

Документ описывает типичные URL-паттерны на пиратских сайтах и способы извлечения ID контента из URL.

---

## 1. Паттерны по CMS

### DLE (DataLife Engine)

```
Формат 1: ID-slug
/1535-hatiko-samyj-vernyj-drug-2009.html
/4322-hatiko-samyj-vernyj-drug-2009.html
Regex: /(\d+)-[^/]+\.html$

Формат 2: Категория + slug
/films/drama/hatiko-samyj-vernyj-drug.html
/movie/hatiko-2009.html
Regex: /[^/]+/([^/]+)\.html$

Формат 3: Год в пути
/2009/06/hatiko.html
Regex: /(\d{4})/\d{2}/([^/]+)\.html$

Формат 4: newsid параметр
/index.php?newsid=1535
Regex: newsid=(\d+)

Пагинация:
/page/2/
/films/page/5/
```

### WordPress

```
Формат 1: Дата + slug (permalinks)
/2009/06/13/hatiko-samyj-vernyj-drug/
Regex: /\d{4}/\d{2}/\d{2}/([^/]+)/?$

Формат 2: Просто slug
/hatiko-samyj-vernyj-drug/
/movie/hatiko/
Regex: /([^/]+)/?$

Формат 3: ID fallback
/?p=12345
Regex: \?p=(\d+)

Формат 4: Категория + slug
/movies/hatiko-samyj-vernyj-drug/
/films/drama/hatiko/
```

### CinemaPress

```
Формат: /movie/cpID или /film/cpID
/movie/cp252188
/film/cp252188

Regex: /(?:movie|film)/cp(\d+)

Сериалы:
/tv/cp123456
/serial/cp123456
```

### uCoz

```
Формат: /section/category/ID
/load/filmy/drama/3-1-0-238
/publ/news/1-1-0-500
/news/2024-01-15-123

Regex секции load:
/load/[^/]+/[^/]+/(\d+-\d+-\d+-\d+)
```

---

## 2. Паттерны с внешними ID в URL

Некоторые сайты включают KP/IMDb ID прямо в URL:

```
С Kinopoisk ID:
/film/kp252188
/movie/252188-hatiko
/watch/kp-252188

Regex: /(?:film|movie|watch)/(?:kp[-_]?)?(\d{5,8})

С IMDb ID:
/movie/tt0266543
/film/imdb-tt0266543

Regex: /(tt\d{7,8})
```

---

## 3. Специальные паттерны

### Поддомен на фильм

```
Домен: hatiko-samyj-vernyj-drug-2009.kino-lordfilm2.org
Основной домен: kino-lordfilm2.org
Slug в поддомене: hatiko-samyj-vernyj-drug-2009

Извлечение:
func extractSubdomainSlug(host string) (mainDomain, slug string) {
    parts := strings.Split(host, ".")
    if len(parts) >= 3 {
        // Первая часть - slug, остальное - домен
        slug = parts[0]
        mainDomain = strings.Join(parts[1:], ".")
    }
    return
}
```

### Числовой ID в пути

```
/films/12486
/movie/278865
/watch/1013343

Regex: /(?:films?|movies?|watch)/(\d+)
```

### Slug с годом

```
hatiko-samyj-vernyj-drug-2009
the-matrix-1999

Regex извлечения года из slug:
([a-z-]+)-(\d{4})$
```

---

## 4. Извлечение ID из URL

### Универсальный экстрактор

```go
type URLPattern struct {
    Name    string
    Pattern *regexp.Regexp
    IDGroup int // индекс группы с ID
}

var urlPatterns = []URLPattern{
    // DLE с числовым ID
    {"dle_numeric", regexp.MustCompile(`/(\d+)-[^/]+\.html$`), 1},

    // WordPress с ID
    {"wp_id", regexp.MustCompile(`\?p=(\d+)`), 1},

    // Числовой путь
    {"numeric_path", regexp.MustCompile(`/(?:films?|movies?|watch|video)/(\d+)`), 1},

    // CinemaPress
    {"cinemapress", regexp.MustCompile(`/(?:movie|film|tv)/cp(\d+)`), 1},

    // С KP ID
    {"kp_in_url", regexp.MustCompile(`/(?:kp[-_]?)?(\d{5,8})(?:[/-]|$)`), 1},

    // IMDb в URL
    {"imdb_in_url", regexp.MustCompile(`/(tt\d{7,8})`), 1},

    // uCoz load
    {"ucoz_load", regexp.MustCompile(`/load/[^/]+/[^/]+/(\d+-\d+-\d+-\d+)`), 1},
}

func extractIDFromURL(url string) (idType string, id string) {
    for _, p := range urlPatterns {
        if matches := p.Pattern.FindStringSubmatch(url); len(matches) > p.IDGroup {
            return p.Name, matches[p.IDGroup]
        }
    }
    return "", ""
}
```

### Извлечение slug

```go
func extractSlugFromURL(rawURL string) string {
    u, err := url.Parse(rawURL)
    if err != nil {
        return ""
    }

    path := strings.TrimSuffix(u.Path, "/")
    path = strings.TrimSuffix(path, ".html")

    parts := strings.Split(path, "/")
    if len(parts) > 0 {
        return parts[len(parts)-1]
    }

    return ""
}

// Примеры:
// /1535-hatiko-samyj-vernyj-drug-2009.html → 1535-hatiko-samyj-vernyj-drug-2009
// /films/drama/hatiko/ → hatiko
// /movie/12345 → 12345
```

---

## 5. Sitemap URL анализ

### Типичная структура sitemap

```xml
<urlset>
  <url>
    <loc>https://example.com/1535-hatiko-2009.html</loc>
    <lastmod>2024-01-15</lastmod>
    <priority>0.7</priority>
  </url>
</urlset>
```

### Фильтрация URL контента

```go
// Паттерны URL, которые являются страницами фильмов
var contentURLPatterns = []*regexp.Regexp{
    regexp.MustCompile(`/\d+-[^/]+\.html$`),           // DLE: /123-slug.html
    regexp.MustCompile(`/(?:film|movie|video)/\d+`),   // /film/123
    regexp.MustCompile(`/(?:films|movies)/[^/]+$`),    // /films/slug
}

// Паттерны URL, которые НЕ являются контентом (исключить)
var excludePatterns = []*regexp.Regexp{
    regexp.MustCompile(`/page/\d+`),                   // Пагинация
    regexp.MustCompile(`/category/`),                  // Категории
    regexp.MustCompile(`/tag/`),                       // Теги
    regexp.MustCompile(`/author/`),                    // Авторы
    regexp.MustCompile(`/\d{4}/\d{2}/?$`),             // Архивы по дате
    regexp.MustCompile(`(?i)/(?:about|contact|dmca)`), // Служебные
}

func isContentURL(url string) bool {
    // Проверить исключения
    for _, pattern := range excludePatterns {
        if pattern.MatchString(url) {
            return false
        }
    }

    // Проверить паттерны контента
    for _, pattern := range contentURLPatterns {
        if pattern.MatchString(url) {
            return true
        }
    }

    return false
}
```

---

## 6. Примеры реальных URL

| Сайт | URL | Тип | ID |
|------|-----|-----|-----|
| zona-films.video | `/1535-hatiko-samyj-vernyj-drug-2009.html` | DLE numeric | 1535 |
| zona-novinki.bar | `/4322-hatiko-samyj-vernyj-drug-2009.html` | DLE numeric | 4322 |
| mi.anwap.today | `/films/12486` | numeric path | 12486 |
| kinomore.netlify.app | `/film/1013343` | numeric path | 1013343 |
| pravfilms.ru | `/load/.../3-1-0-238` | uCoz | 3-1-0-238 |
| kino-lordfilm2.org | subdomain: `hatiko-...` | subdomain slug | — |

---

## 7. Нормализация URL

```go
func normalizeURL(rawURL string) string {
    u, err := url.Parse(rawURL)
    if err != nil {
        return rawURL
    }

    // Удалить trailing slash
    u.Path = strings.TrimSuffix(u.Path, "/")

    // Удалить www
    u.Host = strings.TrimPrefix(u.Host, "www.")

    // Удалить фрагмент
    u.Fragment = ""

    // Удалить tracking параметры
    q := u.Query()
    trackingParams := []string{"utm_source", "utm_medium", "utm_campaign", "ref", "fbclid"}
    for _, param := range trackingParams {
        q.Del(param)
    }
    u.RawQuery = q.Encode()

    return u.String()
}
```

---

## 8. Определение типа страницы по URL

```go
type PageType string

const (
    PageTypeMovie    PageType = "movie"
    PageTypeSeries   PageType = "series"
    PageTypeCategory PageType = "category"
    PageTypeSearch   PageType = "search"
    PageTypeHome     PageType = "home"
    PageTypeOther    PageType = "other"
)

func detectPageType(url string) PageType {
    // Главная
    if regexp.MustCompile(`^https?://[^/]+/?$`).MatchString(url) {
        return PageTypeHome
    }

    // Поиск
    if strings.Contains(url, "search") || strings.Contains(url, "?s=") || strings.Contains(url, "do=search") {
        return PageTypeSearch
    }

    // Категория
    if regexp.MustCompile(`/(?:category|genre|year|country)/`).MatchString(url) {
        return PageTypeCategory
    }

    // Сериал
    if regexp.MustCompile(`/(?:serial|series|tv-series|show)/`).MatchString(url) {
        return PageTypeSeries
    }

    // Фильм (по умолчанию для контентных URL)
    if isContentURL(url) {
        return PageTypeMovie
    }

    return PageTypeOther
}
```
