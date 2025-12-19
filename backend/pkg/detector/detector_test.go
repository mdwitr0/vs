package detector

import (
	"testing"
)

// TestCMSDetector_DLE тестирует детекцию DLE
func TestCMSDetector_DLE(t *testing.T) {
	detector := NewCMSDetector()

	tests := []struct {
		name     string
		html     string
		headers  map[string]string
		wantCMS  CMS
		wantConf float64
	}{
		{
			name: "DLE by dle_root variable",
			html: `<html><head><script>var dle_root = '/';</script></head><body></body></html>`,
			headers: map[string]string{},
			wantCMS: CMSDLE,
			wantConf: 0.9,
		},
		{
			name: "DLE by dle_skin variable",
			html: `<html><script>var dle_skin = 'kino-2023';</script></html>`,
			headers: map[string]string{},
			wantCMS: CMSDLE,
			wantConf: 0.8,
		},
		{
			name: "DLE by engine path",
			html: `<html><script src="/engine/classes/js/dle.js"></script></html>`,
			headers: map[string]string{},
			wantCMS: CMSDLE,
			wantConf: 0.8,
		},
		{
			name: "DLE by meta generator",
			html: `<html><head><meta name="generator" content="DataLife Engine"></head></html>`,
			headers: map[string]string{},
			wantCMS: CMSDLE,
			wantConf: 1.0,
		},
		{
			name: "DLE multiple markers",
			html: `<html>
				<head><meta name="generator" content="DataLife Engine"></head>
				<script>var dle_root = '/'; var dle_login_hash = 'abc123';</script>
				<script src="/engine/classes/js/dle.js"></script>
			</html>`,
			headers: map[string]string{},
			wantCMS: CMSDLE,
			wantConf: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.Detect(tt.html, tt.headers)

			if result.CMS != tt.wantCMS {
				t.Errorf("CMS = %v, want %v", result.CMS, tt.wantCMS)
			}

			if result.Confidence < tt.wantConf-0.15 {
				t.Errorf("Confidence = %v, want >= %v", result.Confidence, tt.wantConf-0.15)
			}
		})
	}
}

// TestCMSDetector_WordPress тестирует детекцию WordPress
func TestCMSDetector_WordPress(t *testing.T) {
	detector := NewCMSDetector()

	tests := []struct {
		name    string
		html    string
		headers map[string]string
		wantCMS CMS
	}{
		{
			name: "WordPress by wp-content path",
			html: `<html><link rel="stylesheet" href="/wp-content/themes/theme/style.css"></html>`,
			headers: map[string]string{},
			wantCMS: CMSWordPress,
		},
		{
			name: "WordPress by meta generator",
			html: `<html><meta name="generator" content="WordPress 6.4.2"></html>`,
			headers: map[string]string{},
			wantCMS: CMSWordPress,
		},
		{
			name: "WordPress by admin-ajax",
			html: `<html><script>ajaxUrl: "/wp-admin/admin-ajax.php"</script></html>`,
			headers: map[string]string{},
			wantCMS: CMSWordPress,
		},
		{
			name: "WordPress by api.w.org header",
			html: `<html></html>`,
			headers: map[string]string{
				"link": `<https://example.com/wp-json/>; rel="https://api.w.org/"`,
			},
			wantCMS: CMSWordPress,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.Detect(tt.html, tt.headers)

			if result.CMS != tt.wantCMS {
				t.Errorf("CMS = %v, want %v", result.CMS, tt.wantCMS)
			}
		})
	}
}

// TestCMSDetector_CinemaPress тестирует детекцию CinemaPress
func TestCMSDetector_CinemaPress(t *testing.T) {
	detector := NewCMSDetector()

	tests := []struct {
		name    string
		html    string
		headers map[string]string
		wantCMS CMS
	}{
		{
			name: "CinemaPress by X-Powered-By header",
			html: `<html></html>`,
			headers: map[string]string{
				"x-powered-by": "CinemaPress",
			},
			wantCMS: CMSCinemaPress,
		},
		{
			name: "CinemaPress by CP_VER variable",
			html: `<html><script>CP_VER = '12345';</script></html>`,
			headers: map[string]string{},
			wantCMS: CMSCinemaPress,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.Detect(tt.html, tt.headers)

			if result.CMS != tt.wantCMS {
				t.Errorf("CMS = %v, want %v", result.CMS, tt.wantCMS)
			}
		})
	}
}

// TestCMSDetector_uCoz тестирует детекцию uCoz
func TestCMSDetector_uCoz(t *testing.T) {
	detector := NewCMSDetector()

	tests := []struct {
		name    string
		html    string
		headers map[string]string
		wantCMS CMS
	}{
		{
			name: "uCoz by window.uCoz",
			html: `<html><script>window.uCoz = {"language":"ru","site":{"domain":"example.ru"}};</script></html>`,
			headers: map[string]string{},
			wantCMS: CMSUCoz,
		},
		{
			name: "uCoz by functions",
			html: `<html><script>_uPostForm(); _uWnd.create();</script></html>`,
			headers: map[string]string{},
			wantCMS: CMSUCoz,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.Detect(tt.html, tt.headers)

			if result.CMS != tt.wantCMS {
				t.Errorf("CMS = %v, want %v", result.CMS, tt.wantCMS)
			}
		})
	}
}

// TestRenderDetector тестирует детекцию SSR/CSR
func TestRenderDetector(t *testing.T) {
	detector := NewRenderDetector()

	tests := []struct {
		name           string
		html           string
		contentLength  int64
		wantRenderType RenderType
		wantFramework  Framework
		wantBrowser    bool
	}{
		{
			name: "SSR - full content",
			html: `<html><body>
				<h1>Название фильма</h1>
				<p>Описание фильма с большим количеством текста для определения SSR рендеринга.
				Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
				Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.
				Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.</p>
			</body></html>`,
			contentLength:  5000,
			wantRenderType: RenderSSR,
			wantFramework:  FrameworkNone,
			wantBrowser:    false,
		},
		{
			name: "CSR - empty root div",
			html: `<html><body><div id="root"></div><script src="/bundle.js"></script></body></html>`,
			contentLength:  100000,
			wantRenderType: RenderCSR,
			wantFramework:  FrameworkNone,
			wantBrowser:    true,
		},
		{
			name: "Next.js SSR",
			html: `<html><body>
				<script id="__NEXT_DATA__" type="application/json">{"props":{}}</script>
				<div>Много контента здесь, фильм описание и так далее, достаточно текста для SSR определения.
				Lorem ipsum dolor sit amet, consectetur adipiscing elit. Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.
				Ut enim ad minim veniam, quis nostrud exercitation ullamco laboris nisi ut aliquip ex ea commodo consequat.
				Duis aute irure dolor in reprehenderit in voluptate velit esse cillum dolore eu fugiat nulla pariatur.
				Excepteur sint occaecat cupidatat non proident, sunt in culpa qui officia deserunt mollit anim id est laborum.</div>
			</body></html>`,
			contentLength:  10000,
			wantRenderType: RenderSSR,
			wantFramework:  FrameworkNextJS,
			wantBrowser:    false,
		},
		{
			name: "Nuxt",
			html: `<html><body><script>window.__NUXT__={}</script></body></html>`,
			contentLength:  50000,
			wantRenderType: RenderCSR,
			wantFramework:  FrameworkNuxt,
			wantBrowser:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.Detect(tt.html, tt.contentLength)

			if result.RenderType != tt.wantRenderType {
				t.Errorf("RenderType = %v, want %v", result.RenderType, tt.wantRenderType)
			}

			if result.Framework != tt.wantFramework {
				t.Errorf("Framework = %v, want %v", result.Framework, tt.wantFramework)
			}

			if result.NeedsBrowser != tt.wantBrowser {
				t.Errorf("NeedsBrowser = %v, want %v", result.NeedsBrowser, tt.wantBrowser)
			}
		})
	}
}

// TestCaptchaDetector тестирует детекцию капчи
func TestCaptchaDetector(t *testing.T) {
	detector := NewCaptchaDetector()

	tests := []struct {
		name        string
		html        string
		headers     map[string]string
		wantCaptcha CaptchaType
	}{
		{
			name:        "No captcha",
			html:        `<html><body>Normal content</body></html>`,
			headers:     map[string]string{},
			wantCaptcha: CaptchaNone,
		},
		{
			name:        "reCAPTCHA",
			html:        `<html><script src="https://www.google.com/recaptcha/api.js"></script><div class="g-recaptcha"></div></html>`,
			headers:     map[string]string{},
			wantCaptcha: CaptchaReCAPTCHA,
		},
		{
			name:        "hCaptcha",
			html:        `<html><script src="https://hcaptcha.com/1/api.js"></script><div class="h-captcha"></div></html>`,
			headers:     map[string]string{},
			wantCaptcha: CaptchaHCaptcha,
		},
		{
			name: "Cloudflare challenge",
			html: `<html><body>
				<div id="cf-browser-verification">Checking your browser before accessing</div>
			</body></html>`,
			headers:     map[string]string{"server": "cloudflare"},
			wantCaptcha: CaptchaCloudflare,
		},
		{
			name:        "DLE Antibot",
			html:        `<html><img src="/engine/modules/antibot/antibot.php?abc"></html>`,
			headers:     map[string]string{},
			wantCaptcha: CaptchaDLEAntibot,
		},
		{
			name:        "uCoz captcha",
			html:        `<html><img src="/secure/?k=abc&m=addcom"></html>`,
			headers:     map[string]string{},
			wantCaptcha: CaptchaUCoz,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detector.Detect(tt.html, tt.headers)

			if result.Type != tt.wantCaptcha {
				t.Errorf("CaptchaType = %v, want %v", result.Type, tt.wantCaptcha)
			}
		})
	}
}

// TestSubdomainPerMovie тестирует детекцию поддомена на фильм
func TestSubdomainPerMovie(t *testing.T) {
	d := &detector{}

	tests := []struct {
		url  string
		want bool
	}{
		{"https://hatiko-samyj-vernyj-drug-2009.kino-lordfilm2.org/", true},
		{"https://matrix-1999-reloaded.movies.example.com/", true},
		{"https://www.example.com/", false},
		{"https://m.example.com/", false},
		{"https://api.example.com/", false},
		{"https://example.com/", false},
		{"https://short.example.com/", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := d.detectSubdomainPerMovie(tt.url)
			if got != tt.want {
				t.Errorf("detectSubdomainPerMovie(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}
