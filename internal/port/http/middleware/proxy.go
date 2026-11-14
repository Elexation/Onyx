package middleware

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
)

// DefaultTrustedProxies covers loopback, private LANs, Docker bridges and Tailscale (100.64.0.0/10, fd7a::/48)
var DefaultTrustedProxies = []netip.Prefix{
	netip.MustParsePrefix("127.0.0.0/8"),
	netip.MustParsePrefix("::1/128"),
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("100.64.0.0/10"),
	netip.MustParsePrefix("fc00::/7"),
}

var forwardedHeaders = []string{"X-Real-IP", "X-Forwarded-For", "X-Forwarded-Proto", "X-Forwarded-Host", "Forwarded"}

// Cap distinct peers warned about, so a scan from many addresses can't flood the log
const maxUntrustedPeerWarnings = 32

// ParseTrustedProxies reads a comma-separated list of IPs and CIDRs. Error if empty or malformed.
func ParseTrustedProxies(s string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "/") {
			p, err := netip.ParsePrefix(part)
			if err != nil {
				return nil, fmt.Errorf("invalid CIDR %q: %w", part, err)
			}
			out = append(out, p.Masked())
			continue
		}
		a, err := netip.ParseAddr(part)
		if err != nil {
			return nil, fmt.Errorf("invalid IP %q: %w", part, err)
		}
		a = a.Unmap().WithZone("")
		out = append(out, netip.PrefixFrom(a, a.BitLen()))
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no addresses given")
	}
	return out, nil
}

// TrustedProxyFilter drops forwarded headers from untrusted peers; wrap the whole server, uploads included
func TrustedProxyFilter(trusted []netip.Prefix) func(http.Handler) http.Handler {
	var mu sync.Mutex
	warned := make(map[string]bool)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := peerHost(r.RemoteAddr)
			if peerTrusted(host, trusted) || !hasForwardedHeaders(r) {
				next.ServeHTTP(w, r)
				return
			}
			mu.Lock()
			if !warned[host] && len(warned) < maxUntrustedPeerWarnings {
				warned[host] = true
				slog.Warn("ignoring forwarded headers from an address that is not a trusted proxy; if this is your reverse proxy, add it to ONYX_TRUSTED_PROXIES", "peer", host)
			}
			mu.Unlock()
			r = r.Clone(r.Context())
			for _, h := range forwardedHeaders {
				r.Header.Del(h)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func hasForwardedHeaders(r *http.Request) bool {
	for _, h := range forwardedHeaders {
		if len(r.Header.Values(h)) > 0 {
			return true
		}
	}
	return false
}

func peerHost(remoteAddr string) string {
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host
	}
	return remoteAddr
}

func peerTrusted(host string, trusted []netip.Prefix) bool {
	a, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	a = a.Unmap().WithZone("")
	for _, p := range trusted {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// ClientIP returns the source IP of the request.
//
// When trustedProxy is true, X-Real-IP is preferred; otherwise the right-most
// X-Forwarded-For entry is used. Standard proxies append the real client IP to
// X-Forwarded-For, so earlier entries are client-controllable and must not be
// trusted for rate limiting. When trustedProxy is false, forwarded headers are
// ignored and RemoteAddr is used. TrustedProxyFilter strips them from untrusted peers first.
func ClientIP(r *http.Request, trustedProxy bool) string {
	if trustedProxy {
		if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" && net.ParseIP(ip) != nil {
			return ip
		}
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			var candidate string
			if idx := strings.LastIndex(xff, ","); idx >= 0 {
				candidate = strings.TrimSpace(xff[idx+1:])
			} else {
				candidate = strings.TrimSpace(xff)
			}
			if candidate != "" && net.ParseIP(candidate) != nil {
				return candidate
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// IsHTTPS reports whether the request originated over HTTPS. X-Forwarded-Proto
// is honored only when trustedProxy is true.
func IsHTTPS(r *http.Request, trustedProxy bool) bool {
	if r.TLS != nil {
		return true
	}
	if trustedProxy && r.Header.Get("X-Forwarded-Proto") == "https" {
		return true
	}
	return false
}

// ClientKey returns the per-client key every limiter counts against. IPv6 is
// masked to /64 so rotating addresses within an attacker-controlled prefix
// cannot dodge a limit; IPv4 keys on the full address. Every call site for one
// limiter must use it, or a success reset or slot release misses its entry.
func ClientKey(r *http.Request, trustedProxy bool) string {
	ip := ClientIP(r, trustedProxy)
	if ip == "" {
		ip = r.RemoteAddr
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ip
	}
	if v4 := parsed.To4(); v4 != nil {
		return v4.String()
	}
	return parsed.Mask(net.CIDRMask(64, 128)).String()
}
