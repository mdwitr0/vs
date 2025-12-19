package detector

import (
	"strings"
)

type CMSDetector struct{}

func NewCMSDetector() *CMSDetector {
	return &CMSDetector{}
}

type CMSResult struct {
	CMS        CMS
	Version    string
	Confidence float64
	Markers    []Marker
}

func (d *CMSDetector) Detect(html string, headers map[string]string) CMSResult {
	if result := d.detectCinemaPress(html, headers); result.CMS != CMSUnknown {
		return result
	}

	if result := d.detectDLE(html, headers); result.CMS != CMSUnknown {
		return result
	}

	if result := d.detectWordPress(html, headers); result.CMS != CMSUnknown {
		return result
	}

	if result := d.detectUCoz(html, headers); result.CMS != CMSUnknown {
		return result
	}

	return CMSResult{
		CMS:        CMSCustom,
		Confidence: 0.3,
	}
}

func (d *CMSDetector) detectCinemaPress(html string, headers map[string]string) CMSResult {
	result := CMSResult{CMS: CMSUnknown}
	var markers []Marker

	if poweredBy, ok := headers["x-powered-by"]; ok {
		if strings.Contains(poweredBy, cpPoweredByValue) {
			markers = append(markers, Marker{
				Type:       "header",
				Name:       "x-powered-by",
				Value:      poweredBy,
				Confidence: 1.0,
			})
		}
	}

	if cpVerPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "script",
			Name:       "CP_VER",
			Confidence: 0.9,
		})
	}

	if cpConfigPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "script",
			Name:       "CP_CONFIG_MD5",
			Confidence: 0.9,
		})
	}

	if cpMovieURLPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "url",
			Name:       "cp_movie_url",
			Confidence: 0.8,
		})
	}

	if len(markers) > 0 {
		result.CMS = CMSCinemaPress
		result.Markers = markers
		result.Confidence = d.calculateConfidence(markers)
	}

	return result
}

func (d *CMSDetector) detectDLE(html string, headers map[string]string) CMSResult {
	result := CMSResult{CMS: CMSUnknown}
	var markers []Marker

	if dleGeneratorPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "meta",
			Name:       "generator",
			Value:      "DataLife Engine",
			Confidence: 1.0,
		})
	}

	if dleRootPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "script",
			Name:       "dle_root",
			Confidence: 0.95,
		})
	}

	if dleLoginHashPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "script",
			Name:       "dle_login_hash",
			Confidence: 0.95,
		})
	}

	if dleSkinPattern.MatchString(html) {
		matches := dleSkinPattern.FindStringSubmatch(html)
		if len(matches) > 1 {
			markers = append(markers, Marker{
				Type:       "script",
				Name:       "dle_skin",
				Value:      matches[1],
				Confidence: 0.9,
			})
		}
	}

	if dleEnginePattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "path",
			Name:       "engine_path",
			Confidence: 0.85,
		})
	}

	if dleCommentsPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "html",
			Name:       "dle_comments",
			Confidence: 0.8,
		})
	}

	if len(markers) > 0 {
		result.CMS = CMSDLE
		result.Markers = markers
		result.Confidence = d.calculateConfidence(markers)
	}

	return result
}

func (d *CMSDetector) detectWordPress(html string, headers map[string]string) CMSResult {
	result := CMSResult{CMS: CMSUnknown}
	var markers []Marker

	if matches := wpGeneratorPattern.FindStringSubmatch(html); len(matches) > 0 {
		marker := Marker{
			Type:       "meta",
			Name:       "generator",
			Value:      "WordPress",
			Confidence: 1.0,
		}
		if len(matches) > 1 && matches[1] != "" {
			result.Version = matches[1]
			marker.Value = "WordPress " + matches[1]
		}
		markers = append(markers, marker)
	}

	if link, ok := headers["link"]; ok {
		if wpAPILinkPattern.MatchString(link) {
			markers = append(markers, Marker{
				Type:       "header",
				Name:       "wp_api_link",
				Confidence: 0.95,
			})
		}
	}

	if wpContentPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "path",
			Name:       "wp_content",
			Confidence: 0.9,
		})
	}

	if wpIncludesPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "path",
			Name:       "wp_includes",
			Confidence: 0.9,
		})
	}

	if wpAdminAjaxPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "path",
			Name:       "wp_admin_ajax",
			Confidence: 0.95,
		})
	}

	if wpBlockPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "css",
			Name:       "wp_block_class",
			Confidence: 0.8,
		})
	}

	if len(markers) > 0 {
		result.CMS = CMSWordPress
		result.Markers = markers
		result.Confidence = d.calculateConfidence(markers)
	}

	return result
}

func (d *CMSDetector) detectUCoz(html string, headers map[string]string) CMSResult {
	result := CMSResult{CMS: CMSUnknown}
	var markers []Marker

	if ucozWindowPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "script",
			Name:       "window_ucoz",
			Confidence: 1.0,
		})
	}

	if ucozFunctionsPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "script",
			Name:       "ucoz_functions",
			Confidence: 0.9,
		})
	}

	if ucozHostPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "url",
			Name:       "ucoz_host",
			Confidence: 0.85,
		})
	}

	if len(markers) > 0 {
		result.CMS = CMSUCoz
		result.Markers = markers
		result.Confidence = d.calculateConfidence(markers)
	}

	return result
}

func (d *CMSDetector) calculateConfidence(markers []Marker) float64 {
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
