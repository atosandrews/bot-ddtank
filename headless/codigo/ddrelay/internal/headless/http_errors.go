package headless

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

func classifyHTTP(stage string, status int, header http.Header, body []byte) string {
	if strings.EqualFold(header.Get("CF-Mitigated"), "challenge") {
		return "challenge"
	}
	text := strings.ToLower(string(body))
	html := strings.Contains(text, "<html") || strings.Contains(text, "<!doctype html") || strings.Contains(strings.ToLower(header.Get("Content-Type")), "text/html")
	cloudflare := strings.Contains(strings.ToLower(header.Get("Server")), "cloudflare") || header.Get("CF-Ray") != "" || strings.Contains(text, "cloudflare")
	if html && cloudflare {
		if strings.Contains(text, "sorry, you have been blocked") || strings.Contains(text, "error 1020") || strings.Contains(text, "error code: 1020") {
			return "blocked"
		}
		if strings.Contains(text, "cf-chl-") || strings.Contains(text, "/cdn-cgi/challenge-platform") || strings.Contains(text, "<title>just a moment") {
			return "challenge"
		}
	}
	if status == http.StatusForbidden {
		return "forbidden"
	}
	if status == http.StatusUnauthorized || (stage == "website" && status >= 300 && status < 400) {
		return "session"
	}
	if status == http.StatusTooManyRequests || status >= 500 {
		return "network"
	}
	if status != http.StatusOK {
		return "protocol"
	}
	return ""
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 32); err == nil {
		if seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
		return 0
	}
	if date, err := http.ParseTime(value); err == nil && date.After(now) {
		return date.Sub(now)
	}
	return 0
}

func safeRayID(value string) string {
	if len(value) > 128 {
		return ""
	}
	for _, c := range value {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return ""
		}
	}
	return value
}
