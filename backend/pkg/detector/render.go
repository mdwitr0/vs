package detector

import (
	"strings"
)

type RenderDetector struct{}

func NewRenderDetector() *RenderDetector {
	return &RenderDetector{}
}

type RenderResult struct {
	RenderType   RenderType
	Framework    Framework
	NeedsBrowser bool
	Confidence   float64
	Markers      []Marker
}

func (d *RenderDetector) Detect(html string, contentLength int64) RenderResult {
	result := RenderResult{
		RenderType: RenderSSR,
		Framework:  FrameworkNone,
	}

	var markers []Marker

	framework, frameworkMarkers := d.detectFramework(html)
	if framework != FrameworkNone {
		result.Framework = framework
		markers = append(markers, frameworkMarkers...)
	}

	textContent := d.extractTextContent(html)
	hasContent := len(textContent) >= minTextContentLength

	hasSPAMarkers := d.hasSPAMarkers(html)

	switch {
	case framework == FrameworkNextJS || framework == FrameworkNuxt:
		if hasContent {
			result.RenderType = RenderSSR
			result.NeedsBrowser = false
		} else {
			result.RenderType = RenderCSR
			result.NeedsBrowser = true
		}

	case hasSPAMarkers && !hasContent:
		result.RenderType = RenderCSR
		result.NeedsBrowser = true
		markers = append(markers, Marker{
			Type:       "html",
			Name:       "spa_empty_container",
			Confidence: 0.9,
		})

	case hasSPAMarkers && hasContent:
		result.RenderType = RenderHybrid
		result.NeedsBrowser = false

	case !hasContent && contentLength > 50000:
		result.RenderType = RenderCSR
		result.NeedsBrowser = true
		markers = append(markers, Marker{
			Type:       "heuristic",
			Name:       "large_js_no_content",
			Confidence: 0.7,
		})

	default:
		result.RenderType = RenderSSR
		result.NeedsBrowser = false
	}

	result.Markers = markers
	result.Confidence = d.calculateConfidence(result)

	return result
}

func (d *RenderDetector) detectFramework(html string) (Framework, []Marker) {
	var markers []Marker

	if nextDataPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "script",
			Name:       "__NEXT_DATA__",
			Confidence: 1.0,
		})
		return FrameworkNextJS, markers
	}

	if nextStaticPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "path",
			Name:       "_next/static",
			Confidence: 0.95,
		})
		return FrameworkNextJS, markers
	}

	if nuxtPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "script",
			Name:       "__NUXT__",
			Confidence: 1.0,
		})
		return FrameworkNuxt, markers
	}

	if vuePattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "script",
			Name:       "vue_init",
			Confidence: 0.9,
		})
		return FrameworkVue, markers
	}

	if reactDOMPattern.MatchString(html) {
		markers = append(markers, Marker{
			Type:       "script",
			Name:       "react_dom",
			Confidence: 0.85,
		})
		return FrameworkReact, markers
	}

	return FrameworkNone, nil
}

func (d *RenderDetector) hasSPAMarkers(html string) bool {
	return reactRootPattern.MatchString(html) || vueAppPattern.MatchString(html)
}

func (d *RenderDetector) extractTextContent(html string) string {
	text := scriptTagPattern.ReplaceAllString(html, "")
	text = styleTagPattern.ReplaceAllString(text, "")
	text = htmlTagPattern.ReplaceAllString(text, " ")
	text = whitespacePattern.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

func (d *RenderDetector) calculateConfidence(result RenderResult) float64 {
	base := 0.7

	if result.Framework != FrameworkNone {
		base += 0.15
	}

	for _, m := range result.Markers {
		base += m.Confidence * 0.1
	}

	if base > 1.0 {
		base = 1.0
	}

	return base
}
