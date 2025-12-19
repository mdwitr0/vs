# Video API провайдеры

## Обзор

Пиратские сайты используют внешние API для получения видео-контента. Каждый провайдер имеет свой формат API и параметры поиска.

---

## 1. Основные провайдеры

### Bazon

```
Base URL: https://bazon.cc/api/search
Docs: https://bazon.cc/

Параметры:
- token (required): API ключ
- kp: Kinopoisk ID
- imdb: IMDb ID

Пример:
GET https://bazon.cc/api/search?token=TOKEN&kp=252188

Ответ:
{
  "results": [{
    "id": "...",
    "title": "...",
    "iframe": "https://bazon.cc/embed/..."
  }]
}
```

### VideoCDN

```
Base URL: https://videocdn.tv/api/short
Docs: https://videocdn.tv/

Параметры:
- api_token (required): API ключ
- kinopoisk_id: KP ID
- imdb_id: IMDb ID

Пример:
GET https://videocdn.tv/api/short?api_token=TOKEN&kinopoisk_id=252188

Embed URL формат:
https://videocdn.tv/embed/ID
```

### Kodik

```
Base URL: https://kodikapi.com/search
Docs: https://kodik.info/

Параметры:
- token (required): API ключ
- kinopoisk_id: KP ID
- imdb_id: IMDb ID
- title: название для поиска

Пример:
GET https://kodikapi.com/search?token=TOKEN&kinopoisk_id=252188

Ответ:
{
  "results": [{
    "id": "...",
    "link": "//kodik.info/video/...",
    "quality": "720p"
  }]
}
```

### Collaps (bhcesh)

```
Base URL: https://api.bhcesh.me/list
Alternative: https://api.bhcesh.me/embed

Параметры:
- token (required): API ключ
- kinopoisk_id: KP ID

Пример:
GET https://api.bhcesh.me/list?token=TOKEN&kinopoisk_id=252188
```

### Alloha

```
Base URL: https://api.alloha.tv/
Параметры:
- token (required): API ключ
- kp: Kinopoisk ID
- imdb: IMDb ID

Пример:
GET https://api.alloha.tv/?token=TOKEN&kp=252188
```

### HDVB

```
Base URL: https://apivb.info/api/videos.json
Параметры:
- token (required): API ключ
- id_kp: Kinopoisk ID
- id_imdb: IMDb ID

Пример:
GET https://apivb.info/api/videos.json?token=TOKEN&id_kp=252188
```

---

## 2. Embed URL паттерны

Типичные форматы embed URL от провайдеров:

```
Bazon:
https://bazon.cc/embed/{hash}

VideoCDN:
https://videocdn.tv/embed/{id}
https://videocdn.tv/api/player?imdb_id={imdb}

Kodik:
//kodik.info/video/{id}/{hash}/720p
//kodik.cc/video/{id}/{hash}

Collaps:
https://api.bhcesh.me/embed/{id}

Alloha:
https://alloha.tv/embed/{id}
```

---

## 3. Regex для извлечения провайдера из iframe

```go
var providerPatterns = map[string]*regexp.Regexp{
    "bazon":    regexp.MustCompile(`bazon\.cc/embed/([a-zA-Z0-9]+)`),
    "videocdn": regexp.MustCompile(`videocdn\.tv/(?:embed|api/player)[/?]([^"'\s]+)`),
    "kodik":    regexp.MustCompile(`kodik\.(info|cc)/video/(\d+)/([a-zA-Z0-9]+)`),
    "collaps":  regexp.MustCompile(`(?:api\.)?bhcesh\.me/embed/([a-zA-Z0-9]+)`),
    "alloha":   regexp.MustCompile(`alloha\.tv/embed/([a-zA-Z0-9]+)`),
    "hdvb":     regexp.MustCompile(`(?:hdvb|apivb)\.(?:co|info)/([^"'\s]+)`),
}

func detectProvider(iframeURL string) (provider string, id string) {
    for name, pattern := range providerPatterns {
        if matches := pattern.FindStringSubmatch(iframeURL); len(matches) > 1 {
            return name, matches[1]
        }
    }
    return "unknown", ""
}
```

---

## 4. Fallback стратегия (CinemaPress подход)

CinemaPress использует последовательный опрос провайдеров:

```go
type VideoProvider struct {
    Name     string
    BaseURL  string
    Priority int
}

var providers = []VideoProvider{
    {"bazon", "https://bazon.cc/api/search", 1},
    {"videocdn", "https://videocdn.tv/api/short", 2},
    {"kodik", "https://kodikapi.com/search", 3},
    {"collaps", "https://api.bhcesh.me/list", 4},
    {"alloha", "https://api.alloha.tv/", 5},
}

// Поиск по KP ID с fallback
func findVideo(kpID string) (*VideoResult, error) {
    for _, provider := range providers {
        result, err := queryProvider(provider, kpID)
        if err == nil && result != nil {
            return result, nil
        }
    }
    return nil, ErrNotFound
}
```

---

## 5. Общие данные и справочники

### TMDB API

```
Base URL: https://api.themoviedb.org/3/
Docs: https://developers.themoviedb.org/3

Поиск фильма:
GET /search/movie?query={title}&language=ru

Детали фильма:
GET /movie/{tmdb_id}?language=ru&append_to_response=videos,external_ids

External IDs:
GET /movie/{tmdb_id}/external_ids
Response: { imdb_id, tvdb_id, ... }
```

### Kinopoisk Unofficial API

```
Base URL: https://kinopoiskapiunofficial.tech/api/

Поиск:
GET /v2.1/films/search-by-keyword?keyword={title}

Детали:
GET /v2.2/films/{kp_id}

Headers:
X-API-KEY: your_key
```

---

## 6. Использование в детекторе

Для нашего indexer важно:

1. **Не вызывать API провайдеров** — у нас нет токенов
2. **Извлекать iframe URL** — из HTML страницы сайта
3. **Определять провайдера** — по паттерну iframe URL
4. **Извлекать ID** — из iframe для возможной идентификации контента

```go
type PlayerInfo struct {
    IframeURL string `json:"iframe_url"`
    Provider  string `json:"provider"`  // bazon, videocdn, kodik, etc
    VideoID   string `json:"video_id"`  // ID внутри провайдера
}

func extractPlayerInfo(html string) *PlayerInfo {
    iframeURL := findPlayerIframe(html)
    if iframeURL == "" {
        return nil
    }

    provider, videoID := detectProvider(iframeURL)

    return &PlayerInfo{
        IframeURL: iframeURL,
        Provider:  provider,
        VideoID:   videoID,
    }
}
```

---

## 7. Дополнительные плееры

### Playerjs

Популярный JS-плеер на пиратских сайтах:

```javascript
new Playerjs({
    id: "player",
    file: "https://example.com/video.m3u8"
});
```

**Извлечение:**
```go
var playerjsRegex = regexp.MustCompile(`Playerjs\(\{[^}]*file:\s*["']([^"']+)["']`)
```

### Kinobalancer

WordPress плагин для балансировки плееров:

```html
<div class="kinobalancer">
    <div class="kinobalancer-frame">
        <iframe src="..."></iframe>
    </div>
</div>
```

---

## 8. Таблица провайдеров

| Провайдер | KP ID | IMDb | TMDB | Качество | Популярность |
|-----------|-------|------|------|----------|--------------|
| Bazon | + | + | - | До 1080p | Высокая |
| VideoCDN | + | + | - | До 4K | Высокая |
| Kodik | + | + | - | До 720p | Средняя |
| Collaps | + | - | - | До 1080p | Средняя |
| Alloha | + | + | - | До 1080p | Средняя |
| HDVB | + | + | - | До 1080p | Низкая |
