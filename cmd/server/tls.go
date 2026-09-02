package main

import (
	"bufio"
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Elexation/onyx/internal/domain"
	"github.com/Elexation/onyx/internal/service"
)

func resolveTLS(settings *service.SettingsService, configDir string) (enabled bool, certFile, keyFile string, envOverrides map[string]string) {
	envOverrides = map[string]string{}

	if v := os.Getenv("ONYX_HTTPS"); v != "" {
		enabled = v == "true"
		envOverrides[domain.SettingServerTLSEnabled] = "ONYX_HTTPS"
	} else if v := os.Getenv("ONYX_TLS"); v != "" {
		slog.Warn("ONYX_TLS is deprecated, use ONYX_HTTPS instead")
		enabled = v == "true"
		envOverrides[domain.SettingServerTLSEnabled] = "ONYX_HTTPS"
	} else {
		v, err := settings.Get(domain.SettingServerTLSEnabled)
		if err == nil {
			enabled = v == "true"
		}
	}

	if !enabled {
		return false, "", "", envOverrides
	}

	envCert := os.Getenv("ONYX_TLS_CERT")
	envKey := os.Getenv("ONYX_TLS_KEY")
	if envCert != "" && envKey != "" {
		return true, envCert, envKey, envOverrides
	}
	if (envCert != "") != (envKey != "") {
		fmt.Fprintln(os.Stderr, "ONYX_TLS_CERT and ONYX_TLS_KEY must both be set or both be unset")
		os.Exit(1)
	}

	certFile = filepath.Join(configDir, "tls.crt")
	keyFile = filepath.Join(configDir, "tls.key")
	return true, certFile, keyFile, envOverrides
}

func ensureSelfSignedCert(certFile, keyFile string) error {
	_, certErr := os.Stat(certFile)
	_, keyErr := os.Stat(keyFile)
	if certErr == nil && keyErr == nil {
		return nil
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("generate key: %w", err)
	}

	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return fmt.Errorf("generate serial: %w", err)
	}

	hostname, _ := os.Hostname()

	template := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "Onyx"},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	}

	if hostname != "" {
		template.DNSNames = append(template.DNSNames, hostname)
	}

	if addrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range addrs {
			if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
				template.IPAddresses = append(template.IPAddresses, ipNet.IP)
			}
		}
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		return fmt.Errorf("create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return fmt.Errorf("marshal key: %w", err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})

	if err := os.WriteFile(certFile, certPEM, 0644); err != nil {
		return fmt.Errorf("write cert: %w", err)
	}
	if err := os.WriteFile(keyFile, keyPEM, 0600); err != nil {
		return fmt.Errorf("write key: %w", err)
	}

	return nil
}

func certFingerprint(certFile string) string {
	data, err := os.ReadFile(certFile)
	if err != nil {
		return "unknown"
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return "unknown"
	}
	sum := sha256.Sum256(block.Bytes)
	return fmt.Sprintf("SHA256:%X", sum)
}

func buildTLSConfig(certFile, keyFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load TLS cert/key: %w", err)
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		// Advertise HTTP/2 via ALPN — the custom TLS listener (srv.Serve, not
		// ServeTLS) skips Go's automatic h2 wiring; non-h2 clients fall back to http/1.1.
		NextProtos:   []string{"h2", "http/1.1"},
		Certificates: []tls.Certificate{cert},
	}, nil
}

type tlsRedirectListener struct {
	net.Listener
	tlsCfg          *tls.Config
	port            string
	canonicalDomain string
	trustedProxy    bool

	conns     chan net.Conn
	errs      chan error
	closeOnce sync.Once
	closed    chan struct{}
}

func newTLSRedirectListener(ln net.Listener, cfg *tls.Config, port, canonicalDomain string, trustedProxy bool) net.Listener {
	l := &tlsRedirectListener{
		Listener:        ln,
		tlsCfg:          cfg,
		port:            port,
		canonicalDomain: canonicalDomain,
		trustedProxy:    trustedProxy,
		conns:           make(chan net.Conn),
		errs:            make(chan error, 1),
		closed:          make(chan struct{}),
	}
	go l.acceptLoop()
	return l
}

// acceptLoop accepts raw connections and hands each to its own dispatch
// goroutine, so one stalled client cannot head-of-line block the accept
// path for everyone else.
func (l *tlsRedirectListener) acceptLoop() {
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			select {
			case l.errs <- err:
			case <-l.closed:
				return
			}
			// http.Server.Serve backs off and retries on temporary errors
			// (e.g. EMFILE); keep accepting so its retry finds a live loop.
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				continue
			}
			return
		}
		go l.dispatch(conn)
	}
}

// dispatch reads the first byte (bounded by a 5s deadline) to decide
// between TLS (0x16 handshake record) and plaintext HTTP, then either
// queues the TLS conn for Accept or answers the HTTP redirect inline.
func (l *tlsRedirectListener) dispatch(conn net.Conn) {
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var buf [1]byte
	if _, err := io.ReadFull(conn, buf[:]); err != nil {
		conn.Close()
		return
	}
	conn.SetReadDeadline(time.Time{})

	pc := newPrefixConn(conn, buf[:])

	if buf[0] != 0x16 {
		redirectHTTP(pc, l.port, l.canonicalDomain, l.trustedProxy)
		return
	}

	select {
	case l.conns <- tls.Server(pc, l.tlsCfg):
	case <-l.closed:
		conn.Close()
	}
}

func (l *tlsRedirectListener) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case err := <-l.errs:
		return nil, err
	case <-l.closed:
		return nil, net.ErrClosed
	}
}

func (l *tlsRedirectListener) Close() error {
	l.closeOnce.Do(func() { close(l.closed) })
	return l.Listener.Close()
}

func redirectHTTP(conn net.Conn, port, canonicalDomain string, trustedProxy bool) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))

	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		return
	}

	host := req.Host
	if host == "" {
		return
	}
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if canonicalDomain != "" {
		host = canonicalDomain
	}

	var target string
	if trustedProxy || port == "443" {
		target = "https://" + host + req.URL.RequestURI()
	} else {
		target = "https://" + net.JoinHostPort(host, port) + req.URL.RequestURI()
	}
	body := "Redirecting to " + target + "\n"
	fmt.Fprintf(conn, "HTTP/1.1 301 Moved Permanently\r\nLocation: %s\r\nContent-Type: text/plain\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", target, len(body), body)
}

type prefixConn struct {
	net.Conn
	r io.Reader
}

func newPrefixConn(c net.Conn, prefix []byte) *prefixConn {
	buf := make([]byte, len(prefix))
	copy(buf, prefix)
	return &prefixConn{
		Conn: c,
		r:    io.MultiReader(bytes.NewReader(buf), c),
	}
}

func (c *prefixConn) Read(b []byte) (int, error) {
	return c.r.Read(b)
}
