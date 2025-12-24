package browser

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/chromedp/chromedp"
	"github.com/rs/zerolog/log"
)

const profileDir = "/data/browser-profile"

var (
	global *GlobalBrowser
	mu     sync.Mutex
)

type GlobalBrowser struct {
	allocCtx      context.Context
	allocCancel   context.CancelFunc
	browserCtx    context.Context
	browserCancel context.CancelFunc
	semaphore     chan struct{}
}

func Init(ctx context.Context, maxTabs int) error {
	mu.Lock()
	defer mu.Unlock()

	if global != nil {
		return fmt.Errorf("browser already initialized")
	}

	if maxTabs < 1 {
		maxTabs = 10
	}

	if err := os.MkdirAll(profileDir, 0755); err != nil {
		log.Warn().Err(err).Msg("failed to create profile dir, using temp")
	}

	opts := []chromedp.ExecAllocatorOption{
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Headless,
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Flag("disable-web-security", true),
		chromedp.Flag("disable-setuid-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-accelerated-2d-canvas", true),
		chromedp.Flag("disable-background-networking", true),
		chromedp.Flag("disable-background-timer-throttling", true),
		chromedp.Flag("disable-backgrounding-occluded-windows", true),
		chromedp.Flag("disable-breakpad", true),
		chromedp.Flag("disable-component-extensions-with-background-pages", true),
		chromedp.Flag("disable-component-update", true),
		chromedp.Flag("disable-default-apps", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.Flag("disable-features", "TranslateUI"),
		chromedp.Flag("disable-hang-monitor", true),
		chromedp.Flag("disable-ipc-flooding-protection", true),
		chromedp.Flag("disable-popup-blocking", true),
		chromedp.Flag("disable-prompt-on-repost", true),
		chromedp.Flag("disable-renderer-backgrounding", true),
		chromedp.Flag("disable-sync", true),
		chromedp.Flag("force-color-profile", "srgb"),
		chromedp.Flag("metrics-recording-only", true),
		chromedp.Flag("safebrowsing-disable-auto-update", true),
		chromedp.Flag("enable-automation", true),
		chromedp.Flag("password-store", "basic"),
		chromedp.Flag("use-mock-keychain", true),
		chromedp.UserAgent("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"),
		chromedp.WindowSize(1920, 1080),
	}

	if _, err := os.Stat(profileDir); err == nil {
		opts = append(opts, chromedp.UserDataDir(profileDir))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	browserCtx, browserCancel := chromedp.NewContext(allocCtx)

	if err := chromedp.Run(browserCtx); err != nil {
		allocCancel()
		return fmt.Errorf("start browser: %w", err)
	}

	global = &GlobalBrowser{
		allocCtx:      allocCtx,
		allocCancel:   allocCancel,
		browserCtx:    browserCtx,
		browserCancel: browserCancel,
		semaphore:     make(chan struct{}, maxTabs),
	}

	log.Info().Int("max_tabs", maxTabs).Msg("browser initialized")
	return nil
}

func Get() *GlobalBrowser {
	mu.Lock()
	defer mu.Unlock()

	if global == nil {
		panic("browser not initialized")
	}
	return global
}

func IsInitialized() bool {
	mu.Lock()
	defer mu.Unlock()
	return global != nil
}

func Close() {
	mu.Lock()
	defer mu.Unlock()

	if global == nil {
		return
	}

	if global.browserCancel != nil {
		global.browserCancel()
	}
	if global.allocCancel != nil {
		global.allocCancel()
	}

	log.Info().Msg("browser closed")
	global = nil
}

func (b *GlobalBrowser) Context() context.Context {
	return b.browserCtx
}

func (b *GlobalBrowser) AcquireWithContext(ctx context.Context) error {
	select {
	case b.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (b *GlobalBrowser) Release() {
	<-b.semaphore
}
