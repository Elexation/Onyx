package main

import (
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Verifies the real TLS wiring (buildTLSConfig + newTLSRedirectListener + srv.Serve,
// with srv.TLSConfig nil exactly as main.go leaves it) negotiates HTTP/2 via ALPN.
// Fails to ProtoMajor 1 if buildTLSConfig stops advertising "h2" in NextProtos.
func TestTLSListenerNegotiatesHTTP2(t *testing.T) {
	dir := t.TempDir()
	certFile := filepath.Join(dir, "tls.crt")
	keyFile := filepath.Join(dir, "tls.key")
	if err := ensureSelfSignedCert(certFile, keyFile); err != nil {
		t.Fatalf("generate cert: %v", err)
	}
	tlsCfg, err := buildTLSConfig(certFile, keyFile)
	if err != nil {
		t.Fatalf("build tls config: %v", err)
	}

	// Trust the generated cert (SANs cover 127.0.0.1/localhost) so the client
	// verifies it rather than skipping verification.
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		t.Fatalf("read cert: %v", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(certPEM) {
		t.Fatal("add cert to pool")
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	srv := &http.Server{
		Handler:           http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, r.Proto) }),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go srv.Serve(newTLSRedirectListener(ln, tlsCfg, "0", "", false))
	defer srv.Close()

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			ForceAttemptHTTP2: true,
			TLSClientConfig:   &tls.Config{RootCAs: pool, NextProtos: []string{"h2", "http/1.1"}},
		},
	}

	var resp *http.Response
	for i := 0; i < 20; i++ {
		resp, err = client.Get("https://" + ln.Addr().String() + "/")
		if err == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.ProtoMajor != 2 {
		t.Fatalf("expected HTTP/2, got %s", resp.Proto)
	}
}
