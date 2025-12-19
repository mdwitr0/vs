package detector

import "regexp"

// DLE
var (
	dleRootPattern      = regexp.MustCompile(`var\s+dle_root\s*=`)
	dleAdminPattern     = regexp.MustCompile(`var\s+dle_admin\s*=`)
	dleLoginHashPattern = regexp.MustCompile(`var\s+dle_login_hash\s*=`)
	dleSkinPattern      = regexp.MustCompile(`var\s+dle_skin\s*=\s*['"]([^'"]+)['"]`)
	dleGroupPattern     = regexp.MustCompile(`var\s+dle_group\s*=`)
	dleEnginePattern    = regexp.MustCompile(`/engine/(?:classes|ajax|modules)/`)
	dleCommentsPattern  = regexp.MustCompile(`(?i)id\s*=\s*["']dle-comments`)
	dleGeneratorPattern = regexp.MustCompile(`(?i)<meta[^>]+generator[^>]+DataLife\s*Engine`)
)

// WordPress
var (
	wpContentPattern   = regexp.MustCompile(`/wp-content/`)
	wpIncludesPattern  = regexp.MustCompile(`/wp-includes/`)
	wpAdminAjaxPattern = regexp.MustCompile(`/wp-admin/admin-ajax\.php`)
	wpJSONPattern      = regexp.MustCompile(`/wp-json/`)
	wpGeneratorPattern = regexp.MustCompile(`(?i)<meta[^>]+generator[^>]+WordPress\s*([\d.]+)?`)
	wpBlockPattern     = regexp.MustCompile(`class\s*=\s*["'][^"']*wp-block-`)
	wpAPILinkPattern   = regexp.MustCompile(`rel\s*=\s*["']https://api\.w\.org/["']`)
)

// CinemaPress
var (
	cpPoweredByValue  = "CinemaPress"
	cpVerPattern      = regexp.MustCompile(`CP_VER\s*=`)
	cpConfigPattern   = regexp.MustCompile(`CP_CONFIG_MD5\s*=`)
	cpMovieURLPattern = regexp.MustCompile(`/(?:movie|film|tv)/cp\d+`)
	cpMobilePattern   = regexp.MustCompile(`/mobile-version/`)
	cpTVPattern       = regexp.MustCompile(`/tv-version/`)
)

// uCoz
var (
	ucozWindowPattern    = regexp.MustCompile(`window\.uCoz\s*=\s*\{`)
	ucozFunctionsPattern = regexp.MustCompile(`_uPostForm|_uWnd|_uAjaxRequest`)
	ucozHostPattern      = regexp.MustCompile(`\.ucoz\.(ru|com|net|org)`)
)

// SPA/Framework
var (
	reactRootPattern  = regexp.MustCompile(`<div\s+id\s*=\s*["']root["']\s*>\s*</div>`)
	reactDOMPattern   = regexp.MustCompile(`react-dom|ReactDOM`)
	vueAppPattern     = regexp.MustCompile(`<div\s+id\s*=\s*["']app["']\s*>\s*</div>`)
	vuePattern        = regexp.MustCompile(`Vue\s*\(|new\s+Vue`)
	nextDataPattern   = regexp.MustCompile(`<script\s+id\s*=\s*["']__NEXT_DATA__["']`)
	nextStaticPattern = regexp.MustCompile(`/_next/static/`)
	nextSSPPattern    = regexp.MustCompile(`"__N_SSP"\s*:\s*true`)
	nuxtPattern       = regexp.MustCompile(`__NUXT__|_nuxt/`)
)

// Captcha
var (
	recaptchaScriptPattern  = regexp.MustCompile(`(?i)google\.com/recaptcha/api`)
	recaptchaClassPattern   = regexp.MustCompile(`class\s*=\s*["'][^"']*g-recaptcha`)
	recaptchaSitekeyPattern = regexp.MustCompile(`data-sitekey\s*=`)
	hcaptchaScriptPattern   = regexp.MustCompile(`(?i)hcaptcha\.com/1/api`)
	hcaptchaClassPattern    = regexp.MustCompile(`class\s*=\s*["'][^"']*h-captcha`)
	cfVerificationPattern   = regexp.MustCompile(`(?i)cf-browser-verification|cloudflare`)
	cfChallengePattern      = regexp.MustCompile(`(?i)checking\s+your\s+browser|just\s+a\s+moment`)
	ddosGuardPattern        = regexp.MustCompile(`(?i)ddos-guard\.net|__ddg1|__ddg2`)
	dleAntibotPattern       = regexp.MustCompile(`/engine/modules/antibot/`)
	ucozCaptchaPattern      = regexp.MustCompile(`/secure/\?k=`)
	genericCaptchaPattern   = regexp.MustCompile(`(?i)captcha|verify.*human`)

	// Pirate captcha - кнопки "Я не робот" с display:none в стилях
	pirateCaptchaButtonPattern = regexp.MustCompile(`(?is)<div[^>]*onclick\s*=\s*["'][^"']+["'][^>]*>\s*Я не робот\s*</div>`)
	pirateCaptchaTextPattern   = regexp.MustCompile(`(?i)Подтвердите,?\s*что\s*вы\s*(не\s*)?(робот|человек)`)
	pirateCaptchaStylePattern  = regexp.MustCompile(`(?is)<style[^>]*>[^<]*display\s*:\s*none[^<]*</style>`)
)

// Sitemap
var (
	sitemapPaths = []string{
		"/sitemap.xml",
		"/sitemap_index.xml",
		"/wp-sitemap.xml",
		"/sitemap/sitemap.xml",
		"/post-sitemap.xml",
	}
	sitemapXMLPattern = regexp.MustCompile(`<urlset|<sitemapindex`)
)

// SSR/CSR detection
var (
	minTextContentLength = 500
	scriptTagPattern     = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	styleTagPattern      = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	htmlTagPattern       = regexp.MustCompile(`<[^>]+>`)
	whitespacePattern    = regexp.MustCompile(`\s+`)
)

// WAF/Block detection - страницы блокировки
var (
	blockRequestDeniedPattern = regexp.MustCompile(`(?i)request\s+(has\s+been\s+)?denied`)
	blockAccessDeniedPattern  = regexp.MustCompile(`(?i)access\s+denied|forbidden`)
	blockErrorTitlePattern    = regexp.MustCompile(`(?i)<title>\s*(error|blocked|denied|forbidden)\s*</title>`)
	blockEmptyBodyPattern     = regexp.MustCompile(`(?is)^<!DOCTYPE[^>]*>\s*<html[^>]*>\s*<head>.*?</head>\s*<body[^>]*>\s*(<[^>]+>\s*){0,5}</body>\s*</html>\s*$`)
)
