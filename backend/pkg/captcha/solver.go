package captcha

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	cdpopts "github.com/video-analitics/backend/pkg/chromedp"
	"github.com/video-analitics/backend/pkg/logger"
)

type CaptchaType string

const (
	CaptchaTypeUnknown CaptchaType = "unknown"
	CaptchaTypeButton  CaptchaType = "button"
	CaptchaTypeColor   CaptchaType = "color"
	CaptchaTypeImage   CaptchaType = "image"
)

type Cookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Expires  int64  `json:"expires,omitempty"`
	HTTPOnly bool   `json:"http_only"`
	Secure   bool   `json:"secure"`
}

type SolveResult struct {
	Success     bool
	CaptchaType CaptchaType
	Cookies     []Cookie
	HTML        string
	Error       error
	Attempts    int
}

type PirateSolver struct {
	timeout     time.Duration
	maxAttempts int
}

func NewPirateSolver() *PirateSolver {
	return &PirateSolver{
		timeout:     60 * time.Second,
		maxAttempts: 5,
	}
}

func (s *PirateSolver) Solve(ctx context.Context, url string) (*SolveResult, error) {
	opts := cdpopts.GetExecAllocatorOptions()

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	timeoutCtx, timeoutCancel := context.WithTimeout(browserCtx, s.timeout)
	defer timeoutCancel()

	err := chromedp.Run(timeoutCtx,
		chromedp.Evaluate(cdpopts.GetStealthScripts(), nil),
		chromedp.Navigate(url),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil {
		return &SolveResult{Success: false, Error: fmt.Errorf("navigation failed: %w", err)}, nil
	}

	var solved bool
	var captchaType CaptchaType
	var attempts int

	for attempts = 1; attempts <= s.maxAttempts; attempts++ {
		captchaType = s.detectCaptchaType(timeoutCtx)
		logger.Log.Info().
			Str("url", url).
			Str("type", string(captchaType)).
			Int("attempt", attempts).
			Msg("detected captcha type")

		switch captchaType {
		case CaptchaTypeButton:
			solved, err = s.solveButtonCaptcha(timeoutCtx)
			if err == nil && solved {
				goto success
			}

		case CaptchaTypeColor:
			solved, err = s.solveColorCaptcha(timeoutCtx)
			if err == nil && solved {
				goto success
			}
			logger.Log.Info().Int("attempt", attempts).Msg("color captcha failed, refreshing for easier type")

		case CaptchaTypeImage:
			solved, err = s.solveImageCaptcha(timeoutCtx)
			if err == nil && solved {
				goto success
			}
			logger.Log.Info().Int("attempt", attempts).Msg("image captcha failed, refreshing for easier type")

		default:
			logger.Log.Warn().Str("type", string(captchaType)).Msg("unknown captcha type, refreshing")
		}

		if attempts < s.maxAttempts {
			err = chromedp.Run(timeoutCtx,
				chromedp.Reload(),
				chromedp.Sleep(2*time.Second),
			)
			if err != nil {
				logger.Log.Warn().Err(err).Msg("refresh failed")
				break
			}
		}
	}

	return &SolveResult{
		Success:     false,
		CaptchaType: captchaType,
		Error:       fmt.Errorf("failed after %d attempts", attempts),
		Attempts:    attempts,
	}, nil

success:
	err = chromedp.Run(timeoutCtx, chromedp.Sleep(3*time.Second))
	if err != nil {
		return &SolveResult{Success: false, CaptchaType: captchaType, Error: err, Attempts: attempts}, nil
	}

	var cookies []*network.Cookie
	var finalHTML string
	err = chromedp.Run(timeoutCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cookies, err = network.GetCookies().Do(ctx)
			return err
		}),
		chromedp.OuterHTML("html", &finalHTML),
	)
	if err != nil {
		return &SolveResult{Success: false, CaptchaType: captchaType, Error: fmt.Errorf("failed to get cookies: %w", err), Attempts: attempts}, nil
	}

	result := &SolveResult{
		Success:     true,
		CaptchaType: captchaType,
		Cookies:     make([]Cookie, len(cookies)),
		HTML:        finalHTML,
		Attempts:    attempts,
	}
	for i, c := range cookies {
		result.Cookies[i] = Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  int64(c.Expires),
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
		}
	}

	logger.Log.Info().
		Str("type", string(captchaType)).
		Int("attempts", attempts).
		Int("cookies", len(cookies)).
		Msg("captcha solved successfully")

	return result, nil
}

func (s *PirateSolver) SolveInContext(browserCtx context.Context, url string) (*SolveResult, error) {
	tabCtx, tabCancel := chromedp.NewContext(browserCtx)
	defer tabCancel()

	timeoutCtx, timeoutCancel := context.WithTimeout(tabCtx, s.timeout)
	defer timeoutCancel()

	err := chromedp.Run(timeoutCtx,
		chromedp.Evaluate(cdpopts.GetStealthScripts(), nil),
		chromedp.Navigate(url),
		chromedp.Sleep(3*time.Second),
	)
	if err != nil {
		return &SolveResult{Success: false, Error: fmt.Errorf("navigation failed: %w", err)}, nil
	}

	var solved bool
	var captchaType CaptchaType
	var attempts int

	for attempts = 1; attempts <= s.maxAttempts; attempts++ {
		captchaType = s.detectCaptchaType(timeoutCtx)
		logger.Log.Info().
			Str("url", url).
			Str("type", string(captchaType)).
			Int("attempt", attempts).
			Msg("detected captcha type (in context)")

		switch captchaType {
		case CaptchaTypeButton:
			solved, err = s.solveButtonCaptcha(timeoutCtx)
			if err == nil && solved {
				goto success
			}

		case CaptchaTypeColor:
			solved, err = s.solveColorCaptcha(timeoutCtx)
			if err == nil && solved {
				goto success
			}
			logger.Log.Info().Int("attempt", attempts).Msg("color captcha failed, refreshing for easier type")

		case CaptchaTypeImage:
			solved, err = s.solveImageCaptcha(timeoutCtx)
			if err == nil && solved {
				goto success
			}
			logger.Log.Info().Int("attempt", attempts).Msg("image captcha failed, refreshing for easier type")

		default:
			logger.Log.Warn().Str("type", string(captchaType)).Msg("unknown captcha type, refreshing")
		}

		if attempts < s.maxAttempts {
			err = chromedp.Run(timeoutCtx,
				chromedp.Reload(),
				chromedp.Sleep(2*time.Second),
			)
			if err != nil {
				logger.Log.Warn().Err(err).Msg("refresh failed")
				break
			}
		}
	}

	return &SolveResult{
		Success:     false,
		CaptchaType: captchaType,
		Error:       fmt.Errorf("failed after %d attempts", attempts),
		Attempts:    attempts,
	}, nil

success:
	err = chromedp.Run(timeoutCtx, chromedp.Sleep(3*time.Second))
	if err != nil {
		return &SolveResult{Success: false, CaptchaType: captchaType, Error: err, Attempts: attempts}, nil
	}

	var cookies []*network.Cookie
	var finalHTML string
	err = chromedp.Run(timeoutCtx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cookies, err = network.GetCookies().Do(ctx)
			return err
		}),
		chromedp.OuterHTML("html", &finalHTML),
	)
	if err != nil {
		return &SolveResult{Success: false, CaptchaType: captchaType, Error: fmt.Errorf("failed to get cookies: %w", err), Attempts: attempts}, nil
	}

	result := &SolveResult{
		Success:     true,
		CaptchaType: captchaType,
		Cookies:     make([]Cookie, len(cookies)),
		HTML:        finalHTML,
		Attempts:    attempts,
	}
	for i, c := range cookies {
		result.Cookies[i] = Cookie{
			Name:     c.Name,
			Value:    c.Value,
			Domain:   c.Domain,
			Path:     c.Path,
			Expires:  int64(c.Expires),
			HTTPOnly: c.HTTPOnly,
			Secure:   c.Secure,
		}
	}

	logger.Log.Info().
		Str("type", string(captchaType)).
		Int("attempts", attempts).
		Int("cookies", len(cookies)).
		Msg("captcha solved successfully (in context)")

	return result, nil
}

func (s *PirateSolver) detectCaptchaType(ctx context.Context) CaptchaType {
	var result string
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				const info = document.querySelector('.info, #content, .block2');
				if (!info) return 'unknown';

				const text = info.textContent || '';

				if (text.includes('похожий цвет') || text.includes('нажмите на похожий цвет')) {
					return 'color';
				}
				if (text.includes('похожую картинку') || text.includes('нажмите на похожую картинку')) {
					return 'image';
				}
				if (text.includes('Я не робот') || text.includes('не робот')) {
					return 'button';
				}

				const styleBlock = info.querySelector('style');
				if (styleBlock && styleBlock.textContent.includes('background-color')) {
					const emElements = info.querySelectorAll('em[onclick]');
					if (emElements.length > 0) return 'color';
				}

				const images = info.querySelectorAll('img[onclick]');
				if (images.length > 0) return 'image';

				const buttons = info.querySelectorAll('div[onclick]');
				for (const btn of buttons) {
					if (btn.textContent.trim() === 'Я не робот') return 'button';
				}

				return 'unknown';
			})()
		`, &result),
	)
	if err != nil {
		return CaptchaTypeUnknown
	}

	switch result {
	case "button":
		return CaptchaTypeButton
	case "color":
		return CaptchaTypeColor
	case "image":
		return CaptchaTypeImage
	default:
		return CaptchaTypeUnknown
	}
}

func (s *PirateSolver) solveButtonCaptcha(ctx context.Context) (bool, error) {
	var clicked bool
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				const buttons = document.querySelectorAll('div[onclick]');
				for (const btn of buttons) {
					const text = btn.textContent.trim();
					if (text === 'Я не робот') {
						const rect = btn.getBoundingClientRect();
						if (rect.width > 0 && rect.height > 0 && rect.top >= 0) {
							btn.click();
							return true;
						}
					}
				}
				return false;
			})()
		`, &clicked),
	)
	return clicked, err
}

func (s *PirateSolver) solveColorCaptcha(ctx context.Context) (bool, error) {
	var result map[string]interface{}
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				const info = document.querySelector('.info, #content, .block2');
				if (!info) return { success: false, reason: 'no info block' };

				let targetColor = null;
				const divs = info.querySelectorAll('div[style*="background-color"]');
				for (const div of divs) {
					const style = div.getAttribute('style') || '';
					if (style.includes('pointer-events: none') || style.includes('cursor: none')) {
						targetColor = div.style.backgroundColor;
						break;
					}
				}

				if (!targetColor) {
					const firstDiv = info.querySelector('div[style*="background-color"]');
					if (firstDiv) targetColor = firstDiv.style.backgroundColor;
				}

				if (!targetColor) return { success: false, reason: 'no target color found' };

				const targetRGB = parseColor(targetColor);
				if (!targetRGB) return { success: false, reason: 'cannot parse target color: ' + targetColor };

				const styleBlock = info.querySelector('style');
				if (!styleBlock) return { success: false, reason: 'no style block' };

				const cssText = styleBlock.textContent;
				const colorMap = {};

				const regex = /\.([a-zA-Z0-9_-]+)\s*\{([^}]+)\}/g;
				let match;
				while ((match = regex.exec(cssText)) !== null) {
					const className = match[1];
					const rules = match[2];

					if (/display\s*:\s*none/i.test(rules)) continue;

					const colorMatch = rules.match(/background-color\s*:\s*(#[0-9a-fA-F]{3,6})/i);
					if (colorMatch) {
						colorMap[className] = colorMatch[1];
					}
				}

				if (Object.keys(colorMap).length === 0) {
					return { success: false, reason: 'no colors parsed from style' };
				}

				const emElements = info.querySelectorAll('em[onclick]');
				let bestElement = null;
				let bestDistance = Infinity;
				let bestColor = null;

				for (const em of emElements) {
					const rect = em.getBoundingClientRect();
					if (rect.width === 0 || rect.height === 0) continue;

					const computedStyle = window.getComputedStyle(em);
					if (computedStyle.display === 'none') continue;

					for (const cls of em.classList) {
						if (colorMap[cls]) {
							const rgb = hexToRGB(colorMap[cls]);
							if (rgb) {
								const dist = colorDistance(targetRGB, rgb);
								if (dist < bestDistance) {
									bestDistance = dist;
									bestElement = em;
									bestColor = colorMap[cls];
								}
							}
						}
					}
				}

				if (bestElement) {
					bestElement.click();
					return {
						success: true,
						distance: bestDistance,
						targetColor: targetColor,
						matchedColor: bestColor
					};
				}

				return { success: false, reason: 'no matching element found' };

				function parseColor(str) {
					if (!str) return null;
					str = str.trim();
					if (str.startsWith('#')) return hexToRGB(str);
					const m = str.match(/rgb\s*\(\s*(\d+)\s*,\s*(\d+)\s*,\s*(\d+)\s*\)/i);
					if (m) return { r: parseInt(m[1]), g: parseInt(m[2]), b: parseInt(m[3]) };
					return null;
				}

				function hexToRGB(hex) {
					if (!hex) return null;
					hex = hex.replace('#', '');
					if (hex.length === 3) {
						hex = hex[0]+hex[0]+hex[1]+hex[1]+hex[2]+hex[2];
					}
					if (hex.length !== 6) return null;
					return {
						r: parseInt(hex.substr(0, 2), 16),
						g: parseInt(hex.substr(2, 2), 16),
						b: parseInt(hex.substr(4, 2), 16)
					};
				}

				function colorDistance(c1, c2) {
					return Math.sqrt(
						Math.pow(c1.r - c2.r, 2) +
						Math.pow(c1.g - c2.g, 2) +
						Math.pow(c1.b - c2.b, 2)
					);
				}
			})()
		`, &result),
	)
	if err != nil {
		return false, err
	}

	if result == nil {
		return false, fmt.Errorf("no result from color captcha solver")
	}

	success, ok := result["success"].(bool)
	if !ok {
		return false, fmt.Errorf("invalid result format")
	}

	if !success {
		reason, _ := result["reason"].(string)
		logger.Log.Debug().Str("reason", reason).Msg("color captcha failed")
		return false, nil
	}

	logger.Log.Debug().
		Interface("distance", result["distance"]).
		Interface("targetColor", result["targetColor"]).
		Interface("matchedColor", result["matchedColor"]).
		Msg("color captcha solved")

	return true, nil
}

func (s *PirateSolver) solveImageCaptcha(ctx context.Context) (bool, error) {
	// Получаем информацию о картинках через JS
	var imgInfo struct {
		TargetSelector     string   `json:"targetSelector"`
		CandidateSelectors []string `json:"candidateSelectors"`
		Error              string   `json:"error"`
	}

	err := chromedp.Run(ctx,
		chromedp.Evaluate(`
			(function() {
				const info = document.querySelector('.info, #content, .block2');
				if (!info) return { error: 'no info block' };

				// Найти образец
				let targetImg = info.querySelector('img[src^="data:image"]');
				if (!targetImg) {
					const allImgs = info.querySelectorAll('img');
					for (const img of allImgs) {
						if (!img.hasAttribute('onclick')) {
							targetImg = img;
							break;
						}
					}
				}
				if (!targetImg) return { error: 'no target image' };

				// Создаём уникальный селектор для target
				targetImg.setAttribute('data-captcha-target', 'true');

				// Найти кандидатов
				const candidates = info.querySelectorAll('img[onclick]');
				if (candidates.length === 0) return { error: 'no candidates' };

				const candidateSelectors = [];
				candidates.forEach((img, i) => {
					img.setAttribute('data-captcha-candidate', i.toString());
					candidateSelectors.push('[data-captcha-candidate="' + i + '"]');
				});

				return {
					targetSelector: '[data-captcha-target="true"]',
					candidateSelectors: candidateSelectors
				};
			})()
		`, &imgInfo),
	)
	if err != nil {
		return false, err
	}

	if imgInfo.Error != "" {
		logger.Log.Debug().Str("error", imgInfo.Error).Msg("image captcha: setup failed")
		return false, nil
	}

	// Делаем скриншот образца
	var targetScreenshot []byte
	err = chromedp.Run(ctx,
		chromedp.Screenshot(imgInfo.TargetSelector, &targetScreenshot, chromedp.NodeVisible),
	)
	if err != nil {
		logger.Log.Debug().Err(err).Msg("image captcha: failed to screenshot target")
		return false, nil
	}

	// Делаем скриншоты кандидатов и сравниваем
	bestIndex := -1
	bestScore := float64(1000000)
	scores := make([]float64, len(imgInfo.CandidateSelectors))

	for i, selector := range imgInfo.CandidateSelectors {
		var candidateScreenshot []byte
		err = chromedp.Run(ctx,
			chromedp.Screenshot(selector, &candidateScreenshot, chromedp.NodeVisible),
		)
		if err != nil {
			logger.Log.Debug().Err(err).Int("index", i).Msg("image captcha: failed to screenshot candidate")
			scores[i] = 1000000
			continue
		}

		score := compareImages(targetScreenshot, candidateScreenshot)
		scores[i] = score

		if score < bestScore {
			bestScore = score
			bestIndex = i
		}
	}

	// Проверяем confidence (разница между лучшим и вторым)
	sortedScores := make([]float64, len(scores))
	copy(sortedScores, scores)
	sortFloat64s(sortedScores)

	confidence := float64(0)
	if len(sortedScores) > 1 && sortedScores[1] > 0 {
		confidence = (sortedScores[1] - sortedScores[0]) / sortedScores[1]
	}

	logger.Log.Debug().
		Int("bestIndex", bestIndex).
		Float64("bestScore", bestScore).
		Float64("confidence", confidence).
		Interface("scores", scores).
		Msg("image captcha: comparison results")

	// Кликаем если есть уверенность
	if bestIndex >= 0 && confidence > 0.05 {
		var clicked bool
		clickSelector := imgInfo.CandidateSelectors[bestIndex]
		err = chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf(`
				(function() {
					const img = document.querySelector('%s');
					if (img && img.onclick) {
						img.click();
						return true;
					}
					return false;
				})()
			`, clickSelector), &clicked),
		)
		if err != nil {
			return false, err
		}

		if clicked {
			logger.Log.Debug().Int("index", bestIndex).Float64("confidence", confidence).Msg("image captcha solved")
			return true, nil
		}
	}

	logger.Log.Debug().Float64("confidence", confidence).Msg("image captcha: low confidence, not clicking")
	return false, nil
}

// compareImages сравнивает два PNG скриншота через декодирование и гистограмму
func compareImages(img1Data, img2Data []byte) float64 {
	if len(img1Data) == 0 || len(img2Data) == 0 {
		return 1000000
	}

	img1, err := png.Decode(bytes.NewReader(img1Data))
	if err != nil {
		return 1000000
	}

	img2, err := png.Decode(bytes.NewReader(img2Data))
	if err != nil {
		return 1000000
	}

	// Вычисляем гистограммы цветов (4 бина на канал = 64 комбинации)
	hist1 := computeHistogram(img1)
	hist2 := computeHistogram(img2)

	// Chi-Square distance между гистограммами
	return chiSquareDistance(hist1, hist2)
}

// computeHistogram создаёт цветовую гистограмму изображения
func computeHistogram(img image.Image) []float64 {
	const bins = 4
	histogram := make([]float64, bins*bins*bins)
	binSize := 256 / bins

	bounds := img.Bounds()
	total := 0

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			// RGBA возвращает 16-bit значения, конвертируем в 8-bit
			r8 := int(r >> 8)
			g8 := int(g >> 8)
			b8 := int(b >> 8)

			rBin := r8 / binSize
			gBin := g8 / binSize
			bBin := b8 / binSize

			if rBin >= bins {
				rBin = bins - 1
			}
			if gBin >= bins {
				gBin = bins - 1
			}
			if bBin >= bins {
				bBin = bins - 1
			}

			idx := rBin*bins*bins + gBin*bins + bBin
			histogram[idx]++
			total++
		}
	}

	// Нормализуем
	if total > 0 {
		for i := range histogram {
			histogram[i] /= float64(total)
		}
	}

	return histogram
}

// chiSquareDistance вычисляет Chi-Square расстояние между гистограммами
func chiSquareDistance(h1, h2 []float64) float64 {
	if len(h1) != len(h2) {
		return 1000000
	}

	dist := 0.0
	for i := range h1 {
		sum := h1[i] + h2[i]
		if sum > 0 {
			diff := h1[i] - h2[i]
			dist += (diff * diff) / sum
		}
	}
	return dist
}

func sortFloat64s(a []float64) {
	for i := 0; i < len(a)-1; i++ {
		for j := i + 1; j < len(a); j++ {
			if a[j] < a[i] {
				a[i], a[j] = a[j], a[i]
			}
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (s *PirateSolver) IsPirateCaptcha(html string) bool {
	return pirateCaptchaDetected(html)
}

func pirateCaptchaDetected(html string) bool {
	if len(html) == 0 {
		return false
	}

	// Button captcha: "Я не робот" with onclick
	hasButton := containsSubstring(html, "Я не робот") && containsSubstring(html, "onclick=")

	// Confirm text captcha
	hasConfirmText := containsSubstring(html, "Подтвердите") &&
		(containsSubstring(html, "человек") || containsSubstring(html, "робот"))

	// Color captcha
	hasColorCaptcha := containsSubstring(html, "похожий цвет") ||
		containsSubstring(html, "нажмите на похожий цвет")

	// Image captcha
	hasImageCaptcha := containsSubstring(html, "похожую картинку") ||
		containsSubstring(html, "нажмите на похожую картинку")

	return hasButton || hasConfirmText || hasColorCaptcha || hasImageCaptcha
}

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) != -1
}

func findSubstring(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(s) < len(substr) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
