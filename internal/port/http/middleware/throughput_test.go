package middleware

import (
	"bufio"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Behind the per-write deadline too, as the router wires it
func throughputServer(t *testing.T, total int, done chan<- error) *httptest.Server {
	t.Helper()
	h := WriteDeadline(30 * time.Second)(MinThroughput(256<<10, 500*time.Millisecond)(streamHandler(total, nil, done)))
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

// A reader trickling well under the floor is dropped, though each write still beats the deadline
func TestMinThroughputDropsTrickle(t *testing.T) {
	done := make(chan error, 1)
	srv := throughputServer(t, 256<<20, done)
	conn := dialAndRequest(t, srv)
	go func() {
		buf := make([]byte, 1024)
		for {
			if _, err := conn.Read(buf); err != nil {
				return
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("trickling reader got the whole body")
		}
	case <-time.After(20 * time.Second):
		t.Fatal("trickling reader still holds the download")
	}
}

func TestMinThroughputKeepsFastReader(t *testing.T) {
	done := make(chan error, 1)
	const total = 32 << 20
	srv := throughputServer(t, total, done)
	conn := dialAndRequest(t, srv)
	resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
	if err != nil {
		t.Fatal(err)
	}
	n, _ := io.Copy(io.Discard, resp.Body)
	if err := <-done; err != nil || n != total {
		t.Fatalf("fast reader got %d of %d bytes, handler err %v", n, total, err)
	}
}
