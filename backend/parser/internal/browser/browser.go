package browser

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/video-analitics/backend/pkg/captcha"
	cdpopts "github.com/video-analitics/backend/pkg/chromedp"
	"github.com/video-analitics/backend/pkg/logger"
)

const profileDir = "/data/browser-profile"

var (
	global *GlobalBrowser
	mu     sync.Mutex
)

// GlobalBrowser is a singleton browser instance for all operations
type GlobalBrowser struct {
	allocCtx      context.Context
	allocCancel   context.CancelFunc
	browserCtx    context.Context
	browserCancel context.CancelFunc
	solver        *captcha.PirateSolver
	semaphore     chan struct{} // limits concurrent tabs
	pageLoadDelay time.Duration
}

// Init initializes the global browser singleton
// Must be called once at application startup
// Set BROWSER_CDP_URL env to use remote browser (e.g., ws://chrome:9222)
func Init(ctx context.Context, solver *captcha.PirateSolver, pageLoadDelay time.Duration, maxTabs int) error {
	mu.Lock()
	defer mu.Unlock()

	if global != nil {
		return fmt.Errorf("browser already initialized")
	}

	if maxTabs < 1 {
		maxTabs = 10
	}

	cdpURL := os.Getenv("BROWSER_CDP_URL")

	var allocCtx context.Context
	var allocCancel context.CancelFunc

	if cdpURL != "" {
		// Remote Chrome browser via CDP
		allocCtx, allocCancel = chromedp.NewRemoteAllocator(ctx, cdpURL)
		logger.Log.Info().Str("cdp_url", cdpURL).Msg("using remote browser")
	} else {
		// Local Chrome
		if err := os.MkdirAll(profileDir, 0755); err != nil {
			return fmt.Errorf("create profile dir: %w", err)
		}

		opts := cdpopts.GetExecAllocatorOptions()
		opts = append(opts, chromedp.UserDataDir(profileDir))

		allocCtx, allocCancel = chromedp.NewExecAllocator(ctx, opts...)
		logger.Log.Info().Str("profile", profileDir).Msg("using local Chrome")
	}

	// Custom error logger that filters out known harmless errors
	errorLogger := func(format string, args ...interface{}) {
		msg := fmt.Sprintf(format, args...)
		// Ignore cookiePartitionKey unmarshal errors (Chrome CDP API change)
		if strings.Contains(msg, "cookiePartitionKey") {
			return
		}
		logger.Log.Error().Msg(msg)
	}

	browserCtx, browserCancel := chromedp.NewContext(allocCtx, chromedp.WithErrorf(errorLogger))

	if err := chromedp.Run(browserCtx); err != nil {
		allocCancel()
		return fmt.Errorf("start browser: %w", err)
	}

	global = &GlobalBrowser{
		allocCtx:      allocCtx,
		allocCancel:   allocCancel,
		browserCtx:    browserCtx,
		browserCancel: browserCancel,
		solver:        solver,
		semaphore:     make(chan struct{}, maxTabs),
		pageLoadDelay: pageLoadDelay,
	}

	logger.Log.Info().Int("max_tabs", maxTabs).Dur("page_load_delay", pageLoadDelay).Msg("global browser initialized")
	return nil
}

// Get returns the global browser instance
// Panics if browser is not initialized
func Get() *GlobalBrowser {
	mu.Lock()
	defer mu.Unlock()

	if global == nil {
		panic("browser not initialized, call browser.Init() first")
	}
	return global
}

// IsInitialized returns true if browser is initialized
func IsInitialized() bool {
	mu.Lock()
	defer mu.Unlock()
	return global != nil
}

// Close shuts down the global browser
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

	logger.Log.Info().Msg("global browser closed")
	global = nil
}

// Context returns the browser context for advanced operations
func (b *GlobalBrowser) Context() context.Context {
	return b.browserCtx
}

// Solver returns the captcha solver
func (b *GlobalBrowser) Solver() *captcha.PirateSolver {
	return b.solver
}

// AcquireWithContext acquires a slot in the semaphore, respecting context cancellation
func (b *GlobalBrowser) AcquireWithContext(ctx context.Context) error {
	select {
	case b.semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release releases a slot in the semaphore
func (b *GlobalBrowser) Release() {
	<-b.semaphore
}
