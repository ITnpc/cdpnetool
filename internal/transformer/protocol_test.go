package transformer_test

import (
	"testing"

	"cdpnetool/internal/transformer"
)

func TestParseCookies(t *testing.T) {
	tests := []struct {
		name      string
		cookieStr string
		wantLen   int
	}{
		{"单个cookie", "name=test", 1},
		{"多个cookie", "name=test; age=18", 2},
		{"空字符串", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := transformer.ParseCookies(tt.cookieStr)
			if len(got) != tt.wantLen {
				t.Errorf("got len %v, want len %v", len(got), tt.wantLen)
			}
		})
	}
}

func TestBuildCookieString(t *testing.T) {
	tests := []struct {
		name    string
		cookies map[string]string
		want    string
	}{
		{"空map", map[string]string{}, ""},
		{"单个cookie", map[string]string{"name": "test"}, "name=test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := transformer.BuildCookieString(tt.cookies)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsBinaryContentType(t *testing.T) {
	tests := []struct {
		contentType string
		want        bool
	}{
		{"image/png", true},
		{"video/mp4", true},
		{"audio/mpeg", true},
		{"text/html", false},
		{"application/json", false},
		{"application/octet-stream", true},
		{"IMAGE/PNG", true},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			got := transformer.IsBinaryContentType(tt.contentType)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
