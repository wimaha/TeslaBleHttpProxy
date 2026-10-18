package routes

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"testing/fstest"
)

func serve(t *testing.T, req *http.Request) int {
	t.Helper()
	return serveTrusting(t, req, nil)
}

func serveTrusting(t *testing.T, req *http.Request, trustedOrigins []string) int {
	t.Helper()
	rec := httptest.NewRecorder()
	SetupRoutes(fstest.MapFS{}, fstest.MapFS{}, trustedOrigins).ServeHTTP(rec, req)
	return rec.Code
}

func commandRequest(headers map[string]string) *http.Request {
	req := httptest.NewRequest("POST", "http://proxy:8080/api/1/vehicles/VIN/command/flash_lights", nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req
}

func TestCrossOriginCommandIsRejected(t *testing.T) {
	cases := map[string]map[string]string{
		"sec-fetch-site": {"Sec-Fetch-Site": "cross-site"},
		"origin":         {"Origin": "http://evil.example"},
	}
	for name, headers := range cases {
		t.Run(name, func(t *testing.T) {
			if code := serve(t, commandRequest(headers)); code != http.StatusForbidden {
				t.Errorf("status = %d, want %d", code, http.StatusForbidden)
			}
		})
	}
}

// evcc, curl and Home Assistant send neither Origin nor Sec-Fetch-Site and must keep working.
// Without a key the handler answers 503, which shows the request got past the protection.
func TestNonBrowserCommandPasses(t *testing.T) {
	if code := serve(t, commandRequest(nil)); code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", code, http.StatusServiceUnavailable)
	}
}

func TestSameOriginDashboardFormPasses(t *testing.T) {
	form := url.Values{"role": {"invalid"}}.Encode()
	req := httptest.NewRequest("POST", "http://proxy:8080/activate_key", strings.NewReader(form))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	if code := serve(t, req); code != http.StatusSeeOther {
		t.Errorf("status = %d, want %d", code, http.StatusSeeOther)
	}
}

// Key changes must not be reachable by GET, where a plain <img> on any site could trigger them.
func TestKeyRoutesRejectGet(t *testing.T) {
	for _, path := range []string{"/gen_keys?role=owner", "/remove_keys?role=owner"} {
		if code := serve(t, httptest.NewRequest("GET", path, nil)); code != http.StatusMethodNotAllowed {
			t.Errorf("GET %s: status = %d, want %d", path, code, http.StatusMethodNotAllowed)
		}
	}
}

// Behind a plain-HTTP reverse proxy that rewrites Host, browsers send no Sec-Fetch-Site and the
// Origin no longer matches Host. Dashboard forms then need the proxy's origin to be trusted.
func TestReverseProxyOriginNeedsTrust(t *testing.T) {
	request := func() *http.Request {
		form := url.Values{"role": {"invalid"}}.Encode()
		req := httptest.NewRequest("POST", "http://127.0.0.1:8080/activate_key", strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Origin", "http://nas.local")
		return req
	}

	if code := serveTrusting(t, request(), nil); code != http.StatusForbidden {
		t.Errorf("untrusted: status = %d, want %d", code, http.StatusForbidden)
	}
	if code := serveTrusting(t, request(), []string{"http://NAS.local:80/"}); code != http.StatusSeeOther {
		t.Errorf("trusted: status = %d, want %d", code, http.StatusSeeOther)
	}
	// An invalid entry is logged and skipped instead of breaking startup.
	if code := serveTrusting(t, request(), []string{"nas.local/path", "http://nas.local"}); code != http.StatusSeeOther {
		t.Errorf("with invalid entry: status = %d, want %d", code, http.StatusSeeOther)
	}
}

func TestNormalizeOrigin(t *testing.T) {
	cases := map[string]string{
		"http://nas.local":       "http://nas.local",
		"http://nas.local/":      "http://nas.local",
		"HTTP://NAS.local:80":    "http://nas.local",
		"https://nas.local:443/": "https://nas.local",
		"https://nas.local:80":   "https://nas.local:80",
		"http://nas.local:8080":  "http://nas.local:8080",
		"http://[::1]:8080":      "http://[::1]:8080",
		"http://nas.local/path":  "http://nas.local/path", // left for AddTrustedOrigin to reject
		"bogus":                  "bogus",
	}
	for in, want := range cases {
		if got := normalizeOrigin(in); got != want {
			t.Errorf("normalizeOrigin(%q) = %q, want %q", in, got, want)
		}
	}
}
