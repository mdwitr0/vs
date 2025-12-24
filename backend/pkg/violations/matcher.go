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
	// Предлоги и служебные слова
	"в": true, "на": true, "без": true, "с": true,
	"in": true, "on": true, "the": true, "a": true, "an": true,
}

var commonTitleWords = map[string]bool{
	"год": true, "время": true, "мир": true, "дом": true,
	"свет": true, "день": true, "ночь": true, "жизнь": true,
	"путь": true, "игра": true, "love": true, "life": true,
	"time": true, "home": true, "world": true, "light": true,
	"game": true, "war": true, "day": true, "night": true,
	"любовь": true, "война": true, "друзья": true, "семья": true,
}

var yearRegex = regexp.MustCompile(`\b(19[5-9]\d|20[0-2]\d)\b`)

var (
	malURLRegex       = regexp.MustCompile(`myanimelist\.net/anime/(\d+)`)
	shikimoriURLRegex = regexp.MustCompile(`shikimori\.(one|me|org)/animes/[a-z]*(\d+)`)
	mdlURLRegex       = regexp.MustCompile(`mydramalist\.com/(\d+)`)
)

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

		if isValidTitle(content.OriginalTitle) {
			matches, err = m.searchByTitleAndYearWithSite(content.OriginalTitle, content.Year, siteFilter, 10000)
			if err != nil {
				return nil, err
			}
			addMatches(matches, MatchByTitleYear)
		}
	}

	// Stage 7: title only (exact phrase)
	// Для однословных названий пропускаем - слишком много ложных срабатываний
	// Используем только kinopoisk_id/imdb_id/title+year для них
	if isValidTitle(content.Title) && !isSingleWordTitle(content.Title) {
		matches, err := m.searchExactPhrase(content.Title, siteFilter, 10000)
		if err != nil {
			return nil, err
		}
		addMatches(matches, MatchByTitle)
	}

	if isValidTitle(content.OriginalTitle) && !isSingleWordTitle(content.OriginalTitle) {
		matches, err := m.searchExactPhrase(content.OriginalTitle, siteFilter, 10000)
		if err != nil {
			return nil, err
		}
		addMatches(matches, MatchByTitle)
	}

	// Stage 8: fuzzy title + год в тексте (title/description)
	if content.Year > 0 && isValidTitle(content.Title) {
		matches, err := m.searchFuzzyWithYearInText(content.Title, content.Year, siteFilter, 10000)
		if err != nil {
			return nil, err
		}
		addMatches(matches, MatchByTitleFuzzyYear)

		if isValidTitle(content.OriginalTitle) {
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
	if content.Year > 0 && isValidTitle(content.Title) {
		matches, err := m.searchByTitleAndYearWithSiteAndType(content.Title, content.Year, siteFilter, MatchByTitleYear, 10000)
		if err != nil {
			return nil, "", err
		}
		if len(matches) > 0 {
			return matches, MatchByTitleYear, nil
		}

		if isValidTitle(content.OriginalTitle) {
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
	// Пропускаем для однословных названий - слишком много ложных срабатываний
	if isValidTitle(content.Title) && !isSingleWordTitle(content.Title) {
		matches, err := m.searchExactPhraseWithType(content.Title, siteFilter, MatchByTitle, 10000)
		if err != nil {
			return nil, "", err
		}
		if len(matches) > 0 {
			return matches, MatchByTitle, nil
		}
	}

	if isValidTitle(content.OriginalTitle) && !isSingleWordTitle(content.OriginalTitle) {
		matches, err := m.searchExactPhraseWithType(content.OriginalTitle, siteFilter, MatchByTitle, 10000)
		if err != nil {
			return nil, "", err
		}
		if len(matches) > 0 {
			return matches, MatchByTitle, nil
		}
	}

	// Priority 8: fuzzy title + год в тексте (title/description)
	if content.Year > 0 && isValidTitle(content.Title) {
		matches, err := m.searchFuzzyWithYearInText(content.Title, content.Year, siteFilter, 10000)
		if err != nil {
			return nil, "", err
		}
		if len(matches) > 0 {
			return matches, MatchByTitleFuzzyYear, nil
		}

		if isValidTitle(content.OriginalTitle) {
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
	if len(id) < 2 {
		return nil, nil
	}

	var regex *regexp.Regexp
	switch matchType {
	case MatchByMAL:
		regex = malURLRegex
	case MatchByShikimori:
		regex = shikimoriURLRegex
	case MatchByMyDramaList:
		regex = mdlURLRegex
	default:
		return nil, nil
	}

	result, err := m.meili.SearchPages(id, siteFilter, limit)
	if err != nil {
		return nil, err
	}

	var matches []PageMatch
	for _, hit := range result.Hits {
		if containsIDInURL(hit.LinksText, id, regex) {
			match := hitToMatch(hit)
			match.MatchType = matchType
			matches = append(matches, match)
		}
	}
	return matches, nil
}

func containsIDInURL(linksText, id string, regex *regexp.Regexp) bool {
	matches := regex.FindAllStringSubmatch(linksText, -1)
	for _, m := range matches {
		if len(m) > 0 && m[len(m)-1] == id {
			return true
		}
	}
	return false
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
// Для коротких названий используется строгая проверка:
// - название должно начинать заголовок
// - после названия могут быть только стоп-слова (смотреть, онлайн и т.д.) или год
// - это отсекает "Между нами горы" при поиске "Между нами"
func filterHitsByPhrase(hits []meili.PageDocument, phrase string) []meili.PageDocument {
	phraseNorm := normalizeTitle(phrase)
	if phraseNorm == "" {
		return nil
	}

	shortPhrase := isShortPhrase(phrase)

	var filtered []meili.PageDocument
	for _, hit := range hits {
		titleNorm := normalizeTitle(hit.Title)
		if shortPhrase {
			// Для коротких названий требуем:
			// 1. Название начинает заголовок
			// 2. После названия идут только стоп-слова или ничего
			// "Между нами горы" ≠ "Между нами" (горы - не стоп-слово)
			// "Между нами смотреть онлайн" = "Между нами" (всё стоп-слова)
			if titleStartsWithPhraseAndOnlyStopWordsFollow(titleNorm, phraseNorm) {
				filtered = append(filtered, hit)
			}
		} else {
			// Для длинных названий - обычный substring match
			if strings.Contains(titleNorm, phraseNorm) {
				filtered = append(filtered, hit)
			}
		}
	}
	return filtered
}

func isShortOrCommonTitle(title string) bool {
	titleLower := strings.ToLower(strings.TrimSpace(title))
	runes := []rune(titleLower)
	if len(runes) <= 5 {
		return true
	}
	return commonTitleWords[titleLower]
}

func isShortPhrase(phrase string) bool {
	phrase = strings.TrimSpace(phrase)
	runes := []rune(phrase)
	// Короткая фраза: <=12 символов
	// "Из ада" (6 символов) - short
	// "Властелин колец" (15 символов) - NOT short
	return len(runes) <= 12
}

func titleStartsWithPhrase(title, phrase string) bool {
	// Проверяем что title начинается с phrase
	if !strings.HasPrefix(title, phrase) {
		return false
	}
	// Проверяем word boundary после phrase
	if len(title) == len(phrase) {
		return true // точное совпадение
	}
	nextChar := title[len(phrase)]
	// Следующий символ должен быть разделителем
	return nextChar == ' ' || nextChar == ':' || nextChar == '/' ||
		nextChar == '.' || nextChar == ',' || nextChar == '-' ||
		nextChar == '(' || nextChar == ')' || nextChar == '!'
}

// titleStartsWithPhraseAndOnlyStopWordsFollow проверяет что:
// 1. title начинается с phrase
// 2. После phrase идут только стоп-слова (смотреть, онлайн, бесплатно и т.д.)
// Это отсекает "Между нами горы" при поиске "Между нами" (горы - не стоп-слово)
// Но пропускает "Между нами смотреть онлайн" (всё после - стоп-слова)
func titleStartsWithPhraseAndOnlyStopWordsFollow(title, phrase string) bool {
	if !strings.HasPrefix(title, phrase) {
		return false
	}
	if len(title) == len(phrase) {
		return true // точное совпадение
	}

	// Получаем остаток после фразы
	rest := title[len(phrase):]

	// Убираем разделители в начале
	rest = strings.TrimLeft(rest, " :/-.,()!?")
	if rest == "" {
		return true
	}

	// Все оставшиеся слова должны быть стоп-словами или числами
	words := strings.Fields(rest)
	for _, word := range words {
		word = strings.Trim(word, ".,!?:;\"'()-«»")
		word = strings.ToLower(word)
		if word == "" {
			continue
		}
		// Числа ОК (года, сезоны, разрешение)
		if _, err := strconv.Atoi(word); err == nil {
			continue
		}
		// Если слово не стоп-слово - это другой фильм/контент
		if !stopWords[word] {
			return false
		}
	}
	return true
}

func isSingleWordTitle(title string) bool {
	title = strings.TrimSpace(title)
	words := strings.Fields(title)
	return len(words) == 1
}

func isSingleWordShortTitle(title string) bool {
	return isSingleWordTitle(title) && isShortOrCommonTitle(title)
}

func containsWholeWord(text, word string) bool {
	words := strings.Fields(text)
	for _, w := range words {
		if w == word {
			return true
		}
	}
	return false
}

func titleStartsWithWord(text, word string) bool {
	words := strings.Fields(text)
	if len(words) == 0 {
		return false
	}
	firstWord := strings.Trim(words[0], ".,!?:;\"'()-«»")
	return firstWord == word
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

// isValidTitle проверяет что название не является мусорным значением
func isValidTitle(title string) bool {
	title = strings.TrimSpace(title)
	if title == "" {
		return false
	}
	// Мусорные значения: "-", "--", "...", "N/A", "n/a", "TBA" и т.д.
	invalidTitles := map[string]bool{
		"-": true, "--": true, "---": true,
		"...": true, "..": true, ".": true,
		"n/a": true, "na": true, "tba": true, "tbd": true,
		"unknown": true, "untitled": true, "none": true,
	}
	return !invalidTitles[strings.ToLower(title)]
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
