package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
)

func TestParseTrustedProxies(t *testing.T) {
	got, err := ParseTrustedProxies(" 10.1.2.3 , 172.18.0.0/16,fd7a:115c:a1e0::/48,::ffff:192.168.1.9,,10.9.9.9/8")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"10.1.2.3/32", "172.18.0.0/16", "fd7a:115c:a1e0::/48", "192.168.1.9/32", "10.0.0.0/8"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i].String() != want[i] {
			t.Errorf("entry %d = %s, want %s", i, got[i], want[i])
		}
	}
	for _, bad := range []string{"", " , ", "10.0.0.300", "10.0.0.0/33", "proxy.local"} {
		if _, err := ParseTrustedProxies(bad); err == nil {
			t.Errorf("ParseTrustedProxies(%q) succeeded, want error", bad)
		}
	}
}

func TestTrustedProxyFilter(t *testing.T) {
	cases := []struct {
		name    string
		remote  string
		trusted bool
	}{
		{"docker bridge", "172.18.0.5:40000", true},
		{"loopback", "127.0.0.1:40000", true},
		{"lan", "192.168.1.20:40000", true},
		{"tailscale v4", "100.101.102.103:40000", true},
		{"tailscale v6", "[fd7a:115c:a1e0::1]:40000", true},
		{"mapped v4 lan", "[::ffff:10.0.0.7]:40000", true},
		{"public v4", "203.0.113.9:40000", false},
		{"public v6", "[2001:db8::1]:40000", false},
		{"unparseable", "garbage", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var seenIP string
			var seenHTTPS bool
			var seenHost, seenFwd string
			h := TrustedProxyFilter(DefaultTrustedProxies)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seenIP = ClientIP(r, true)
				seenHTTPS = IsHTTPS(r, true)
				seenHost = r.Header.Get("X-Forwarded-Host")
				seenFwd = r.Header.Get("Forwarded")
			}))
			req := httptest.NewRequest(http.MethodGet, "http://onyx/api/health", nil)
			req.RemoteAddr = tc.remote
			req.Header.Set("X-Real-IP", "198.51.100.77")
			req.Header.Set("X-Forwarded-For", "198.51.100.77")
			req.Header.Set("X-Forwarded-Proto", "https")
			req.Header.Set("X-Forwarded-Host", "evil.example")
			req.Header.Set("Forwarded", "for=198.51.100.77;proto=https")
			h.ServeHTTP(httptest.NewRecorder(), req)

			if tc.trusted {
				if seenIP != "198.51.100.77" || !seenHTTPS || seenHost != "evil.example" || seenFwd == "" {
					t.Fatalf("trusted peer lost headers: ip=%q https=%v host=%q fwd=%q", seenIP, seenHTTPS, seenHost, seenFwd)
				}
				return
			}
			if seenIP == "198.51.100.77" || seenHTTPS || seenHost != "" || seenFwd != "" {
				t.Fatalf("untrusted peer headers survived: ip=%q https=%v host=%q fwd=%q", seenIP, seenHTTPS, seenHost, seenFwd)
			}
			if req.Header.Get("X-Real-IP") == "" {
				t.Fatal("filter modified the caller's request instead of a clone")
			}
		})
	}
}

// A client reaching Onyx directly must not dodge login lockout by rotating X-Real-IP
func TestTrustedProxyFilterStopsLockoutBypass(t *testing.T) {
	rl := NewRateLimiter(true)
	h := TrustedProxyFilter(DefaultTrustedProxies)(rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})))
	var last int
	for i := 0; i < 6; i++ {
		req := httptest.NewRequest(http.MethodPost, "http://onyx/api/auth/login", nil)
		req.RemoteAddr = "203.0.113.9:40000"
		req.Header.Set("X-Real-IP", fmt.Sprintf("198.51.100.%d", i+1))
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		last = rec.Code
	}
	if last != http.StatusTooManyRequests {
		t.Fatalf("6th attempt status = %d, want 429", last)
	}
}

func TestTrustedProxyFilterCustomList(t *testing.T) {
	nets := []netip.Prefix{netip.MustParsePrefix("10.27.27.8/32")}
	var seen string
	h := TrustedProxyFilter(nets)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = ClientIP(r, true)
	}))
	for remote, want := range map[string]string{"10.27.27.8:1": "198.51.100.1", "10.27.27.9:1": "10.27.27.9"} {
		req := httptest.NewRequest(http.MethodGet, "http://onyx/", nil)
		req.RemoteAddr = remote
		req.Header.Set("X-Real-IP", "198.51.100.1")
		h.ServeHTTP(httptest.NewRecorder(), req)
		if seen != want {
			t.Errorf("peer %s: ClientIP = %q, want %q", remote, seen, want)
		}
	}
}
