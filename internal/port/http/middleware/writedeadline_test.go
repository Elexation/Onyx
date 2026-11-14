package middleware

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// streamHandler writes total bytes in 32 KiB chunks and reports how it ended
func streamHandler(total int, setup func(w http.ResponseWriter), done chan<- error) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if setup != nil {
			setup(w)
		}
		buf := make([]byte, 32<<10)
		for sent := 0; sent < total; sent += len(buf) {
			if _, err := w.Write(buf); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	})
}

func dialAndRequest(t *testing.T, srv *httptest.Server) net.Conn {
	t.Helper()
	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: onyx\r\n\r\n")
	return conn
}

func TestWriteDeadlineDropsStalledReader(t *testing.T) {
	done := make(chan error, 1)
	// 256 MiB is far past any socket buffer, so writes must block once the client stops reading
	srv := httptest.NewServer(WriteDeadline(200 * time.Millisecond)(streamHandler(256<<20, nil, done)))
	defer srv.Close()
	dialAndRequest(t, srv)

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("handler finished writing to a client that read nothing")
		}
	case <-time.After(15 * time.Second):
		t.Fatal("handler still blocked writing to a stalled client")
	}
}

// The deadline is per write: a slow client that keeps reading must get the whole body
func TestWriteDeadlineKeepsSlowReader(t *testing.T) {
	done := make(chan error, 1)
	const total = 8 << 20
	srv := httptest.NewServer(WriteDeadline(300 * time.Millisecond)(streamHandler(total, nil, done)))
	defer srv.Close()
	conn := dialAndRequest(t, srv)

	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	var got int64
	for {
		n, err := io.CopyN(io.Discard, resp.Body, 512<<10)
		got += n
		if err != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := <-done; err != nil || got != total {
		t.Fatalf("slow reader got %d of %d bytes over %v, handler err %v", got, total, time.Since(start), err)
	}
	if time.Since(start) < time.Second {
		t.Fatalf("read finished in %v, too fast to prove the deadline is per write", time.Since(start))
	}
}

// A handler that sets its own deadline, like the SSE feed, keeps it
func TestWriteDeadlineYieldsToHandlerDeadline(t *testing.T) {
	done := make(chan error, 1)
	own := func(w http.ResponseWriter) {
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(time.Hour))
	}
	srv := httptest.NewServer(WriteDeadline(200 * time.Millisecond)(streamHandler(256<<20, own, done)))
	defer srv.Close()
	conn := dialAndRequest(t, srv)

	select {
	case err := <-done:
		t.Fatalf("handler with its own hour-long deadline ended early: %v", err)
	case <-time.After(1500 * time.Millisecond):
	}
	conn.Close()
	<-done
}

func TestGlobalLimiterRejectsWhenFull(t *testing.T) {
	l := NewGlobalLimiter(2)
	release := make(chan struct{})
	held := make(chan struct{}, 2)
	h := l.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		held <- struct{}{}
		<-release
	}))
	for i := 0; i < 2; i++ {
		go h.ServeHTTP(httptest.NewRecorder(), requestFrom(fmt.Sprintf("10.0.0.%d:4000", i+1)))
		<-held
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, requestFrom("10.0.0.9:4000"))
	close(release)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("third client got %d with both slots held, want 503", rec.Code)
	}
}

func TestRecoveryPassesAbortThrough(t *testing.T) {
	h := Recovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("partial"))
		panic(http.ErrAbortHandler)
	}))
	rec := httptest.NewRecorder()
	defer func() {
		if got := recover(); got != http.ErrAbortHandler {
			t.Fatalf("recovered %v, want http.ErrAbortHandler to reach net/http", got)
		}
		if rec.Code != http.StatusOK || rec.Body.String() != "partial" {
			t.Fatalf("Recovery wrote to an aborted response: %d %q", rec.Code, rec.Body.String())
		}
	}()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
}
