# Knowledge Base — Indexer Service

База знаний для разработки сервиса индексации пиратских видео-сайтов.

## Документы

| Документ | Описание |
|----------|----------|
| [tech-detection.md](tech-detection.md) | Детекция CMS и технологий (DLE, WordPress, CinemaPress, uCoz) |
| [data-extraction.md](data-extraction.md) | Извлечение данных со страниц (title, year, external IDs, player) |
| [video-providers.md](video-providers.md) | Video API провайдеры (Bazon, Kodik, VideoCDN, etc) |
| [url-patterns.md](url-patterns.md) | URL паттерны и извлечение ID из URL |
| [data-models.md](data-models.md) | Структуры данных и API DTO |

---

## Краткая справка

### Поддерживаемые CMS

| CMS | Маркер детекции | Рендеринг |
|-----|-----------------|-----------|
| DLE | `var dle_root` | SSR |
| WordPress | `/wp-content/`, `wp-admin` | SSR |
| CinemaPress | Header `X-Powered-By: CinemaPress` | SSR |
| uCoz | `window.uCoz` | SSR |

### Внешние ID

| ID | Формат | Regex |
|----|--------|-------|
| Kinopoisk | число (5-8 цифр) | `kinopoisk\.ru/film/(\d+)` |
| IMDb | tt + 7-8 цифр | `imdb\.com/title/(tt\d+)` |
| TMDB | число | `themoviedb\.org/movie/(\d+)` |

### Video провайдеры

| Провайдер | Embed паттерн |
|-----------|---------------|
| Bazon | `bazon.cc/embed/` |
| VideoCDN | `videocdn.tv/embed/` |
| Kodik | `kodik.info/video/` |
| Collaps | `bhcesh.me/embed/` |

---

## Тестовые сайты

```
# DLE
https://zona-films.video/1535-hatiko-samyj-vernyj-drug-2009.html
https://zona-novinki.bar/4322-hatiko-samyj-vernyj-drug-2009.html

# WordPress
https://hatiko-lordfillm.ru/

# Custom
https://mi.anwap.today/films/12486
https://w140.zona.plus/movies/hatiko-samyi-vernyi-drug

# Next.js (SPA SSR)
https://kinomore.netlify.app/film/1013343

# uCoz
https://pravfilms.ru/load/filmy/.../3-1-0-238

# Поддомен на фильм
https://hatiko-samyj-vernyj-drug-2009.kino-lordfilm2.org/
```

---

## Использование

Перед началом работы над задачей:

1. Прочитать релевантные документы из knowledge-base
2. Использовать готовые regex и селекторы
3. После реализации — обновить документы новыми находками

```bash
# Структура
docs/knowledge-base/
├── README.md           # Этот файл
├── tech-detection.md   # Детекция технологий
├── data-extraction.md  # Извлечение данных
├── video-providers.md  # Video API
├── url-patterns.md     # URL паттерны
└── data-models.md      # Структуры данных
```
