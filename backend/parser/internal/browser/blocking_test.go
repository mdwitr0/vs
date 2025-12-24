package browser

import "testing"

func TestIsPirateCaptcha_AntibotChallenge(t *testing.T) {
	testCases := []struct {
		name     string
		html     string
		expected bool
	}{
		{
			name: "peel.js with loading text",
			html: `<!DOCTYPE html>
<html>
<head>
<script src="/peel.js"></script>
</head>
<body>
<div>Идёт загрузка...</div>
</body>
</html>`,
			expected: true,
		},
		{
			name: "antibot with loading text",
			html: `<!DOCTYPE html>
<html>
<head>
<script src="/antibot8.js"></script>
</head>
<body>
<div>Идёт загрузка</div>
</body>
</html>`,
			expected: true,
		},
		{
			name:     "simple blocked page - no captcha",
			html:     `<html><body>Sorry, your request has been denied.</body></html>`,
			expected: false,
		},
		{
			name: "button captcha",
			html: `<html><body>
<button onclick="solve()">Я не робот</button>
</body></html>`,
			expected: true,
		},
		{
			name:     "normal page",
			html:     `<html><head><title>Normal</title></head><body>Content</body></html>`,
			expected: false,
		},
		{
			name:     "peel.js without loading - not captcha yet",
			html:     `<html><script src="/peel.js"></script><body>Other text</body></html>`,
			expected: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := IsPirateCaptcha(tc.html)
			if result != tc.expected {
				t.Errorf("IsPirateCaptcha() = %v, expected %v", result, tc.expected)
			}
		})
	}
}

func TestDetectBlocking_CaptchaBeforeHTTPStatus(t *testing.T) {
	// narko-tv.com returns HTTP 403 with antibot JS challenge
	// Should detect as captcha, not just blocked
	html := `<!DOCTYPE html>
<html>
<head>
<script src="/antibot8/peel.js"></script>
</head>
<body>
<div>Идёт загрузка...</div>
</body>
</html>`

	result := DetectBlocking(html, 403)

	if !result.Blocked {
		t.Error("Expected page to be blocked")
	}
	if !result.IsCaptcha {
		t.Errorf("Expected IsCaptcha=true, got false. Reason: %s", result.Reason)
	}
}
