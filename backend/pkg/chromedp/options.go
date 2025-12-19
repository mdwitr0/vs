package chromedp

import (
	"os"

	"github.com/chromedp/chromedp"
)

// GetExecAllocatorOptions returns chromedp options that work both locally and in Docker
func GetExecAllocatorOptions() []chromedp.ExecAllocatorOption {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", "new"),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("excludeSwitches", "enable-automation"),
		chromedp.Flag("enable-automation", false),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-component-extensions-with-background-pages", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("enable-features", "NetworkService,NetworkServiceInProcess"),
		chromedp.Flag("window-size", "1920,1080"),
		chromedp.UserAgent("Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/140.0.0.0 YaBrowser/25.10.0.0 Safari/537.36"),

		// Stability flags to prevent renderer crashes
		chromedp.Flag("disable-features", "site-per-process,TranslateUI"),
		chromedp.Flag("disable-hang-monitor", true),
		chromedp.Flag("disable-ipc-flooding-protection", true),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.Flag("disable-prompt-on-repost", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("force-color-profile", "srgb"),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("safebrowsing-disable-auto-update", true),
		chromedp.Flag("password-store", "basic"),
		chromedp.Flag("use-mock-keychain", true),

		// Memory/resource limits
		chromedp.Flag("js-flags", "--max-old-space-size=512"),
		chromedp.Flag("disable-client-side-phishing-detection", true),
	)

	// In Docker container, find the Chrome/Chromium executable
	chromePaths := []string{
		"/headless-shell/headless-shell",      // chromedp/headless-shell
		"/usr/bin/chromium-browser",           // zenika/alpine-chrome
		"/usr/bin/chromium",                   // some alpine images
		"/usr/bin/google-chrome",              // debian-based images
		"/usr/bin/google-chrome-stable",       // debian-based images
	}
	for _, p := range chromePaths {
		if _, err := os.Stat(p); err == nil {
			opts = append(opts, chromedp.ExecPath(p))
			break
		}
	}

	return opts
}

// GetStealthScripts returns JavaScript code to inject for anti-detection
func GetStealthScripts() string {
	return `
		// Override navigator.webdriver - CRITICAL
		Object.defineProperty(navigator, 'webdriver', {
			get: () => undefined,
		});

		// Override navigator.headless - CRITICAL for headless detection
		Object.defineProperty(navigator, 'headless', {
			get: () => false,
		});

		// Override navigator.deviceMemory
		Object.defineProperty(navigator, 'deviceMemory', {
			get: () => 8,
		});

		// Override navigator.hardwareConcurrency
		Object.defineProperty(navigator, 'hardwareConcurrency', {
			get: () => 8,
		});

		// Override navigator.vendor
		Object.defineProperty(navigator, 'vendor', {
			get: () => 'Google Inc.',
		});

		// Override navigator.platform
		Object.defineProperty(navigator, 'platform', {
			get: () => 'MacIntel',
		});

		// Override navigator.plugins to simulate real browser
		Object.defineProperty(navigator, 'plugins', {
			get: () => {
				const plugins = [
					{
						name: 'Chrome PDF Plugin',
						description: 'Portable Document Format',
						filename: 'internal-pdf-viewer',
					},
					{
						name: 'Chrome PDF Viewer',
						description: '',
						filename: 'mhjfbmdgcfjbbpaeojofohoefgiehjai',
					},
					{
						name: 'Native Client',
						description: '',
						filename: 'internal-nacl-plugin',
					},
				];
				plugins.item = (i) => plugins[i] || null;
				plugins.namedItem = (name) => plugins.find(p => p.name === name) || null;
				plugins.refresh = () => {};
				return plugins;
			},
		});

		// Override navigator.languages
		Object.defineProperty(navigator, 'languages', {
			get: () => ['ru-RU', 'ru', 'en-US', 'en'],
		});

		// Override chrome runtime and add missing methods
		if (!window.chrome) {
			window.chrome = {};
		}
		if (!window.chrome.runtime) {
			window.chrome.runtime = {
				connect: () => {},
				sendMessage: () => {},
			};
		}

		// Add chrome.loadTimes - often checked by bot detection
		window.chrome.loadTimes = function() {
			return {
				commitLoadTime: Date.now() / 1000,
				connectionInfo: 'h2',
				finishDocumentLoadTime: Date.now() / 1000,
				finishLoadTime: Date.now() / 1000,
				firstPaintAfterLoadTime: 0,
				firstPaintTime: Date.now() / 1000,
				navigationType: 'Other',
				npnNegotiatedProtocol: 'h2',
				requestTime: Date.now() / 1000,
				startLoadTime: Date.now() / 1000,
				wasAlternateProtocolAvailable: false,
				wasFetchedViaSpdy: true,
				wasNpnNegotiated: true
			};
		};

		// Add chrome.csi - often checked by bot detection
		window.chrome.csi = function() {
			return {
				onloadT: Date.now(),
				pageT: Date.now(),
				startE: Date.now(),
				tran: 15
			};
		};

		// Override permissions query
		const originalQuery = window.navigator.permissions.query;
		window.navigator.permissions.query = (parameters) => (
			parameters.name === 'notifications' ?
				Promise.resolve({ state: Notification.permission }) :
				originalQuery(parameters)
		);

		// Override WebGL vendor and renderer
		const getParameter = WebGLRenderingContext.prototype.getParameter;
		WebGLRenderingContext.prototype.getParameter = function(parameter) {
			if (parameter === 37445) {
				return 'Intel Inc.';
			}
			if (parameter === 37446) {
				return 'Intel Iris OpenGL Engine';
			}
			return getParameter.apply(this, arguments);
		};

		// Override screen properties
		Object.defineProperty(screen, 'colorDepth', { get: () => 24 });
		Object.defineProperty(screen, 'pixelDepth', { get: () => 24 });
	`
}
