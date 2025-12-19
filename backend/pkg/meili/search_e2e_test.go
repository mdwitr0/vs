//go:build e2e
// +build e2e

package meili_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/video-analitics/backend/pkg/meili"
)

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

// TestSearch_ShortTitle_Gody_RealData тестирует поиск контента "Годы" (KPID: 1208544)
// Реальная проблема: поиск по "Годы" находит "Лихие", "Гедда", "Пит и его дракон" и другие
// несвязанные фильмы. Это ложные срабатывания, которые нужно исключить.
func TestSearch_ShortTitle_Gody_RealData(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, cleanup := setupMeilisearch(t, ctx)
	defer cleanup()

	// Реальные данные из продакшена - страницы которые ДОЛЖНЫ матчиться
	validPages := []meili.PageDocument{
		{
			ID:        "gody_valid_001",
			SiteID:    "site001",
			Domain:    "lordfilm-subway.ru",
			URL:       "https://lordfilm-subway.ru/luchshie-gody-2020/",
			Title:     "Лучшие годы",
			IndexedAt: time.Now().Format(time.RFC3339),
		},
		{
			ID:        "gody_valid_002",
			SiteID:    "site002",
			Domain:    "lordfilm-cherry.ru",
			URL:       "https://lordfilm-cherry.ru/luchshie-gody-nashey-zhizni-1946/",
			Title:     "Лучшие годы нашей жизни",
			IndexedAt: time.Now().Format(time.RFC3339),
		},
		{
			ID:        "gody_valid_003",
			SiteID:    "site002",
			Domain:    "lordfilm-cherry.ru",
			URL:       "https://lordfilm-cherry.ru/elvis-rannie-gody-2005/",
			Title:     "Элвис. Ранние Годы",
			IndexedAt: time.Now().Format(time.RFC3339),
		},
		{
			ID:        "gody_valid_004",
			SiteID:    "site003",
			Domain:    "turkruhd.net",
			URL:       "https://tv4.turkruhd.net/932-utrachennye-gody-2006-na-russkom-turkruhd.html",
			Title:     "Утраченные годы",
			IndexedAt: time.Now().Format(time.RFC3339),
		},
		{
			ID:        "gody_valid_005",
			SiteID:    "site001",
			Domain:    "lordfilm-subway.ru",
			URL:       "https://lordfilm-subway.ru/luchshie-gody-zhizni-2019/",
			Title:     "Лучшие годы жизни",
			IndexedAt: time.Now().Format(time.RFC3339),
		},
	}

	// Реальные данные из продакшена - страницы которые НЕ ДОЛЖНЫ матчиться (false positives)
	invalidPages := []meili.PageDocument{
		{
			ID:          "invalid_001",
			SiteID:      "site004",
			Domain:      "mc.lordfilmi.lol",
			URL:         "https://mc.lordfilmi.lol/95-lihie.html",
			Title:       "Лихие (сериал, 1-2 сезон) 1-6,7,8 серия",
			Description: "Криминальный сериал Лихие",
			IndexedAt:   time.Now().Format(time.RFC3339),
		},
		{
			ID:          "invalid_002",
			SiteID:      "site005",
			Domain:      "lardserials.ru",
			URL:         "https://lardserials.ru/filmy/2566-pit-i-ego-drakon-film-2016-smotret-onlayn-besplatno.html",
			Title:       "Пит и его дракон (фильм 2016) смотреть онлайн",
			Description: "Фэнтези про мальчика и дракона",
			IndexedAt:   time.Now().Format(time.RFC3339),
		},
		{
			ID:          "invalid_003",
			SiteID:      "site006",
			Domain:      "lordfilmgori.life",
			URL:         "https://lordfilmgori.life/1516-gedda-cyn.html",
			Title:       "Гедда",
			Description: "Драма",
			IndexedAt:   time.Now().Format(time.RFC3339),
		},
		{
			ID:          "invalid_004",
			SiteID:      "site004",
			Domain:      "mc.lordfilmi.lol",
			URL:         "https://mc.lordfilmi.lol/123-kruella.html",
			Title:       "Круэлла (фильм, 2021) смотреть онлайн",
			Description: "Фильм про злодейку",
			IndexedAt:   time.Now().Format(time.RFC3339),
		},
		{
			ID:          "invalid_005",
			SiteID:      "site005",
			Domain:      "lardserials.ru",
			URL:         "https://lardserials.ru/filmy/serialy/1773-bezumcy-serial-2007-smotret-onlayn-besplatno.html",
			Title:       "Безумцы (сериал 2007 1-7 сезон) смотреть онлайн",
			Description: "Сериал про рекламное агентство",
			IndexedAt:   time.Now().Format(time.RFC3339),
		},
		{
			ID:          "invalid_006",
			SiteID:      "site006",
			Domain:      "lordfilmgori.life",
			URL:         "https://lordfilmgori.life/971-revoljucija-iisusa-1tz.html",
			Title:       "Революция Иисуса",
			Description: "Религиозная драма",
			IndexedAt:   time.Now().Format(time.RFC3339),
		},
		{
			ID:          "invalid_007",
			SiteID:      "site004",
			Domain:      "mc.lordfilmi.lol",
			URL:         "https://mc.lordfilmi.lol/195-monstry.html",
			Title:       "Монстры (сериал, 1-2,3 сезон) 1-6,7,8 серия",
			Description: "Сериал про монстров",
			IndexedAt:   time.Now().Format(time.RFC3339),
		},
	}

	// Индексируем все страницы
	allPages := append(validPages, invalidPages...)
	err := client.IndexPages(allPages)
	require.NoError(t, err)

	// Ждём индексации
	time.Sleep(1 * time.Second)

	t.Run("ExactPhraseSearch_Gody_ShouldOnlyMatchPagesWithGody", func(t *testing.T) {
		// Поиск по точной фразе "Годы"
		result, err := client.SearchPages(`"Годы"`, "", 100)
		require.NoError(t, err)

		t.Logf("Found %d hits for exact phrase 'Годы'", len(result.Hits))
		for _, hit := range result.Hits {
			t.Logf("  - %s (%s)", hit.Title, hit.Domain)
		}

		// Все результаты ДОЛЖНЫ содержать "годы" в title или description
		for _, hit := range result.Hits {
			titleLower := strings.ToLower(hit.Title)
			descLower := strings.ToLower(hit.Description)
			containsGody := strings.Contains(titleLower, "годы") || strings.Contains(descLower, "годы")
			assert.True(t, containsGody,
				"hit '%s' should contain 'годы' but doesn't", hit.Title)
		}

		// НЕ ДОЛЖНЫ найти эти страницы
		invalidTitles := []string{"Лихие", "Гедда", "Пит и его дракон", "Круэлла", "Безумцы", "Революция Иисуса", "Монстры"}
		for _, hit := range result.Hits {
			for _, invalid := range invalidTitles {
				assert.NotContains(t, hit.Title, invalid,
					"should not match '%s' when searching for 'Годы'", invalid)
			}
		}
	})

	t.Run("SearchPagesByContent_Gody_NoFalsePositives", func(t *testing.T) {
		// Поиск как в реальном коде violations matcher
		result, err := client.SearchPagesByContentWithFilterAndYear("Годы", "", "1208544", "", 0, "", 100)
		require.NoError(t, err)

		t.Logf("SearchPagesByContent found %d hits", len(result.Hits))
		for _, hit := range result.Hits {
			t.Logf("  - %s (%s)", hit.Title, hit.Domain)
		}

		// Если нашли что-то - проверяем что это правильные результаты
		for _, hit := range result.Hits {
			titleLower := strings.ToLower(hit.Title)
			descLower := strings.ToLower(hit.Description)
			containsGody := strings.Contains(titleLower, "годы") || strings.Contains(descLower, "годы")
			hasKPID := hit.KPID == "1208544"
			assert.True(t, containsGody || hasKPID,
				"hit '%s' should contain 'годы' or have KPID 1208544", hit.Title)
		}
	})
}

// TestSearch_MatchingStrategy тестирует что matchingStrategy=all работает корректно
func TestSearch_MatchingStrategy(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	client, cleanup := setupMeilisearch(t, ctx)
	defer cleanup()

	pages := []meili.PageDocument{
		{
			ID:        "exact_match",
			SiteID:    "site001",
			Domain:    "test.com",
			URL:       "https://test.com/gody",
			Title:     "Годы смотреть онлайн",
			IndexedAt: time.Now().Format(time.RFC3339),
		},
		{
			ID:        "partial_match",
			SiteID:    "site001",
			Domain:    "test.com",
			URL:       "https://test.com/other",
			Title:     "Другой фильм онлайн",
			IndexedAt: time.Now().Format(time.RFC3339),
		},
	}

	err := client.IndexPages(pages)
	require.NoError(t, err)
	time.Sleep(500 * time.Millisecond)

	t.Run("ExactPhraseWithQuotes", func(t *testing.T) {
		result, err := client.SearchPages(`"Годы"`, "", 100)
		require.NoError(t, err)

		t.Logf("Found %d hits", len(result.Hits))
		for _, hit := range result.Hits {
			t.Logf("  - %s", hit.Title)
		}

		// Должен найти только страницу с "Годы"
		assert.LessOrEqual(t, len(result.Hits), 1, "should find at most 1 hit for exact phrase")
		if len(result.Hits) > 0 {
			assert.Contains(t, result.Hits[0].Title, "Годы")
		}
	})
}