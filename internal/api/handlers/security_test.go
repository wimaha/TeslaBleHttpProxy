package handlers_test

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/wimaha/TeslaBleHttpProxy/internal/ble/fakecar"
)

// raw sends a request through the proxy and returns status and body without decoding JSON,
// because the cross-origin protection answers with plain text.
func (p *proxy) raw(method, path, body string, headers map[string]string) (int, string) {
	p.t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, p.server.URL+path, reader)
	if err != nil {
		p.t.Fatal(err)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		p.t.Fatal(err)
	}
	defer res.Body.Close()
	b, err := io.ReadAll(res.Body)
	if err != nil {
		p.t.Fatal(err)
	}
	return res.StatusCode, string(b)
}

// A page on another site that submits a form to the proxy must not unlock the car.
func TestHTTPCrossSiteUnlockNeverReachesCar(t *testing.T) {
	cases := map[string]map[string]string{
		"sec-fetch-site": {"Sec-Fetch-Site": "cross-site"},
		"origin":         {"Origin": "http://evil.example"},
	}
	for name, headers := range cases {
		t.Run(name, func(t *testing.T) {
			vin := newVin()
			p := startProxy(t, vin)

			code, _ := p.raw("POST", "/api/1/vehicles/"+vin+"/command/door_unlock?wait=true", "", headers)

			if code != http.StatusForbidden {
				t.Errorf("status = %d, want %d", code, http.StatusForbidden)
			}
			if calls := p.car.Calls(); len(calls) != 0 {
				t.Errorf("car was contacted: %v", p.car.Methods())
			}
			p.car.Do(func(c *fakecar.Car) {
				if !c.Locked {
					t.Error("car is unlocked")
				}
			})
		})
	}
}

// evcc, curl and Home Assistant send neither Origin nor Sec-Fetch-Site and keep working.
func TestHTTPNonBrowserUnlockReachesCar(t *testing.T) {
	vin := newVin()
	p := startProxy(t, vin)

	code, resp := p.do("POST", "/api/1/vehicles/"+vin+"/command/door_unlock?wait=true", "")

	wantStatus(t, code, resp, http.StatusOK)
	p.car.Do(func(c *fakecar.Car) {
		if c.Locked {
			t.Error("car is still locked")
		}
	})
}

// Key changes must not be reachable by GET, where a plain <img> on any site could trigger them.
func TestHTTPKeyRoutesRejectGet(t *testing.T) {
	p := startProxy(t, newVin())
	for _, path := range []string{"/gen_keys?role=owner", "/remove_keys?role=owner"} {
		if code, _ := p.raw("GET", path, "", nil); code != http.StatusMethodNotAllowed {
			t.Errorf("GET %s: status = %d, want %d", path, code, http.StatusMethodNotAllowed)
		}
	}
}

// Form input from send_key ends up in a dashboard message, which must render escaped.
func TestHTTPDashboardEscapesSendKeyInput(t *testing.T) {
	p := startProxy(t, newVin())
	const script = `<script>alert(1)</script>`
	form := url.Values{"VIN": {script}, "role": {script}}.Encode()

	code, _ := p.raw("POST", "/send_key", form, map[string]string{
		"Content-Type":   "application/x-www-form-urlencoded",
		"Sec-Fetch-Site": "same-origin",
	})
	if code != http.StatusSeeOther {
		t.Fatalf("send_key status = %d, want %d", code, http.StatusSeeOther)
	}

	code, body := p.raw("GET", "/dashboard", "", nil)
	if code != http.StatusOK {
		t.Fatalf("dashboard status = %d", code)
	}
	if strings.Contains(body, script) {
		t.Fatal("input rendered unescaped")
	}
	if !strings.Contains(body, "&lt;script&gt;alert(1)&lt;/script&gt;") {
		t.Fatalf("escaped input not found in dashboard")
	}
}
