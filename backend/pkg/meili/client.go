package meili

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/meilisearch/meilisearch-go"
	"github.com/video-analitics/backend/pkg/logger"
)

const (
	PagesIndex = "pages"
)

// PageDocument представляет документ страницы в Meilisearch
type PageDocument struct {
	ID            string   `json:"id"`
	SiteID        string   `json:"site_id"`
	Domain        string   `json:"domain"`
	URL           string   `json:"url"`
	Title         string   `json:"title"`
	Description   string   `json:"description"`
	MainText      string   `json:"main_text"`
	Year          int      `json:"year,omitempty"`
	KinopoiskID   string   `json:"kinopoisk_id,omitempty"`
	IMDBID        string   `json:"imdb_id,omitempty"`
	MALID         string   `json:"mal_id,omitempty"`
	ShikimoriID   string   `json:"shikimori_id,omitempty"`
	MyDramaListID string   `json:"mydramalist_id,omitempty"`
	LinksText     string   `json:"links_text,omitempty"`
	PlayerURLs    []string `json:"player_urls,omitempty"`
	IndexedAt     string   `json:"indexed_at"`
}

// Client обёртка над meilisearch-go клиентом
type Client struct {
	client meilisearch.ServiceManager
}

// New создаёт новый клиент Meilisearch
func New(url, apiKey string) (*Client, error) {
	client := meilisearch.New(url, meilisearch.WithAPIKey(apiKey))

	// Проверяем соединение
	if _, err := client.Health(); err != nil {
		return nil, err
	}

	c := &Client{client: client}

	// Настраиваем индексы
	if err := c.setupIndexes(); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Client) setupIndexes() error {
	log := logger.Log

	_, err := c.client.CreateIndex(&meilisearch.IndexConfig{
		Uid:        PagesIndex,
		PrimaryKey: "id",
	})
	if err != nil {
		log.Debug().Str("index", PagesIndex).Msg("index already exists")
	} else {
		log.Info().Str("index", PagesIndex).Msg("index created")
	}

	pagesIndex := c.client.Index(PagesIndex)

	// Получаем текущие настройки
	currentSettings, err := pagesIndex.GetSettings()
	if err != nil {
		log.Warn().Err(err).Msg("failed to get current settings, will update all")
		currentSettings = &meilisearch.Settings{}
	}

	// 1. Searchable attributes (title, IDs и links_text для поиска по ID в ссылках)
	searchable := []string{"kinopoisk_id", "imdb_id", "title", "links_text"}
	if !stringSlicesEqual(currentSettings.SearchableAttributes, searchable) {
		if _, err := pagesIndex.UpdateSearchableAttributes(&searchable); err != nil {
			log.Warn().Err(err).Msg("failed to update searchable attributes")
		} else {
			log.Info().Strs("attrs", searchable).Msg("searchable attributes updated")
		}
	}

	// 2. Filterable attributes
	filterable := []string{"site_id", "domain", "year", "kinopoisk_id", "imdb_id", "mal_id", "shikimori_id", "mydramalist_id"}
	if !stringSlicesEqual(currentSettings.FilterableAttributes, filterable) {
		filterableIface := make([]interface{}, len(filterable))
		for i, v := range filterable {
			filterableIface[i] = v
		}
		if _, err := pagesIndex.UpdateFilterableAttributes(&filterableIface); err != nil {
			log.Warn().Err(err).Msg("failed to update filterable attributes")
		} else {
			log.Info().Strs("attrs", filterable).Msg("filterable attributes updated")
		}
	}

	// 3. Ranking rules
	rankingRules := []string{
		"words",
		"typo",
		"exactness",
		"proximity",
		"attribute",
		"sort",
	}
	if !stringSlicesEqual(currentSettings.RankingRules, rankingRules) {
		if _, err := pagesIndex.UpdateRankingRules(&rankingRules); err != nil {
			log.Warn().Err(err).Msg("failed to update ranking rules")
		} else {
			log.Info().Strs("rules", rankingRules).Msg("ranking rules updated")
		}
	}

	// 4. Typo tolerance
	typoTolerance := meilisearch.TypoTolerance{
		Enabled:             true,
		DisableOnAttributes: []string{"kinopoisk_id", "imdb_id", "links_text"},
		MinWordSizeForTypos: meilisearch.MinWordSizeForTypos{
			OneTypo:  5,
			TwoTypos: 9,
		},
	}
	if !typoToleranceEqual(currentSettings.TypoTolerance, &typoTolerance) {
		if _, err := pagesIndex.UpdateTypoTolerance(&typoTolerance); err != nil {
			log.Warn().Err(err).Msg("failed to update typo tolerance")
		} else {
			log.Info().Msg("typo tolerance updated")
		}
	}

	log.Info().Str("index", PagesIndex).Msg("meilisearch index configured")
	return nil
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func typoToleranceEqual(a, b *meilisearch.TypoTolerance) bool {
	if a == nil || b == nil {
		return false
	}
	if a.Enabled != b.Enabled {
		return false
	}
	if !stringSlicesEqual(a.DisableOnAttributes, b.DisableOnAttributes) {
		return false
	}
	if a.MinWordSizeForTypos.OneTypo != b.MinWordSizeForTypos.OneTypo {
		return false
	}
	if a.MinWordSizeForTypos.TwoTypos != b.MinWordSizeForTypos.TwoTypos {
		return false
	}
	return true
}

// IndexPage индексирует страницу
func (c *Client) IndexPage(doc *PageDocument) error {
	docs := []map[string]interface{}{docToMap(doc)}
	pk := "id"
	_, err := c.client.Index(PagesIndex).AddDocuments(docs, &pk)
	return err
}

// IndexPages индексирует несколько страниц
func (c *Client) IndexPages(docs []PageDocument) error {
	if len(docs) == 0 {
		return nil
	}
	maps := make([]map[string]interface{}, len(docs))
	for i := range docs {
		maps[i] = docToMap(&docs[i])
	}
	pk := "id"
	_, err := c.client.Index(PagesIndex).AddDocuments(maps, &pk)
	return err
}

// SearchResult результат поиска
type SearchResult struct {
	Hits             []PageDocument `json:"hits"`
	TotalHits        int64          `json:"totalHits"`
	ProcessingTimeMs int64          `json:"processingTimeMs"`
}

// SearchPages ищет страницы по запросу
func (c *Client) SearchPages(query string, filters string, limit int64) (*SearchResult, error) {
	searchParams := &meilisearch.SearchRequest{
		Query: query,
		Limit: limit,
	}
	if filters != "" {
		searchParams.Filter = filters
	}

	resp, err := c.client.Index(PagesIndex).Search(query, searchParams)
	if err != nil {
		return nil, err
	}

	result := &SearchResult{
		TotalHits:        resp.EstimatedTotalHits,
		ProcessingTimeMs: resp.ProcessingTimeMs,
	}

	// Конвертируем hits
	for _, hit := range resp.Hits {
		doc := hitToPageDocument(hit)
		result.Hits = append(result.Hits, doc)
	}

	return result, nil
}

// SearchPagesByContent ищет страницы, соответствующие контенту
func (c *Client) SearchPagesByContent(title, originalTitle, kpid, imdbID string, limit int64) (*SearchResult, error) {
	return c.SearchPagesByContentWithFilter(title, originalTitle, kpid, imdbID, "", limit)
}

// SearchPagesByContentWithFilter ищет страницы с приоритетами:
// 1. Точный матч по KPID или IMDB (приоритет)
// 2. Если ID нет - поиск по названию + год
func (c *Client) SearchPagesByContentWithFilter(title, originalTitle, kpid, imdbID, extraFilter string, limit int64) (*SearchResult, error) {
	return c.SearchPagesByContentWithFilterAndYear(title, originalTitle, kpid, imdbID, 0, extraFilter, limit)
}

// SearchPagesByContentWithFilterAndYear ищет с учётом года
// Приоритеты: 1) ID точный матч, 2) название+год, 3) только название
func (c *Client) SearchPagesByContentWithFilterAndYear(title, originalTitle, kpid, imdbID string, year int, extraFilter string, limit int64) (*SearchResult, error) {
	// Приоритет 1: точный матч по ID
	if kpid != "" || imdbID != "" {
		result, err := c.searchByIDs(kpid, imdbID, extraFilter, limit)
		if err != nil {
			return nil, err
		}
		if result.TotalHits > 0 {
			return result, nil
		}
	}

	// Приоритет 2: название + год
	if year > 0 {
		yearFilter := "year = " + strconv.Itoa(year)
		if extraFilter != "" {
			yearFilter = yearFilter + " AND " + extraFilter
		}

		if title != "" {
			result, err := c.searchExactPhrase(title, yearFilter, limit)
			if err != nil {
				return nil, err
			}
			if result.TotalHits > 0 {
				return result, nil
			}
		}

		if originalTitle != "" {
			result, err := c.searchExactPhrase(originalTitle, yearFilter, limit)
			if err != nil {
				return nil, err
			}
			if result.TotalHits > 0 {
				return result, nil
			}
		}
	}

	// Приоритет 3: только название (exact phrase, без года)
	if title != "" {
		result, err := c.searchExactPhrase(title, extraFilter, limit)
		if err != nil {
			return nil, err
		}
		if result.TotalHits > 0 {
			return result, nil
		}
	}

	if originalTitle != "" {
		result, err := c.searchExactPhrase(originalTitle, extraFilter, limit)
		if err != nil {
			return nil, err
		}
		if result.TotalHits > 0 {
			return result, nil
		}
	}

	return &SearchResult{}, nil
}

func (c *Client) searchByIDs(kinopoiskID, imdbID, extraFilter string, limit int64) (*SearchResult, error) {
	var filters []string
	if kinopoiskID != "" {
		filters = append(filters, `kinopoisk_id = "`+kinopoiskID+`"`)
	}
	if imdbID != "" {
		filters = append(filters, `imdb_id = "`+imdbID+`"`)
	}

	filterStr := strings.Join(filters, " OR ")
	if extraFilter != "" {
		filterStr = "(" + filterStr + ") AND " + extraFilter
	}

	return c.SearchPages("", filterStr, limit)
}

func (c *Client) searchExactPhrase(phrase, filter string, limit int64) (*SearchResult, error) {
	query := `"` + phrase + `"`
	return c.SearchPages(query, filter, limit)
}

// CountPagesByContent считает страницы, соответствующие контенту
func (c *Client) CountPagesByContent(title, originalTitle, kpid, imdbID string) (int64, error) {
	result, err := c.SearchPagesByContent(title, originalTitle, kpid, imdbID, 1)
	if err != nil {
		return 0, err
	}
	return result.TotalHits, nil
}

// CountUniqueSitesByContent считает уникальные сайты с контентом
func (c *Client) CountUniqueSitesByContent(title, originalTitle, kpid, imdbID string) (int64, error) {
	result, err := c.SearchPagesByContent(title, originalTitle, kpid, imdbID, 1000)
	if err != nil {
		return 0, err
	}

	sites := make(map[string]bool)
	for _, hit := range result.Hits {
		sites[hit.SiteID] = true
	}
	return int64(len(sites)), nil
}

// DeletePage удаляет страницу из индекса
func (c *Client) DeletePage(id string) error {
	_, err := c.client.Index(PagesIndex).DeleteDocument(id)
	return err
}

// DeleteAllDocuments удаляет все документы из индекса pages
func (c *Client) DeleteAllDocuments() error {
	_, err := c.client.Index(PagesIndex).DeleteAllDocuments()
	return err
}

// DeleteBySiteID удаляет все страницы сайта из индекса
func (c *Client) DeleteBySiteID(siteID string) error {
	_, err := c.client.Index(PagesIndex).DeleteDocumentsByFilter("site_id = \"" + siteID + "\"")
	return err
}

// docToMap конвертирует PageDocument в map
func docToMap(doc *PageDocument) map[string]interface{} {
	m := map[string]interface{}{
		"id":          doc.ID,
		"site_id":     doc.SiteID,
		"domain":      doc.Domain,
		"url":         doc.URL,
		"title":       doc.Title,
		"description": doc.Description,
		"main_text":   doc.MainText,
		"indexed_at":  doc.IndexedAt,
	}
	if doc.Year > 0 {
		m["year"] = doc.Year
	}
	if doc.KinopoiskID != "" {
		m["kinopoisk_id"] = doc.KinopoiskID
	}
	if doc.IMDBID != "" {
		m["imdb_id"] = doc.IMDBID
	}
	if doc.MALID != "" {
		m["mal_id"] = doc.MALID
	}
	if doc.ShikimoriID != "" {
		m["shikimori_id"] = doc.ShikimoriID
	}
	if doc.MyDramaListID != "" {
		m["mydramalist_id"] = doc.MyDramaListID
	}
	if doc.LinksText != "" {
		m["links_text"] = doc.LinksText
	}
	if len(doc.PlayerURLs) > 0 {
		m["player_urls"] = doc.PlayerURLs
	}
	return m
}

// hitToPageDocument конвертирует hit в PageDocument
func hitToPageDocument(hit interface{}) PageDocument {
	var doc PageDocument

	// Сначала сериализуем в JSON, потом десериализуем
	b, err := json.Marshal(hit)
	if err != nil {
		return doc
	}
	json.Unmarshal(b, &doc)
	return doc
}
