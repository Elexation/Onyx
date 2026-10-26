package handler

import (
	"net/http/httptest"
	"runtime"
	"testing"
)

func TestRejectSepAlias(t *testing.T) {
	w := httptest.NewRecorder()
	if rejectSepAlias(w, "/docs/a.txt", "/x") {
		t.Fatal("rejected a plain path")
	}
	w = httptest.NewRecorder()
	got := rejectSepAlias(w, "/docs/a.txt", `/docs\a.txt`)
	if want := runtime.GOOS == "windows"; got != want {
		t.Fatalf("backslash path: rejected=%v, want %v", got, want)
	}
	if got && w.Code != 400 {
		t.Fatalf("status %d, want 400", w.Code)
	}
}
