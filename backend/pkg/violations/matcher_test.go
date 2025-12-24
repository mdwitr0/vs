package violations

import (
	"strings"
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
			phrase:   "Властелин колец",
			expected: 0,
		},
		{
			name: "exact match in title - case insensitive",
			hits: []meili.PageDocument{
				{ID: "1", Title: "Властелин Колец: Братство кольца", Description: ""},
				{ID: "2", Title: "Другой фильм", Description: ""},
			},
			phrase:   "Властелин колец",
			expected: 1,
		},
		{
			name: "description is ignored - only title matters",
			hits: []meili.PageDocument{
				{ID: "1", Title: "Фильм", Description: "Про Властелин колец"},
				{ID: "2", Title: "Другой фильм", Description: "Без совпадений"},
			},
			phrase:   "Властелин колец",
			expected: 0, // description игнорируется
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
			phrase:   "Властелин колец",
			expected: 0,
		},
		{
			name: "real data - mixed valid and invalid",
			hits: []meili.PageDocument{
				{ID: "1", Title: "Властелин Колец: Братство Кольца", Description: ""},
				{ID: "2", Title: "Властелин Колец: Две крепости", Description: ""},
				{ID: "3", Title: "Властелин Колец: Возвращение короля", Description: ""},
				{ID: "4", Title: "Хоббит: Нежданное путешествие", Description: ""},
				{ID: "5", Title: "Гедда", Description: ""},
				{ID: "6", Title: "Безумцы (сериал 2007 1-7 сезон)", Description: ""},
			},
			phrase:   "Властелин колец",
			expected: 3,
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

func TestContainsIDInURL(t *testing.T) {
	tests := []struct {
		name      string
		linksText string
		id        string
		matchType MatchType
		expected  bool
	}{
		{
			name:      "MAL ID in valid URL",
			linksText: "https://myanimelist.net/anime/600 https://example.com",
			id:        "600",
			matchType: MatchByMAL,
			expected:  true,
		},
		{
			name:      "MAL ID - false positive in screen resolution",
			linksText: "https://counter.yadro.ru/hit?t58.5;r;s800*600*24;uhttps%3A//go.kinogo1.biz/comments.html",
			id:        "600",
			matchType: MatchByMAL,
			expected:  false,
		},
		{
			name:      "MAL ID in random URL number",
			linksText: "https://go.kinogo1.biz/17698-krasnaja-cherta-2010-kinogo.html",
			id:        "600",
			matchType: MatchByMAL,
			expected:  false,
		},
		{
			name:      "MAL ID valid - short ID 20",
			linksText: "https://myanimelist.net/anime/20 https://other.com",
			id:        "20",
			matchType: MatchByMAL,
			expected:  true,
		},
		{
			name:      "Shikimori ID in valid URL - with z prefix",
			linksText: "https://shikimori.one/animes/z600-legend-of-duo",
			id:        "600",
			matchType: MatchByShikimori,
			expected:  true,
		},
		{
			name:      "Shikimori ID in valid URL - without prefix",
			linksText: "https://shikimori.me/animes/600",
			id:        "600",
			matchType: MatchByShikimori,
			expected:  true,
		},
		{
			name:      "Shikimori ID - false positive",
			linksText: "https://counter.yadro.ru/hit?t58.5;r;s800*600*24",
			id:        "600",
			matchType: MatchByShikimori,
			expected:  false,
		},
		{
			name:      "MyDramaList ID in valid URL",
			linksText: "https://mydramalist.com/714269-some-drama",
			id:        "714269",
			matchType: MatchByMyDramaList,
			expected:  true,
		},
		{
			name:      "MyDramaList ID - false positive",
			linksText: "https://example.com/714269-something",
			id:        "714269",
			matchType: MatchByMyDramaList,
			expected:  false,
		},
		{
			name:      "Empty links text",
			linksText: "",
			id:        "600",
			matchType: MatchByMAL,
			expected:  false,
		},
		{
			name:      "Multiple MAL URLs - one matches",
			linksText: "https://myanimelist.net/anime/100 https://myanimelist.net/anime/600 https://myanimelist.net/anime/300",
			id:        "600",
			matchType: MatchByMAL,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var regex = malURLRegex
			switch tt.matchType {
			case MatchByShikimori:
				regex = shikimoriURLRegex
			case MatchByMyDramaList:
				regex = mdlURLRegex
			}

			result := containsIDInURL(tt.linksText, tt.id, regex)
			if result != tt.expected {
				t.Errorf("containsIDInURL() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestIsShortOrCommonTitle(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected bool
	}{
		{
			name:     "short title - 3 chars cyrillic",
			title:    "Год",
			expected: true,
		},
		{
			name:     "short title - 5 chars cyrillic",
			title:    "Время",
			expected: true,
		},
		{
			name:     "common word in list",
			title:    "Жизнь",
			expected: true,
		},
		{
			name:     "common word - english",
			title:    "Love",
			expected: true,
		},
		{
			name:     "normal title - 8 chars",
			title:    "Аватар",
			expected: false,
		},
		{
			name:     "long title",
			title:    "Властелин колец",
			expected: false,
		},
		{
			name:     "case insensitive - common word",
			title:    "ГОД",
			expected: true,
		},
		{
			name:     "6 char title - borderline",
			title:    "Кимера",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isShortOrCommonTitle(tt.title)
			if result != tt.expected {
				t.Errorf("isShortOrCommonTitle(%q) = %v, want %v", tt.title, result, tt.expected)
			}
		})
	}
}

func TestContainsWholeWord(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		word     string
		expected bool
	}{
		{
			name:     "word at start",
			text:     "год дракона смотреть",
			word:     "год",
			expected: true,
		},
		{
			name:     "word in middle",
			text:     "новый год 2024",
			word:     "год",
			expected: true,
		},
		{
			name:     "word at end",
			text:     "прекрасный год",
			word:     "год",
			expected: true,
		},
		{
			name:     "word is part of another word - should not match",
			text:     "выходной",
			word:     "год",
			expected: false,
		},
		{
			name:     "similar but different word",
			text:     "годы войны",
			word:     "год",
			expected: false,
		},
		{
			name:     "exact single word",
			text:     "год",
			word:     "год",
			expected: true,
		},
		{
			name:     "empty text",
			text:     "",
			word:     "год",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsWholeWord(tt.text, tt.word)
			if result != tt.expected {
				t.Errorf("containsWholeWord(%q, %q) = %v, want %v", tt.text, tt.word, result, tt.expected)
			}
		})
	}
}

func TestIsValidTitle(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected bool
	}{
		{"empty", "", false},
		{"dash", "-", false},
		{"double dash", "--", false},
		{"dots", "...", false},
		{"n/a", "N/A", false},
		{"tba", "TBA", false},
		{"unknown", "unknown", false},
		{"valid title", "Naruto", true},
		{"valid russian", "Наруто", true},
		{"valid with dash", "Spider-Man", true},
		{"valid with dots", "Dr. Strange", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidTitle(tt.title)
			if result != tt.expected {
				t.Errorf("isValidTitle(%q) = %v, want %v", tt.title, result, tt.expected)
			}
		})
	}
}

func TestFilterHitsByPhraseStrictMode(t *testing.T) {
	tests := []struct {
		name     string
		hits     []meili.PageDocument
		phrase   string
		expected int
	}{
		{
			name: "single-word title - must start with phrase and only stop words follow",
			hits: []meili.PageDocument{
				{ID: "1", Title: "Любовь (2015)", Description: ""},          // год - матчится
				{ID: "2", Title: "Любовь. Смерть. Роботы", Description: ""}, // "Смерть", "Роботы" не стоп-слова - НЕ матчится
				{ID: "3", Title: "Тор: Любовь и гром", Description: ""},     // НЕ в начале - не матчится
				{ID: "4", Title: "Первая любовь", Description: ""},          // НЕ в начале - не матчится
				{ID: "5", Title: "Походу любовь", Description: ""},          // НЕ в начале - не матчится
				{ID: "6", Title: "Любовь смотреть онлайн", Description: ""}, // стоп-слова - матчится
			},
			phrase:   "Любовь",
			expected: 2, // Любовь (2015), Любовь смотреть онлайн
		},
		{
			name: "single-word english title - only stop words after",
			hits: []meili.PageDocument{
				{ID: "1", Title: "Love (2015)", Description: ""},       // год - матчится
				{ID: "2", Title: "Love Story", Description: ""},        // "Story" не стоп-слово - НЕ матчится
				{ID: "3", Title: "Lovely Day", Description: ""},        // love != lovely - не матчится
				{ID: "4", Title: "I love you", Description: ""},        // НЕ в начале - не матчится
				{ID: "5", Title: "Crazy Stupid Love", Description: ""}, // НЕ в начале - не матчится
			},
			phrase:   "Love",
			expected: 1, // только Love (2015)
		},
		{
			name: "Монстр - only stop words allowed after",
			hits: []meili.PageDocument{
				{ID: "1", Title: "Монстр (2003)", Description: ""},          // год - матчится
				{ID: "2", Title: "Монстр: История Дамера", Description: ""}, // "История Дамера" не стоп-слова - НЕ матчится
				{ID: "3", Title: "Морской монстр", Description: ""},         // НЕ в начале - не матчится
				{ID: "4", Title: "Мой монстр", Description: ""},             // НЕ в начале - не матчится
				{ID: "5", Title: "Монстр в Париже", Description: ""},        // "Париже" не стоп-слово - НЕ матчится
				{ID: "6", Title: "Монстр смотреть онлайн", Description: ""}, // стоп-слова - матчится
			},
			phrase:   "Монстр",
			expected: 2, // Монстр (2003), Монстр смотреть онлайн
		},
		{
			name: "short multi-word phrase - strict mode",
			hits: []meili.PageDocument{
				{ID: "1", Title: "Ла Ла Ленд", Description: ""},            // "Ленд" не стоп-слово - НЕ матчится
				{ID: "2", Title: "Ла ла смотреть онлайн", Description: ""}, // стоп-слова - матчится
				{ID: "3", Title: "Бла ла ла бла", Description: ""},         // НЕ в начале
			},
			phrase:   "Ла ла",
			expected: 1, // только "Ла ла смотреть онлайн"
		},
		{
			name: "long title - normal mode - substring match",
			hits: []meili.PageDocument{
				{ID: "1", Title: "Властелин колец: Братство кольца", Description: ""},
				{ID: "2", Title: "Колец не видно", Description: ""},
			},
			phrase:   "Властелин колец",
			expected: 1,
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

func TestIsSingleWordTitle(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		expected bool
	}{
		{"single word cyrillic", "Любовь", true},
		{"single word english", "Love", true},
		{"two words", "Love Story", false},
		{"three words", "Во все тяжкие", false},
		{"single word with spaces", "  Любовь  ", true},
		{"empty", "", false},
		{"only spaces", "   ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSingleWordTitle(tt.title)
			if result != tt.expected {
				t.Errorf("isSingleWordTitle(%q) = %v, want %v", tt.title, result, tt.expected)
			}
		})
	}
}

func TestTitleStartsWithWord(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		word     string
		expected bool
	}{
		{"word at start", "любовь смотреть онлайн", "любовь", true},
		{"word in middle", "тор любовь и гром", "любовь", false},
		{"word at end", "первая любовь", "любовь", false},
		{"exact match", "любовь", "любовь", true},
		{"word not present", "время", "любовь", false},
		{"empty text", "", "любовь", false},
		{"word as prefix of first word", "любовьнежна", "любовь", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := titleStartsWithWord(tt.text, tt.word)
			if result != tt.expected {
				t.Errorf("titleStartsWithWord(%q, %q) = %v, want %v", tt.text, tt.word, result, tt.expected)
			}
		})
	}
}

func TestFilterHitsByPhraseShortMultiWord(t *testing.T) {
	tests := []struct {
		name     string
		hits     []meili.PageDocument
		phrase   string
		expected int
	}{
		{
			name: "short two-word phrase - Из ада - must start title with only stop words after",
			hits: []meili.PageDocument{
				{ID: "1", Title: "Из ада (2001)", Description: ""},                    // начинает + год - матчится
				{ID: "2", Title: "Из ада: Продолжение", Description: ""},              // "Продолжение" не стоп-слово - НЕ матчится (другой контент)
				{ID: "3", Title: "Судья из ада", Description: ""},                     // НЕ начинает - не матчится
				{ID: "4", Title: "Восставший из ада", Description: ""},                // НЕ начинает - не матчится
				{ID: "5", Title: "Сбежать из ада", Description: ""},                   // НЕ начинает - не матчится
				{ID: "6", Title: "Из ада смотреть онлайн бесплатно", Description: ""}, // стоп-слова - матчится
			},
			phrase:   "Из ада",
			expected: 2, // "Из ада (2001)" и "Из ада смотреть онлайн бесплатно"
		},
		{
			name: "medium three-word phrase - substring match since >12 chars",
			hits: []meili.PageDocument{
				{ID: "1", Title: "Игра в кальмара (2021)", Description: ""},      // содержит - матчится
				{ID: "2", Title: "Игра в кальмара 2", Description: ""},           // содержит - матчится
				{ID: "3", Title: "Смертельная игра в кальмара", Description: ""}, // содержит - матчится (15 символов - не short)
			},
			phrase:   "Игра в кальмара", // 15 символов - NOT short, substring match
			expected: 3,
		},
		{
			name: "long title - normal substring match",
			hits: []meili.PageDocument{
				{ID: "1", Title: "Властелин колец: Братство кольца", Description: ""},
				{ID: "2", Title: "Властелин колец: Две крепости", Description: ""},
				{ID: "3", Title: "История властелин колец", Description: ""}, // даже не в начале - но длинное название
			},
			phrase:   "Властелин колец",
			expected: 3, // длинное название (15 символов) - используется обычный substring
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterHitsByPhrase(tt.hits, tt.phrase)
			if len(result) != tt.expected {
				t.Errorf("filterHitsByPhrase(%q) returned %d hits, want %d", tt.phrase, len(result), tt.expected)
				for _, h := range result {
					t.Logf("  - %s", h.Title)
				}
			}
		})
	}
}

func TestIsShortPhrase(t *testing.T) {
	tests := []struct {
		name     string
		phrase   string
		expected bool
	}{
		{"single word short", "Любовь", true},             // 6 символов, 1 слово - short
		{"single word long", "Интерстеллар", true},        // 12 символов, 1 слово - short
		{"two words short", "Из ада", true},               // 6 символов, 2 слова - short
		{"two words medium", "Темный рыцарь", false},      // 13 символов - NOT short
		{"three words short", "Во все тяжкие", false},     // 13 символов - NOT short
		{"long two-word title", "Властелин колец", false}, // 15 символов - NOT short
		{"very long title", "Пираты Карибского моря", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isShortPhrase(tt.phrase)
			if result != tt.expected {
				t.Errorf("isShortPhrase(%q) = %v, want %v (len=%d, words=%d)",
					tt.phrase, result, tt.expected, len([]rune(tt.phrase)), len(strings.Fields(tt.phrase)))
			}
		})
	}
}

func TestTitleStartsWithPhrase(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		phrase   string
		expected bool
	}{
		{"exact match", "из ада", "из ада", true},
		{"phrase at start with space", "из ада 2001", "из ада", true},
		{"phrase at start with colon", "из ада: продолжение", "из ада", true},
		{"phrase at start with paren", "из ада(2001)", "из ада", true},
		{"phrase in middle", "судья из ада", "из ада", false},
		{"phrase at end", "восставший из ада", "из ада", false},
		{"phrase not present", "другой фильм", "из ада", false},
		{"partial match", "из адамантина", "из ада", false}, // "из ада" не на word boundary
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := titleStartsWithPhrase(tt.title, tt.phrase)
			if result != tt.expected {
				t.Errorf("titleStartsWithPhrase(%q, %q) = %v, want %v", tt.title, tt.phrase, result, tt.expected)
			}
		})
	}
}

func TestFilterHitsByPhraseWithStopWords(t *testing.T) {
	tests := []struct {
		name     string
		hits     []meili.PageDocument
		phrase   string
		expected int
		titles   []string // expected titles in result
	}{
		{
			name: "Между нами - different movie vs SEO garbage",
			hits: []meili.PageDocument{
				{ID: "1", Title: "Между нами горы", Description: ""},                         // другой фильм - НЕ матчится
				{ID: "2", Title: "Между нами смотреть онлайн", Description: ""},              // стоп-слова - матчится
				{ID: "3", Title: "Фильм между нами", Description: ""},                        // НЕ в начале - не матчится
				{ID: "4", Title: "Между нами (2016)", Description: ""},                       // точный матч + год - матчится
				{ID: "5", Title: "Между нами бесплатно в хорошем качестве", Description: ""}, // стоп-слова - матчится
				{ID: "6", Title: "Между нами тает лёд", Description: ""},                     // другой контент - НЕ матчится
			},
			phrase:   "Между нами",
			expected: 3,
			titles:   []string{"Между нами смотреть онлайн", "Между нами (2016)", "Между нами бесплатно в хорошем качестве"},
		},
		{
			name: "Монстр - different movie titles",
			hits: []meili.PageDocument{
				{ID: "1", Title: "Монстр (2003)", Description: ""},          // матчится
				{ID: "2", Title: "Монстр смотреть онлайн", Description: ""}, // стоп-слова - матчится
				{ID: "3", Title: "Монстр в Париже", Description: ""},        // "в Париже" - не стоп-слова - НЕ матчится
				{ID: "4", Title: "Морской монстр", Description: ""},         // не в начале - НЕ матчится
				{ID: "5", Title: "Монстр: История Дамера", Description: ""}, // "История Дамера" - не стоп - НЕ матчится
			},
			phrase:   "Монстр",
			expected: 2,
			titles:   []string{"Монстр (2003)", "Монстр смотреть онлайн"},
		},
		{
			name: "Long phrase - uses substring match",
			hits: []meili.PageDocument{
				{ID: "1", Title: "Властелин колец: Братство кольца", Description: ""},
				{ID: "2", Title: "История властелин колец", Description: ""},
			},
			phrase:   "Властелин колец", // 15 chars - not short, substring mode
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterHitsByPhrase(tt.hits, tt.phrase)
			if len(result) != tt.expected {
				t.Errorf("filterHitsByPhrase(%q) returned %d hits, want %d", tt.phrase, len(result), tt.expected)
				for _, h := range result {
					t.Logf("  got: %s", h.Title)
				}
				if tt.titles != nil {
					t.Logf("  expected titles: %v", tt.titles)
				}
			}
			if tt.titles != nil {
				for i, h := range result {
					if i < len(tt.titles) && h.Title != tt.titles[i] {
						t.Errorf("  result[%d] = %q, want %q", i, h.Title, tt.titles[i])
					}
				}
			}
		})
	}
}
