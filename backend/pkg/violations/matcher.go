package violations

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/video-analitics/backend/pkg/meili"
)

var stopWords = map[string]bool{
	"смотреть": true, "онлайн": true, "бесплатно": true, "hd": true,
	"качество": true, "качестве": true, "хорошем": true, "сезон": true,
	"серия": true, "фильм": true, "сериал": true, "lostfilm": true,
	"дубляж": true, "кубик": true, "кубе": true, "русском": true,
	"переводе": true, "1080": true, "720": true, "480": true,
	"watch": true, "free": true, "online": true, "season": true,
	"episode": true, "series": true, "movie": true, "film": true,
	"мобильных": true, "устройствах": true, "ios": true, "android": true,
	"windows": true, "phone": true, "смотрите": true,
}

var yearRegex = regexp.MustCompile(`\b(19[5-9]\d|20[0-2]\d)\b`)

type Matcher struct {
	meili *meili.Client
}

func NewMatcher(meiliClient *meili.Client) *Matcher {
	return &Matcher{meili: meiliClient}
}

// FindMatches ищет все совпадения для контента, возвращая лучший MatchType
// (для обратной совместимости)
func (m *Matcher) FindMatches(ctx context.Context, content ContentInfo) ([]PageMatch, MatchType, error) {
	return m.findMatchesWithSiteFilter(ctx, content, "")
}

// FindMatchesForSite ищет совпадения только на конкретном сайте
func (m *Matcher) FindMatchesForSite(ctx context.Context, content ContentInfo, siteID string) ([]PageMatch, MatchType, error) {
	if siteID == "" {
		return m.FindMatches(ctx, content)
	}
	return m.findMatchesWithSiteFilter(ctx, content, siteID)
}

// FindAllMatches собирает ВСЕ совпадения со всех этапов поиска.
// Каждый PageMatch содержит свой MatchType, показывающий как был найден.
func (m *Matcher) FindAllMatches(ctx context.Context, content ContentInfo) ([]PageMatch, error) {
	return m.findAllMatchesWithSiteFilter(ctx, content, "")
}

// FindAllMatchesForSite собирает все совпадения только на конкретном сайте
func (m *Matcher) FindAllMatchesForSite(ctx context.Context, content ContentInfo, siteID string) ([]PageMatch, error) {
	return m.findAllMatchesWithSiteFilter(ctx, content, siteID)
}

func (m *Matcher) findAllMatchesWithSiteFilter(ctx context.Context, content ContentInfo, siteID string) ([]PageMatch, error) {
	if m.meili == nil {
		return nil, nil
	}

	siteFilter := ""
	if siteID != "" {
		siteFilter = `site_id = "` + siteID + `"`
	}

	seen := make(map[string]bool)
	var allMatches []PageMatch

	addMatches := func(matches []PageMatch, matchType MatchType) {
		for _, m := range matches {
			if !seen[m.PageID] {
				seen[m.PageID] = true
				m.MatchType = matchType
				allMatches = append(allMatches, m)
			}
		}
	}

	// Stage 1: exact match by Kinopoisk ID
	if content.KinopoiskID != "" {
		filter := `kinopoisk_id = "` + content.KinopoiskID + `"`
		if siteFilter != "" {
			filter = filter + " AND " + siteFilter
		}
		matches, err := m.searchByFilter(filter, 10000)
		if err != nil {
			return nil, err
		}
		addMatches(matches, MatchByKinopoisk)
	}

	// Stage 2: exact match by IMDB
	if content.IMDBID != "" {
		filter := `imdb_id = "` + content.IMDBID + `"`
		if siteFilter != "" {
			filter = filter + " AND " + siteFilter
		}
		matches, err := m.searchByFilter(filter, 10000)
		if err != nil {
			return nil, err
		}
		addMatches(matches, MatchByIMDB)
	}

	// Stage 3-5: MAL, Shikimori, MyDramaList (search in links_text)
	for _, idSearch := range []struct {
		id        string
		matchType MatchType
	}{
		{content.MALID, MatchByMAL},
		{content.ShikimoriID, MatchByShikimori},
		{content.MyDramaListID, MatchByMyDramaList},
	} {
		if idSearch.id != "" && len(idSearch.id) >= 3 {
			matches, err := m.searchByIDInLinksText(idSearch.id, siteFilter, idSearch.matchType, 10000)
			if err != nil {
				return nil, err
			}
			addMatches(matches, idSearch.matchType)
		}
	}

	// Stage 6: title + year (structured field)
	if content.Year > 0 && content.Title != "" {
		matches, err := m.searchByTitleAndYearWithSite(content.Title, content.Year, siteFilter, 10000)
		if err != nil {
			return nil, err
		}
		addMatches(matches, MatchByTitleYear)

		if content.OriginalTitle != "" {
			matches, err = m.searchByTitleAndYearWithSite(content.OriginalTitle, content.Year, siteFilter, 10000)
			if err != nil {
				return nil, err
			}
			addMatches(matches, MatchByTitleYear)
		}
	}

	// Stage 7: title only (exact phrase)
	if content.Title != "" {
		matches, err := m.searchExactPhrase(content.Title, siteFilter, 10000)
		if err != nil {
			return nil, err
		}
		addMatches(matches, MatchByTitle)
	}

	if content.OriginalTitle != "" {
		matches, err := m.searchExactPhrase(content.OriginalTitle, siteFilter, 10000)
		if err != nil {
			return nil, err
		}
		addMatches(matches, MatchByTitle)
	}

	// Stage 8: fuzzy title + год в тексте (title/description)
	if content.Year > 0 && content.Title != "" {
		matches, err := m.searchFuzzyWithYearInText(content.Title, content.Year, siteFilter, 10000)
		if err != nil {
			return nil, err
		}
		addMatches(matches, MatchByTitleFuzzyYear)

		if content.OriginalTitle != "" {
			matches, err = m.searchFuzzyWithYearInText(content.OriginalTitle, content.Year, siteFilter, 10000)
			if err != nil {
				return nil, err
			}
			addMatches(matches, MatchByTitleFuzzyYear)
		}
	}

	return allMatches, nil
}

func (m *Matcher) findMatchesWithSiteFilter(ctx context.Context, content ContentInfo, siteID string) ([]PageMatch, MatchType, error) {
	if m.meili == nil {
		return nil, "", nil
	}

	siteFilter := ""
	if siteID != "" {
		siteFilter = `site_id = "` + siteID + `"`
	}

	// Priority 1: exact match by Kinopoisk ID
	if content.KinopoiskID != "" {
		filter := `kinopoisk_id = "` + content.KinopoiskID + `"`
		if siteFilter != "" {
			filter = filter + " AND " + siteFilter
		}
		matches, err := m.searchByFilterWithType(filter, MatchByKinopoisk, 10000)
		if err != nil {
			return nil, "", err
		}
		if len(matches) > 0 {
			return matches, MatchByKinopoisk, nil
		}
	}

	// Priority 2: exact match by IMDB
	if content.IMDBID != "" {
		filter := `imdb_id = "` + content.IMDBID + `"`
		if siteFilter != "" {
			filter = filter + " AND " + siteFilter
		}
		matches, err := m.searchByFilterWithType(filter, MatchByIMDB, 10000)
		if err != nil {
			return nil, "", err
		}
		if len(matches) > 0 {
			return matches, MatchByIMDB, nil
		}
	}

	// Priority 3-5: MAL, Shikimori, MyDramaList (search in links_text)
	for _, idSearch := range []struct {
		id        string
		matchType MatchType
	}{
		{content.MALID, MatchByMAL},
		{content.ShikimoriID, MatchByShikimori},
		{content.MyDramaListID, MatchByMyDramaList},
	} {
		if idSearch.id != "" && len(idSearch.id) >= 3 {
			matches, err := m.searchByIDInLinksText(idSearch.id, siteFilter, idSearch.matchType, 10000)
			if err != nil {
				return nil, "", err
			}
			if len(matches) > 0 {
				return matches, idSearch.matchType, nil
			}
		}
	}

	// Priority 6: title + year
	if content.Year > 0 && content.Title != "" {
		matches, err := m.searchByTitleAndYearWithSiteAndType(content.Title, content.Year, siteFilter, MatchByTitleYear, 10000)
		if err != nil {
			return nil, "", err
		}
		if len(matches) > 0 {
			return matches, MatchByTitleYear, nil
		}

		if content.OriginalTitle != "" {
			matches, err = m.searchByTitleAndYearWithSiteAndType(content.OriginalTitle, content.Year, siteFilter, MatchByTitleYear, 10000)
			if err != nil {
				return nil, "", err
			}
			if len(matches) > 0 {
				return matches, MatchByTitleYear, nil
			}
		}
	}

	// Priority 7: title only (exact phrase)
	if content.Title != "" {
		matches, err := m.searchExactPhraseWithType(content.Title, siteFilter, MatchByTitle, 10000)
		if err != nil {
			return nil, "", err
		}
		if len(matches) > 0 {
			return matches, MatchByTitle, nil
		}
	}

	if content.OriginalTitle != "" {
		matches, err := m.searchExactPhraseWithType(content.OriginalTitle, siteFilter, MatchByTitle, 10000)
		if err != nil {
			return nil, "", err
		}
		if len(matches) > 0 {
			return matches, MatchByTitle, nil
		}
	}

	// Priority 8: fuzzy title + год в тексте (title/description)
	if content.Year > 0 && content.Title != "" {
		matches, err := m.searchFuzzyWithYearInText(content.Title, content.Year, siteFilter, 10000)
		if err != nil {
			return nil, "", err
		}
		if len(matches) > 0 {
			return matches, MatchByTitleFuzzyYear, nil
		}

		if content.OriginalTitle != "" {
			matches, err = m.searchFuzzyWithYearInText(content.OriginalTitle, content.Year, siteFilter, 10000)
			if err != nil {
				return nil, "", err
			}
			if len(matches) > 0 {
				return matches, MatchByTitleFuzzyYear, nil
			}
		}
	}

	return nil, "", nil
}

func (m *Matcher) searchByFilter(filter string, limit int64) ([]PageMatch, error) {
	result, err := m.meili.SearchPages("", filter, limit)
	if err != nil {
		return nil, err
	}
	return hitsToMatches(result.Hits), nil
}

func (m *Matcher) searchByFilterWithType(filter string, matchType MatchType, limit int64) ([]PageMatch, error) {
	result, err := m.meili.SearchPages("", filter, limit)
	if err != nil {
		return nil, err
	}
	return hitsToMatchesWithType(result.Hits, matchType), nil
}

func (m *Matcher) searchByIDInLinksText(id, siteFilter string, matchType MatchType, limit int64) ([]PageMatch, error) {
	if len(id) < 3 {
		return nil, nil
	}

	result, err := m.meili.SearchPages(id, siteFilter, limit)
	if err != nil {
		return nil, err
	}

	var matches []PageMatch
	for _, hit := range result.Hits {
		if strings.Contains(hit.LinksText, id) {
			match := hitToMatch(hit)
			match.MatchType = matchType
			matches = append(matches, match)
		}
	}
	return matches, nil
}

func (m *Matcher) searchByTitleAndYear(title string, year int, limit int64) ([]PageMatch, error) {
	return m.searchByTitleAndYearWithSite(title, year, "", limit)
}

func (m *Matcher) searchByTitleAndYearWithSite(title string, year int, siteFilter string, limit int64) ([]PageMatch, error) {
	title = strings.TrimSpace(title)
	query := `"` + title + `"`
	filter := "year = " + itoa(year)
	if siteFilter != "" {
		filter = filter + " AND " + siteFilter
	}
	result, err := m.meili.SearchPages(query, filter, limit)
	if err != nil {
		return nil, err
	}
	// Пост-фильтрация: убеждаемся что title реально присутствует
	filtered := filterHitsByPhrase(result.Hits, title)
	return hitsToMatches(filtered), nil
}

func (m *Matcher) searchByTitleAndYearWithSiteAndType(title string, year int, siteFilter string, matchType MatchType, limit int64) ([]PageMatch, error) {
	title = strings.TrimSpace(title)
	query := `"` + title + `"`
	filter := "year = " + itoa(year)
	if siteFilter != "" {
		filter = filter + " AND " + siteFilter
	}
	result, err := m.meili.SearchPages(query, filter, limit)
	if err != nil {
		return nil, err
	}
	// Пост-фильтрация: убеждаемся что title реально присутствует
	filtered := filterHitsByPhrase(result.Hits, title)
	return hitsToMatchesWithType(filtered, matchType), nil
}

func (m *Matcher) searchExactPhrase(phrase, extraFilter string, limit int64) ([]PageMatch, error) {
	phrase = strings.TrimSpace(phrase)
	query := `"` + phrase + `"`
	result, err := m.meili.SearchPages(query, extraFilter, limit)
	if err != nil {
		return nil, err
	}
	// Пост-фильтрация: убеждаемся что фраза реально присутствует в title или description
	filtered := filterHitsByPhrase(result.Hits, phrase)
	return hitsToMatches(filtered), nil
}

func (m *Matcher) searchExactPhraseWithType(phrase, extraFilter string, matchType MatchType, limit int64) ([]PageMatch, error) {
	phrase = strings.TrimSpace(phrase)
	query := `"` + phrase + `"`
	result, err := m.meili.SearchPages(query, extraFilter, limit)
	if err != nil {
		return nil, err
	}
	// Пост-фильтрация: убеждаемся что фраза реально присутствует в title или description
	filtered := filterHitsByPhrase(result.Hits, phrase)
	return hitsToMatchesWithType(filtered, matchType), nil
}

// filterHitsByPhrase отфильтровывает результаты, оставляя только те где фраза
// действительно присутствует в title (case-insensitive).
// Description не проверяется - там слишком много мусора и false positives.
func filterHitsByPhrase(hits []meili.PageDocument, phrase string) []meili.PageDocument {
	phraseNorm := normalizeTitle(phrase)
	if phraseNorm == "" {
		return nil
	}
	var filtered []meili.PageDocument
	for _, hit := range hits {
		titleNorm := normalizeTitle(hit.Title)
		if strings.Contains(titleNorm, phraseNorm) {
			filtered = append(filtered, hit)
		}
	}
	return filtered
}

// normalizeTitle очищает title для сравнения:
// - lowercase
// - убирает пробелы по краям
// - убирает кавычки «»"'
// - убирает год в скобках (2013)
// - убирает лишние пробелы
func normalizeTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// Убираем кавычки
	s = strings.ReplaceAll(s, "«", "")
	s = strings.ReplaceAll(s, "»", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "'", "")
	s = strings.ReplaceAll(s, "`", "")
	// Убираем год в скобках (1999)-(2029)
	s = yearInParensRegex.ReplaceAllString(s, "")
	// Убираем лишние пробелы
	s = strings.Join(strings.Fields(s), " ")
	return s
}

var yearInParensRegex = regexp.MustCompile(`\s*\((19[5-9]\d|20[0-2]\d)\)\s*`)

func (m *Matcher) searchFuzzyWithYearInText(title string, year int, extraFilter string, limit int64) ([]PageMatch, error) {
	title = strings.TrimSpace(title)
	result, err := m.meili.SearchPages(title, extraFilter, limit)
	if err != nil {
		return nil, err
	}

	yearStr := strconv.Itoa(year)
	var filtered []PageMatch
	for _, hit := range result.Hits {
		// Проверяем только title - description содержит слишком много мусора
		titleMatch := containsTitleWithoutStopWords(hit.Title, title)
		yearMatch := containsYear(hit.Title, yearStr)
		if titleMatch && yearMatch {
			match := hitToMatch(hit)
			match.MatchType = MatchByTitleFuzzyYear
			filtered = append(filtered, match)
		}
	}
	return filtered, nil
}

func containsTitleWithoutStopWords(text, title string) bool {
	if text == "" || title == "" {
		return false
	}
	textLower := strings.ToLower(text)
	titleLower := strings.ToLower(title)

	if strings.Contains(textLower, titleLower) {
		return true
	}

	titleWords := extractMeaningfulWords(titleLower)
	if len(titleWords) == 0 {
		return false
	}

	for _, word := range titleWords {
		if len(word) < 2 {
			continue
		}
		if !strings.Contains(textLower, word) {
			return false
		}
	}
	return true
}

func extractMeaningfulWords(text string) []string {
	words := strings.Fields(text)
	var meaningful []string
	for _, word := range words {
		word = strings.Trim(word, ".,!?:;\"'()-")
		if len(word) < 2 {
			continue
		}
		if stopWords[word] {
			continue
		}
		if _, err := strconv.Atoi(word); err == nil && len(word) <= 2 {
			continue
		}
		meaningful = append(meaningful, word)
	}
	return meaningful
}

func containsYear(text, year string) bool {
	if text == "" {
		return false
	}
	matches := yearRegex.FindAllString(text, -1)
	for _, m := range matches {
		if m == year {
			return true
		}
	}
	return false
}

func hitToMatch(hit meili.PageDocument) PageMatch {
	return PageMatch{
		PageID:    hit.ID,
		SiteID:    hit.SiteID,
		Domain:    hit.Domain,
		URL:       hit.URL,
		Title:     hit.Title,
		IndexedAt: parseTime(hit.IndexedAt),
	}
}

func hitsToMatches(hits []meili.PageDocument) []PageMatch {
	matches := make([]PageMatch, len(hits))
	for i, hit := range hits {
		matches[i] = PageMatch{
			PageID:    hit.ID,
			SiteID:    hit.SiteID,
			Domain:    hit.Domain,
			URL:       hit.URL,
			Title:     hit.Title,
			IndexedAt: parseTime(hit.IndexedAt),
		}
	}
	return matches
}

func hitsToMatchesWithType(hits []meili.PageDocument, matchType MatchType) []PageMatch {
	matches := make([]PageMatch, len(hits))
	for i, hit := range hits {
		matches[i] = PageMatch{
			PageID:    hit.ID,
			SiteID:    hit.SiteID,
			Domain:    hit.Domain,
			URL:       hit.URL,
			Title:     hit.Title,
			MatchType: matchType,
			IndexedAt: parseTime(hit.IndexedAt),
		}
	}
	return matches
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
