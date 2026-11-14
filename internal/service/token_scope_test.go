package service

import (
	"net/http"
	"testing"

	"github.com/Elexation/onyx/internal/domain"
)

func TestCheckScope(t *testing.T) {
	G, P, D := http.MethodGet, http.MethodPost, http.MethodDelete
	cases := []struct {
		scope, method, path string
		want                bool
	}{
		// Never via token, whatever the scope
		{domain.ScopeFull, P, "/api/shares", false},
		{domain.ScopeFull, P, "/api/shares/", false},
		{domain.ScopeFull, D, "/api/trash/12", false},
		{domain.ScopeFull, D, "/api/trash", false},
		{domain.ScopeFull, D, "/api/trash/", false},
		{domain.ScopeFull, D, "/api/versions/7", false},
		{domain.ScopeFull, P, "/api/tokens", false},
		{domain.ScopeFull, G, "/api/settings", false},

		// Full scope keeps everything else
		{domain.ScopeFull, G, "/api/shares", true},
		{domain.ScopeFull, D, "/api/shares/3", true},
		{domain.ScopeFull, D, "/api/files", true},
		{domain.ScopeFull, P, "/api/trash/12/restore", true},
		{domain.ScopeFull, P, "/api/versions/7/restore", true},
		{domain.ScopeFull, P, "/api/files/move", true},

		// Upload scope
		{domain.ScopeUpload, G, "/api/files/", true},
		{domain.ScopeUpload, P, "/api/upload/", true},
		{domain.ScopeUpload, P, "/api/files/mkdir", true},
		{domain.ScopeUpload, D, "/api/files", false},
		{domain.ScopeUpload, P, "/api/shares", false},

		// Read scope
		{domain.ScopeRead, G, "/api/download/a.txt", true},
		{domain.ScopeRead, P, "/api/upload/", false},
	}
	for _, c := range cases {
		if got := CheckScope(c.scope, c.method, c.path); got != c.want {
			t.Errorf("CheckScope(%s, %s, %s) = %v, want %v", c.scope, c.method, c.path, got, c.want)
		}
	}
}
