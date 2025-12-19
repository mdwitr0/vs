package violations

import (
	"testing"

	"github.com/video-analitics/backend/pkg/meili"
)

func TestFilterHitsByPhrase(t *testing.T) {
	tests := []struct {
		name     string
		hits     []meili.PageDocument
		phrase   string
		expected int
	}{
		{
			name:     "empty hits",
			hits:     []meili.PageDocument{},
			phrase:   "Годы",
			expected: 0,
		},
		{
			name: "exact match in title - case insensitive",
			hits: []meili.PageDocument{
				{ID: "1", Title: "Лучшие годы", Description: ""},
				{ID: "2", Title: "Лихие", Description: ""},
			},
			phrase:   "Годы",
			expected: 1,
		},
		{
			name: "description is ignored - only title matters",
			hits: []meili.PageDocument{
				{ID: "1", Title: "Фильм", Description: "Про годы жизни"},
				{ID: "2", Title: "Другой фильм", Description: "Без совпадений"},
			},
			phrase:   "Годы",
			expected: 0, // description игнорируется, в title нет "Годы"
		},
		{
			name: "no false positives - completely different titles",
			hits: []meili.PageDocument{
				{ID: "1", Title: "Лихие (сериал)", Description: ""},
				{ID: "2", Title: "Гедда", Description: ""},
				{ID: "3", Title: "Пит и его дракон", Description: ""},
				{ID: "4", Title: "Круэлла", Description: ""},
				{ID: "5", Title: "Железный кулак", Description: ""},
				{ID: "6", Title: "Монстры", Description: ""},
			},
			phrase:   "Годы",
			expected: 0,
		},
		{
			name: "real data - mixed valid and invalid",
			hits: []meili.PageDocument{
				{ID: "1", Title: "Лучшие годы", Description: ""},
				{ID: "2", Title: "Утраченные годы", Description: ""},
				{ID: "3", Title: "Элвис. Ранние Годы", Description: ""},
				{ID: "4", Title: "Лихие (сериал, 1-2 сезон) 1-6,7,8 серия", Description: ""},
				{ID: "5", Title: "Гедда", Description: ""},
				{ID: "6", Title: "Безумцы (сериал 2007 1-7 сезон)", Description: ""},
			},
			phrase:   "Годы",
			expected: 3, // Лучшие годы, Утраченные годы, Элвис. Ранние Годы
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterHitsByPhrase(tt.hits, tt.phrase)
			if len(result) != tt.expected {
				t.Errorf("filterHitsByPhrase() returned %d hits, want %d", len(result), tt.expected)
				for _, h := range result {
					t.Logf("  - %s", h.Title)
				}
			}
		})
	}
}

func TestContainsTitleWithoutStopWords(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		title    string
		expected bool
	}{
		{
			name:     "exact match",
			text:     "Нарко",
			title:    "Нарко",
			expected: true,
		},
		{
			name:     "title in text with SEO words",
			text:     "Нарко 3 сезон смотреть онлайн бесплатно!",
			title:    "Нарко",
			expected: true,
		},
		{
			name:     "case insensitive",
			text:     "НАРКО 3 сезон смотреть онлайн",
			title:    "нарко",
			expected: true,
		},
		{
			name:     "multi-word title",
			text:     "Во все тяжкие 1 сезон смотреть онлайн бесплатно",
			title:    "Во все тяжкие",
			expected: true,
		},
		{
			name:     "multi-word title scattered in text",
			text:     "Смотреть все серии тяжкие сезоны во",
			title:    "Во все тяжкие",
			expected: true,
		},
		{
			name:     "title not in text",
			text:     "Игра престолов смотреть онлайн",
			title:    "Нарко",
			expected: false,
		},
		{
			name:     "partial match - should fail",
			text:     "Нар 3 сезон смотреть онлайн",
			title:    "Нарко",
			expected: false,
		},
		{
			name:     "empty text",
			text:     "",
			title:    "Нарко",
			expected: false,
		},
		{
			name:     "empty title",
			text:     "Нарко 3 сезон",
			title:    "",
			expected: false,
		},
		{
			name:     "english title with SEO",
			text:     "Narcos Season 3 Watch Online Free HD 1080",
			title:    "Narcos",
			expected: true,
		},
		{
			name:     "original title in description",
			text:     "Смотрите сериал Narcos в хорошем качестве",
			title:    "Narcos",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsTitleWithoutStopWords(tt.text, tt.title)
			if result != tt.expected {
				t.Errorf("containsTitleWithoutStopWords(%q, %q) = %v, want %v",
					tt.text, tt.title, result, tt.expected)
			}
		})
	}
}

func TestExtractMeaningfulWords(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name:     "simple title",
			text:     "нарко",
			expected: []string{"нарко"},
		},
		{
			name:     "title with stop words",
			text:     "нарко смотреть онлайн бесплатно",
			expected: []string{"нарко"},
		},
		{
			name:     "multi-word title",
			text:     "во все тяжкие",
			expected: []string{"во", "все", "тяжкие"},
		},
		{
			name:     "title with numbers",
			text:     "нарко 3 сезон 2015",
			expected: []string{"нарко", "2015"},
		},
		{
			name:     "english with stop words",
			text:     "narcos watch free online hd",
			expected: []string{"narcos"},
		},
		{
			name:     "only stop words",
			text:     "смотреть онлайн бесплатно hd",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMeaningfulWords(tt.text)
			if !stringSlicesEqual(result, tt.expected) {
				t.Errorf("extractMeaningfulWords(%q) = %v, want %v",
					tt.text, result, tt.expected)
			}
		})
	}
}

func TestContainsYear(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		year     string
		expected bool
	}{
		{
			name:     "year in parentheses",
			text:     "Нарко (2015) смотреть онлайн",
			year:     "2015",
			expected: true,
		},
		{
			name:     "year without parentheses",
			text:     "Нарко 2015 смотреть онлайн",
			year:     "2015",
			expected: true,
		},
		{
			name:     "year at end",
			text:     "Нарко смотреть онлайн 2015",
			year:     "2015",
			expected: true,
		},
		{
			name:     "wrong year",
			text:     "Нарко (2016) смотреть онлайн",
			year:     "2015",
			expected: false,
		},
		{
			name:     "no year in text",
			text:     "Нарко смотреть онлайн",
			year:     "2015",
			expected: false,
		},
		{
			name:     "year in resolution should not match",
			text:     "Нарко смотреть онлайн 1080",
			year:     "1080",
			expected: false,
		},
		{
			name:     "multiple years in text",
			text:     "Нарко (2015-2017) все сезоны",
			year:     "2015",
			expected: true,
		},
		{
			name:     "empty text",
			text:     "",
			year:     "2015",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsYear(tt.text, tt.year)
			if result != tt.expected {
				t.Errorf("containsYear(%q, %q) = %v, want %v",
					tt.text, tt.year, result, tt.expected)
			}
		})
	}
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
