//go:build dev

package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const buildDir = "frontend/build"

func SPAHandler() http.HandlerFunc {
	fileServer := http.FileServer(http.Dir(buildDir))

	return func(w http.ResponseWriter, r *http.Request) {
		// path.Clean ignores "\", which Windows treats as a separator
		if strings.Contains(r.URL.Path, `\`) {
			http.NotFound(w, r)
			return
		}
		path := filepath.Join(buildDir, r.URL.Path)
		if _, err := os.Stat(path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		http.ServeFile(w, r, filepath.Join(buildDir, "index.html"))
	}
}
