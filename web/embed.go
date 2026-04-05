//go:build !dev

package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:build
var staticFiles embed.FS

func SPAHandler() http.HandlerFunc {
	fsys, err := fs.Sub(staticFiles, "build")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServer(http.FS(fsys))

	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path != "/" {
			_, err := fs.Stat(fsys, path[1:])
			if err == nil {
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		index, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(index)
	}
}
