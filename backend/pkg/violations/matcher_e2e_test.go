//go:build e2e
// +build e2e

package violations_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/video-analitics/backend/pkg/meili"
	"github.com/video-analitics/backend/pkg/violations"
)

type testPage struct {
	ID          string
	SiteID      string
	Domain      string
	URL         string
	Title       string
	Description string
	MainText    string
	Year        int
	KPID        string
	IMDBID      string
}

var narkoTVPages = []testPage{
	{
		ID:          "6933991de6243812e17d4b81",
		SiteID:      "6933986caba43e828338ba89",
		Domain:      "narko-tv.com",
		URL:         "https://narko-tv.com/season-3/",
		Title:       "Нарко 3 сезон смотреть онлайн бесплатно!",
		Description: "Нарко 3 сезон смотреть онлайн в русском переводе LostFilm, Дубляж или Кубик в Кубе в хорошем 1080 HD качестве!",
		MainText:    "Нарко 3 сезон смотреть онлайн 2017 | 2 772 просмотров... Великолепный третий сезон популярного сериала «Нарко» вышел на экраны 1 сентября 2017 года.",
	},
	{
		ID:          "6933991de6243812e17d4b82",
		SiteID:      "6933986caba43e828338ba89",
		Domain:      "narko-tv.com",
		URL:         "https://narko-tv.com/season-1/",
		Title:       "Нарко 1 сезон смотреть онлайн бесплатно!",
		Description: "Нарко 1 сезон смотреть онлайн в русском переводе LostFilm в хорошем HD качестве!",
		MainText:    "Нарко 1 сезон 2015 года. Сериал о наркобароне Пабло Эскобаре.",
	},
	{
		ID:          "6933991de6243812e17d4b83",
		SiteID:      "6933986caba43e828338ba89",
		Domain:      "narko-tv.com",
		URL:         "https://narko-tv.com/season-2/",
		Title:       "Нарко 2 сезон смотреть онлайн бесплатно!",
		Description: "Нарко 2 сезон смотреть онлайн бесплатно 2016",
	},
}

var lordfilmPages = []testPage{
	{
		ID:          "lordfilm001",
		SiteID:      "lordfilm_site_001",
		Domain:      "lordfilm.com",
		URL:         "https://lordfilm.com/narcos/",
		Title:       "Нарко (1-3 сезон) смотреть онлайн",
		Description: "Сериал Нарко (Narcos) 2015-2017 все сезоны смотреть онлайн",
		Year:        2015,
		KPID:        "789123",
	},
	{
		ID:          "lordfilm002",
		SiteID:      "lordfilm_site_001",
		Domain:      "lordfilm.com",
		URL:         "https://lordfilm.com/breaking-bad/",
		Title:       "Во все тяжкие (1-5 сезон) смотреть онлайн",
		Description: "Breaking Bad все сезоны",
		Year:        2008,
		KPID:        "404900",
		IMDBID:      "tt0903747",
	},
}

func setupMeilisearch(t *testing.T, ctx context.Context) (*meili.Client, func()) {
	t.Helper()

	req := testcontainers.ContainerRequest{
		Image:        "getmeili/meilisearch:v1.12",
		ExposedPorts: []string{"7700/tcp"},
		Env: map[string]string{
			"MEILI_MASTER_KEY": "testMasterKey",
			"MEILI_ENV":        "development",
		},
		WaitingFor: wait.ForHTTP("/health").WithPort("7700/tcp").WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err, "failed to start meilisearch container")

	host, err := container.Host(ctx)
	require.NoError(t, err)

	port, err := container.MappedPort(ctx, "7700")
	require.NoError(t, err)

	url := fmt.Sprintf("http://%s:%s", host, port.Port())

	client, err := meili.New(url, "testMasterKey")
	require.NoError(t, err, "failed to create meilisearch client")

	cleanup := func() {
		if err := container.Terminate(ctx); err != nil {
			t.Logf("failed to terminate container: %v", err)
		}
	}

	return client, cleanup
}

func indexPages(t *testing.T, client *meili.Client, pages []testPage) {
	t.Helper()

	for _, p := range pages {
		doc := &meili.PageDocument{
			ID:          p.ID,
			SiteID:      p.SiteID,
			Domain:      p.Domain,
			URL:         p.URL,
			Title:       p.Title,
			Description: p.Description,
			MainText:    p.MainText,
			Year:        p.Year,
			KPID:        p.KPID,
			IMDBID:      p.IMDBID,
			IndexedAt:   time.Now().Format(time.RFC3339),
		}
		err := client.IndexPage(doc)
		require.NoError(t, err, "failed to index page: %s", p.Title)
	}

	time.Sleep(500 * time.Millisecond)
}

func TestMatcher_NarkoTV_E2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, cleanup := setupMeilisearch(t, ctx)
	defer cleanup()

	allPages := append(narkoTVPages, lordfilmPages...)
	indexPages(t, client, allPages)

	matcher := violations.NewMatcher(client)

	t.Run("FindByExactTitle", func(t *testing.T) {
		content := violations.ContentInfo{
			ID:            "narcos_content",
			Title:         "Нарко",
			OriginalTitle: "Narcos",
			Year:          2015,
		}

		matches, matchType, err := matcher.FindMatches(ctx, content)
		require.NoError(t, err)

		t.Logf("MatchType: %s", matchType)
		t.Logf("Found %d matches", len(matches))
		for _, m := range matches {
			t.Logf("  - %s (%s) [%s]", m.Title, m.Domain, m.MatchType)
		}

		assert.NotEmpty(t, matches, "should find matches for 'Нарко'")
		assert.Contains(t, []violations.MatchType{
			violations.MatchByTitle,
			violations.MatchByTitleYear,
			violations.MatchByKPID,
		}, matchType)
	})

	t.Run("FindAllMatches_CollectsAllSites", func(t *testing.T) {
		content := violations.ContentInfo{
			ID:            "narcos_all",
			Title:         "Нарко",
			OriginalTitle: "Narcos",
			Year:          2015,
		}

		matches, err := matcher.FindAllMatches(ctx, content)
		require.NoError(t, err)

		t.Logf("Found %d total matches from all stages", len(matches))
		for _, m := range matches {
			t.Logf("  - %s (%s) [%s]", m.Title, m.Domain, m.MatchType)
		}

		assert.NotEmpty(t, matches, "should find matches")

		// Check both sites are found
		domains := make(map[string]bool)
		for _, m := range matches {
			domains[m.Domain] = true
		}

		assert.True(t, domains["narko-tv.com"], "narko-tv.com should be in matches")
		assert.True(t, domains["lordfilm.com"], "lordfilm.com should be in matches")
	})

	t.Run("FindByKPID", func(t *testing.T) {
		content := violations.ContentInfo{
			ID:    "narcos_kpid",
			Title: "Нарко",
			KPID:  "789123",
		}

		matches, matchType, err := matcher.FindMatches(ctx, content)
		require.NoError(t, err)

		t.Logf("MatchType: %s", matchType)
		t.Logf("Found %d matches", len(matches))

		assert.Equal(t, violations.MatchByKPID, matchType)
		assert.Len(t, matches, 1)
		assert.Equal(t, "lordfilm.com", matches[0].Domain)
	})

	t.Run("FindByIMDB", func(t *testing.T) {
		content := violations.ContentInfo{
			ID:     "breaking_bad",
			Title:  "Во все тяжкие",
			IMDBID: "tt0903747",
		}

		matches, matchType, err := matcher.FindMatches(ctx, content)
		require.NoError(t, err)

		t.Logf("MatchType: %s", matchType)

		assert.Equal(t, violations.MatchByIMDB, matchType)
		assert.Len(t, matches, 1)
	})

	t.Run("FindByTitleYear_WithYearField", func(t *testing.T) {
		content := violations.ContentInfo{
			ID:    "narcos_2015",
			Title: "Нарко",
			Year:  2015,
		}

		matches, matchType, err := matcher.FindMatches(ctx, content)
		require.NoError(t, err)

		t.Logf("MatchType: %s", matchType)
		t.Logf("Found %d matches", len(matches))

		// lordfilm has year=2015 in structured field
		if matchType == violations.MatchByTitleYear {
			for _, m := range matches {
				assert.Equal(t, "lordfilm.com", m.Domain, "title_year should match lordfilm with year field")
			}
		}
	})

	t.Run("FindByTitleFuzzyYear_YearInText", func(t *testing.T) {
		content := violations.ContentInfo{
			ID:            "narcos_fuzzy",
			Title:         "Narcos",
			OriginalTitle: "Нарко",
			Year:          2017,
		}

		matches, matchType, err := matcher.FindMatches(ctx, content)
		require.NoError(t, err)

		t.Logf("MatchType: %s", matchType)
		t.Logf("Found %d matches", len(matches))
		for _, m := range matches {
			t.Logf("  - %s (%s)", m.Title, m.Domain)
		}

		// Year 2017 is mentioned in narko-tv.com season 3 page main_text
		// This should trigger fuzzy year matching
		if matchType == violations.MatchByTitleFuzzyYear {
			var foundSeason3 bool
			for _, m := range matches {
				if m.URL == "https://narko-tv.com/season-3/" {
					foundSeason3 = true
				}
			}
			assert.True(t, foundSeason3, "should find season 3 page with year 2017 in text")
		}
	})

	t.Run("FindByTitle_ExactPhrase", func(t *testing.T) {
		content := violations.ContentInfo{
			ID:    "narcos_title",
			Title: "Нарко",
		}

		matches, matchType, err := matcher.FindMatches(ctx, content)
		require.NoError(t, err)

		t.Logf("MatchType: %s", matchType)
		t.Logf("Found %d matches", len(matches))

		assert.NotEmpty(t, matches)
		assert.Equal(t, violations.MatchByTitle, matchType)

		var domains []string
		for _, m := range matches {
			domains = append(domains, m.Domain)
		}
		assert.Contains(t, domains, "narko-tv.com", "should match narko-tv.com with exact phrase 'Нарко'")
	})

	t.Run("NoMatchForUnknownContent", func(t *testing.T) {
		content := violations.ContentInfo{
			ID:    "unknown",
			Title: "Несуществующий фильм 12345",
		}

		matches, matchType, err := matcher.FindMatches(ctx, content)
		require.NoError(t, err)

		assert.Empty(t, matches)
		assert.Empty(t, matchType)
	})

	t.Run("NoFalsePositives_UnrelatedContent", func(t *testing.T) {
		// Этот контент НЕ должен матчить "Нарко" страницы
		content := violations.ContentInfo{
			ID:            "breaking_bad_test",
			Title:         "Во все тяжкие",
			OriginalTitle: "Breaking Bad",
			Year:          2008,
		}

		matches, err := matcher.FindAllMatches(ctx, content)
		require.NoError(t, err)

		t.Logf("Found %d matches", len(matches))
		for _, m := range matches {
			t.Logf("  - %s (%s) [%s]", m.Title, m.Domain, m.MatchType)
		}

		// Должны найти только "Во все тяжкие", а не "Нарко" страницы
		for _, m := range matches {
			assert.NotContains(t, m.Title, "Нарко", "should not match unrelated content")
		}
	})

	t.Run("FindMatchesForSpecificSite", func(t *testing.T) {
		content := violations.ContentInfo{
			ID:    "narcos_site",
			Title: "Нарко",
		}

		matches, matchType, err := matcher.FindMatchesForSite(ctx, content, "6933986caba43e828338ba89")
		require.NoError(t, err)

		t.Logf("MatchType: %s", matchType)
		t.Logf("Found %d matches for site", len(matches))

		for _, m := range matches {
			assert.Equal(t, "narko-tv.com", m.Domain, "all matches should be from narko-tv.com")
		}
	})
}

func TestMatcher_StopWords_E2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, cleanup := setupMeilisearch(t, ctx)
	defer cleanup()

	pages := []testPage{
		{
			ID:          "stopwords001",
			SiteID:      "site001",
			Domain:      "test.com",
			URL:         "https://test.com/movie1",
			Title:       "Аватар смотреть онлайн бесплатно в хорошем качестве HD 1080",
			Description: "Фильм Аватар 2009 смотреть онлайн",
		},
		{
			ID:          "stopwords002",
			SiteID:      "site001",
			Domain:      "test.com",
			URL:         "https://test.com/movie2",
			Title:       "Аватар: Путь воды (2022) смотреть онлайн бесплатно",
			Description: "Avatar: The Way of Water",
			Year:        2022,
		},
	}

	indexPages(t, client, pages)

	matcher := violations.NewMatcher(client)

	t.Run("MatchesDespiteStopWords", func(t *testing.T) {
		content := violations.ContentInfo{
			ID:            "avatar",
			Title:         "Аватар",
			OriginalTitle: "Avatar",
			Year:          2009,
		}

		matches, matchType, err := matcher.FindMatches(ctx, content)
		require.NoError(t, err)

		t.Logf("MatchType: %s", matchType)
		t.Logf("Found %d matches", len(matches))

		assert.NotEmpty(t, matches, "should match 'Аватар' despite SEO words")
	})

	t.Run("MatchesWithYear2022", func(t *testing.T) {
		content := violations.ContentInfo{
			ID:    "avatar2",
			Title: "Аватар: Путь воды",
			Year:  2022,
		}

		matches, matchType, err := matcher.FindMatches(ctx, content)
		require.NoError(t, err)

		t.Logf("MatchType: %s", matchType)

		assert.NotEmpty(t, matches)
		if matchType == violations.MatchByTitleYear {
			assert.Len(t, matches, 1)
		}
	})
}

func TestMatcher_YearInText_E2E(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, cleanup := setupMeilisearch(t, ctx)
	defer cleanup()

	pages := []testPage{
		{
			ID:          "yeartext001",
			SiteID:      "site001",
			Domain:      "pirate.com",
			URL:         "https://pirate.com/movie",
			Title:       "Дюна (2021) смотреть онлайн",
			Description: "Фантастический фильм Дюна 2021 года в хорошем качестве",
		},
		{
			ID:          "yeartext002",
			SiteID:      "site001",
			Domain:      "pirate.com",
			URL:         "https://pirate.com/movie2",
			Title:       "Дюна: Часть вторая смотреть онлайн",
			Description: "Продолжение фильма Дюна. Премьера 2024 года.",
		},
	}

	indexPages(t, client, pages)

	matcher := violations.NewMatcher(client)

	t.Run("FindByYearInTitle", func(t *testing.T) {
		content := violations.ContentInfo{
			ID:    "dune1",
			Title: "Дюна",
			Year:  2021,
		}

		matches, matchType, err := matcher.FindMatches(ctx, content)
		require.NoError(t, err)

		t.Logf("MatchType: %s", matchType)
		t.Logf("Found %d matches", len(matches))

		assert.NotEmpty(t, matches)
		// Should find the first page with 2021 in title/description
	})

	t.Run("FindByYearInDescription", func(t *testing.T) {
		content := violations.ContentInfo{
			ID:            "dune2",
			Title:         "Дюна: Часть вторая",
			OriginalTitle: "Dune: Part Two",
			Year:          2024,
		}

		matches, matchType, err := matcher.FindMatches(ctx, content)
		require.NoError(t, err)

		t.Logf("MatchType: %s", matchType)
		t.Logf("Found %d matches", len(matches))

		// Year 2024 is in description of second page
		for _, m := range matches {
			t.Logf("  - %s", m.Title)
		}
	})
}
