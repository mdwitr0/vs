package detector

import (
	"strings"

	"github.com/video-analitics/backend/pkg/logger"
)

type CaptchaDetector struct{}

func NewCaptchaDetector() *CaptchaDetector {
	return &CaptchaDetector{}
}

type CaptchaResult struct {
	Type       CaptchaType
	Confidence float64
	Markers    []Marker
}

func (d *CaptchaDetector) Detect(html string, headers map[string]string) CaptchaResult {
	if cfResult := d.detectCloudflare(html, headers); cfResult.Type != CaptchaNone {
		return cfResult
	}

	if ddgResult := d.detectDDoSGuard(html, headers); ddgResult.Type != CaptchaNone {
		return ddgResult
	}

	if recaptchaResult := d.detectReCAPTCHA(html); recaptchaResult.Type != CaptchaNone {
		return recaptchaResult
	}

	if hcaptchaResult := d.detectHCaptcha(html); hcaptchaResult.Type != CaptchaNone {
		return hcaptchaResult
	}

	if dleResult := d.detectDLEAntibot(html); dleResult.Type != CaptchaNone {
		return dleResult
	}

	if ucozResult := d.detectUCozCaptcha(html); ucozResult.Type != CaptchaNone {
		return ucozResult
	}

	if pirateResult := d.detectPirateCaptcha(html); pirateResult.Type != CaptchaNone {
		return pirateResult
	}

	if genericResult := d.detectGenericCaptcha(html); genericResult.Type != CaptchaNone {
		return genericResult
	}

	return CaptchaResult{Type: CaptchaNone}
}

func (d *CaptchaDetector) detectCloudflare(html string, headers map[string]string) CaptchaResult {
	result := CaptchaResult{Type: CaptchaNone}
	var markers []Marker

	if server, ok := headers["server"]; ok {
		if strings.Contains(strings.ToLower(server), "cloudflare") {
			markers = append(markers, Marker{
				Type:       "header",
				Name:       "cf_server",
				Value:      server,
				Confidence: 0.5,
			})
		}
	}

	if _, ok := headers["cf-ray"]; ok {
		markers = append(markers, Marker{
			Type:       "header",
			Name:       "cf_ray",
			Confidence: 0.5,
		})
	}

	if cfVerificationPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "html",
			Name:       "cf_verification",
			Confidence: 0.9,
		})
	}

	if cfChallengePattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "html",
			Name:       "cf_challenge_text",
			Confidence: 0.85,
		})
	}

	hasChallenge := false
	for _, m := range markers {
		if m.Confidence >= 0.85 {
			hasChallenge = true
			break
		}
	}

	if hasChallenge {
		result.Type = CaptchaCloudflare
		result.Markers = markers
		result.Confidence = d.calculateConfidence(markers)
	}

	return result
}

func (d *CaptchaDetector) detectDDoSGuard(html string, headers map[string]string) CaptchaResult {
	result := CaptchaResult{Type: CaptchaNone}
	var markers []Marker

	if server, ok := headers["server"]; ok {
		if strings.Contains(strings.ToLower(server), "ddos-guard") {
			markers = append(markers, Marker{
				Type:       "header",
				Name:       "ddos_guard_server",
				Value:      server,
				Confidence: 0.9,
			})
		}
	}

	if ddosGuardPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "html",
			Name:       "ddos_guard_markers",
			Confidence: 0.85,
		})
	}

	if len(markers) > 0 {
		result.Type = CaptchaDDoSGuard
		result.Markers = markers
		result.Confidence = d.calculateConfidence(markers)
	}

	return result
}

func (d *CaptchaDetector) detectReCAPTCHA(html string) CaptchaResult {
	result := CaptchaResult{Type: CaptchaNone}
	var markers []Marker

	if recaptchaScriptPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "script",
			Name:       "recaptcha_api",
			Confidence: 0.95,
		})
	}

	if recaptchaClassPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "html",
			Name:       "g_recaptcha_class",
			Confidence: 0.95,
		})
	}

	if recaptchaSitekeyPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "html",
			Name:       "recaptcha_sitekey",
			Confidence: 0.9,
		})
	}

	if len(markers) > 0 {
		result.Type = CaptchaReCAPTCHA
		result.Markers = markers
		result.Confidence = d.calculateConfidence(markers)
	}

	return result
}

func (d *CaptchaDetector) detectHCaptcha(html string) CaptchaResult {
	result := CaptchaResult{Type: CaptchaNone}
	var markers []Marker

	if hcaptchaScriptPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "script",
			Name:       "hcaptcha_api",
			Confidence: 0.95,
		})
	}

	if hcaptchaClassPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "html",
			Name:       "h_captcha_class",
			Confidence: 0.95,
		})
	}

	if len(markers) > 0 {
		result.Type = CaptchaHCaptcha
		result.Markers = markers
		result.Confidence = d.calculateConfidence(markers)
	}

	return result
}

func (d *CaptchaDetector) detectDLEAntibot(html string) CaptchaResult {
	result := CaptchaResult{Type: CaptchaNone}
	var markers []Marker

	if dleAntibotPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "path",
			Name:       "dle_antibot_module",
			Confidence: 0.95,
		})
	}

	if len(markers) > 0 {
		result.Type = CaptchaDLEAntibot
		result.Markers = markers
		result.Confidence = d.calculateConfidence(markers)
	}

	return result
}

func (d *CaptchaDetector) detectUCozCaptcha(html string) CaptchaResult {
	result := CaptchaResult{Type: CaptchaNone}
	var markers []Marker

	if ucozCaptchaPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "path",
			Name:       "ucoz_secure",
			Confidence: 0.9,
		})
	}

	if len(markers) > 0 {
		result.Type = CaptchaUCoz
		result.Markers = markers
		result.Confidence = d.calculateConfidence(markers)
	}

	return result
}

func (d *CaptchaDetector) detectPirateCaptcha(html string) CaptchaResult {
	result := CaptchaResult{Type: CaptchaNone}
	var markers []Marker
	log := logger.Log

	buttonMatch := pirateCaptchaButtonPattern.MatchString(html)
	textMatch := pirateCaptchaTextPattern.MatchString(html)
	styleMatch := pirateCaptchaStylePattern.MatchString(html)

	log.Debug().
		Bool("button_match", buttonMatch).
		Bool("text_match", textMatch).
		Bool("style_match", styleMatch).
		Int("html_len", len(html)).
		Msg("pirate captcha detection check")

	if buttonMatch {
		markers = append(markers, Marker{
			Type:       "html",
			Name:       "pirate_button",
			Confidence: 0.7,
		})
	}

	if textMatch {
		markers = append(markers, Marker{
			Type:       "html",
			Name:       "pirate_confirm_text",
			Confidence: 0.6,
		})
	}

	if styleMatch {
		markers = append(markers, Marker{
			Type:       "html",
			Name:       "pirate_hidden_styles",
			Confidence: 0.8,
		})
	}

	if len(markers) >= 2 {
		result.Type = CaptchaPirate
		result.Markers = markers
		result.Confidence = d.calculateConfidence(markers)
		log.Info().Int("markers", len(markers)).Msg("pirate captcha detected")
	}

	return result
}

func (d *CaptchaDetector) detectGenericCaptcha(html string) CaptchaResult {
	result := CaptchaResult{Type: CaptchaNone}
	var markers []Marker

	if genericCaptchaPattern.MatchString(html) {
		lowerHTML := strings.ToLower(html)
		if strings.Contains(lowerHTML, "captcha") &&
			(strings.Contains(lowerHTML, "input") || strings.Contains(lowerHTML, "img")) {
			markers = append(markers, Marker{
				Type:       "html",
				Name:       "generic_captcha",
				Confidence: 0.6,
			})
		}
	}

	if len(markers) > 0 {
		result.Type = CaptchaCustom
		result.Markers = markers
		result.Confidence = d.calculateConfidence(markers)
	}

	return result
}

func (d *CaptchaDetector) calculateConfidence(markers []Marker) float64 {
	if len(markers) == 0 {
		return 0
	}

	maxConf := 0.0
	for _, m := range markers {
		if m.Confidence > maxConf {
			maxConf = m.Confidence
		}
	}

	bonus := float64(len(markers)-1) * 0.02
	if bonus > 0.1 {
		bonus = 0.1
	}

	total := maxConf + bonus
	if total > 1.0 {
		total = 1.0
	}

	return total
}
