package headless

import (
	"net/http"
	"net/url"
	"strings"

	fhttp "github.com/bogdanfinn/fhttp"
)

// The session file is authoritative for the website's .Auth cookie. Preserve
// every other jar cookie and native domain/path rules, including other hosts.
type websiteSessionJar struct{ http.CookieJar }

func (j websiteSessionJar) Cookies(u *url.URL) []*http.Cookie {
	cookies := j.CookieJar.Cookies(u)
	if !strings.EqualFold(u.Hostname(), "ddclassico.com") {
		return cookies
	}
	filtered := make([]*http.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie.Name != ".Auth" {
			filtered = append(filtered, cookie)
		}
	}
	return filtered
}

type launcherSessionJar struct{ fhttp.CookieJar }

func (j launcherSessionJar) Cookies(u *url.URL) []*fhttp.Cookie {
	cookies := j.CookieJar.Cookies(u)
	if !strings.EqualFold(u.Hostname(), "ddclassico.com") {
		return cookies
	}
	filtered := make([]*fhttp.Cookie, 0, len(cookies))
	for _, cookie := range cookies {
		if cookie.Name != ".Auth" {
			filtered = append(filtered, cookie)
		}
	}
	return filtered
}
