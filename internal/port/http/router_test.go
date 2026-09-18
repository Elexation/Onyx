package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// tusd never validates the upload id it reads off the URL, and its filestore
// joins that id straight into a filesystem path. Paths are set directly here
// because that is what normalizePath hands downstream.
func TestValidUploadID(t *testing.T) {
	cases := []struct {
		name string
		path string
		want int
	}{
		{"create has no id", "/api/upload/", http.StatusOK},
		{"bare prefix", "/api/upload", http.StatusOK},
		{"tusd hex id", "/api/upload/0f8b2c1d4e5a6b7c8d9e0f1a", http.StatusOK},
		{"hyphen and underscore", "/api/upload/probe-1_a", http.StatusOK},
		{"backslash traversal", `/api/upload/..\..\..\Windows\Temp\pwn`, http.StatusNotFound},
		{"slash traversal", "/api/upload/../../etc/passwd", http.StatusNotFound},
		{"nested segment", "/api/upload/a/b", http.StatusNotFound},
		{"dot in id", "/api/upload/id.info", http.StatusNotFound},
		{"null byte", "/api/upload/id\x00", http.StatusNotFound},
		{"drive letter", "/api/upload/C:/Windows/Temp/pwn", http.StatusNotFound},
	}

	reached := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodHead, "http://x/api/upload/", nil)
			req.URL.Path = tc.path
			rec := httptest.NewRecorder()
			validUploadID(reached).ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
