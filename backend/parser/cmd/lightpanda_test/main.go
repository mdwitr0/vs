package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

func main() {
	// Connect to Lightpanda CDP
	allocCtx, cancel := chromedp.NewRemoteAllocator(context.Background(), "ws://localhost:9222")
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	// Real Chrome User-Agent
	userAgent := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

	var html string
	url := "https://narko-tv.com"

	fmt.Printf("=== Testing %s ===\n", url)
	fmt.Printf("User-Agent: %s\n\n", userAgent)

	start := time.Now()

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(5*time.Second), // Wait for JS to execute
		chromedp.OuterHTML("html", &html),
	)

	elapsed := time.Since(start)

	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Time: %v\n", elapsed)
	fmt.Printf("HTML length: %d\n\n", len(html))

	// Check for captcha indicators
	captchaIndicators := []string{
		"Я не робот",
		"я не робот",
		"onclick",
		"Подтвердите",
		"капча",
		"captcha",
		"похожий цвет",
		"похожую картинку",
		"button",
		"Loading",
		"Идёт загрузка",
		"peel.js",
		"antibot",
	}

	fmt.Println("=== Captcha indicators ===")
	for _, indicator := range captchaIndicators {
		if strings.Contains(html, indicator) {
			fmt.Printf("✓ Found: %s\n", indicator)
		}
	}

	fmt.Println("\n=== Full HTML ===")
	fmt.Println(html)
}
