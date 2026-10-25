package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	_ "github.com/ncruces/go-sqlite3/driver"
)

func openTestDB(t *testing.T) (*SearchRepo, *sql.DB) {
	t.Helper()
	dbPath := t.TempDir() + "/test.db"
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return NewSearchRepo(db), db
}

// countByPath returns the number of rows in the files table matching the exact path.
func countByPath(t *testing.T, db *sql.DB, path string) int {
	t.Helper()
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM files WHERE path = ?", path).Scan(&n); err != nil {
		t.Fatalf("count by path %q: %v", path, err)
	}
	return n
}

func TestDeleteTree_CaseExact(t *testing.T) {
	repo, db := openTestDB(t)
	now := time.Now().Unix()

	repo.Upsert("a.txt", "/Docs/a.txt", false, 1, now)
	repo.Upsert("b.txt", "/docs/b.txt", false, 1, now)
	repo.Upsert("c.txt", "/Docs0/c.txt", false, 1, now)
	repo.Upsert("d_%.txt", "/Docs/d_%.txt", false, 1, now)

	if err := repo.DeleteTree("/Docs"); err != nil {
		t.Fatalf("delete tree: %v", err)
	}
	if countByPath(t, db, "/Docs/a.txt") != 0 || countByPath(t, db, "/Docs/d_%.txt") != 0 {
		t.Error("descendant not deleted")
	}
	if countByPath(t, db, "/docs/b.txt") != 1 {
		t.Error("case-only sibling deleted")
	}
	if countByPath(t, db, "/Docs0/c.txt") != 1 {
		t.Error("name-prefix sibling deleted")
	}
}

func TestSearch_FilenamePunctuation(t *testing.T) {
	repo, _ := openTestDB(t)
	now := time.Now().Unix()

	repo.Upsert("report.pdf", "/docs/report.pdf", false, 100, now)
	repo.Upsert("my-file.txt", "/docs/my-file.txt", false, 100, now)
	repo.Upsert("a,b.txt", "/docs/a,b.txt", false, 100, now)

	tests := []struct {
		query string
		want  string // path expected in results; "" = only assert no error
	}{
		{"report.pdf", "/docs/report.pdf"},
		{"report", "/docs/report.pdf"},
		{"my-file", "/docs/my-file.txt"},
		{"a,b", "/docs/a,b.txt"},
		{"(report)", "/docs/report.pdf"},
		{"AND", ""},
		{"NOT report", ""},
		{"...", ""},
		{`"`, ""},
	}
	for _, tt := range tests {
		results, _, err := repo.Search(context.Background(), tt.query, 10)
		if err != nil {
			t.Errorf("Search(%q) returned error: %v", tt.query, err)
			continue
		}
		if tt.want == "" {
			continue
		}
		found := false
		for _, r := range results {
			if r.Path == tt.want {
				found = true
			}
		}
		if !found {
			t.Errorf("Search(%q): expected %s in results, got %v", tt.query, tt.want, results)
		}
	}
}

func TestDeleteTree_PercentInPath(t *testing.T) {
	repo, db := openTestDB(t)
	now := time.Now().Unix()

	// /100% (target) and /100-other (must NOT be affected)
	repo.Upsert("100%", "/100%", true, 0, now)
	repo.Upsert("file.txt", "/100%/file.txt", false, 100, now)
	repo.Upsert("100-other", "/100-other", true, 0, now)
	repo.Upsert("keep.txt", "/100-other/keep.txt", false, 200, now)

	if err := repo.DeleteTree("/100%"); err != nil {
		t.Fatalf("DeleteTree: %v", err)
	}

	if n := countByPath(t, db, "/100%"); n != 0 {
		t.Error("/100% should have been deleted")
	}
	if n := countByPath(t, db, "/100%/file.txt"); n != 0 {
		t.Error("/100%/file.txt should have been deleted")
	}
	if n := countByPath(t, db, "/100-other"); n != 1 {
		t.Error("/100-other was incorrectly deleted")
	}
	if n := countByPath(t, db, "/100-other/keep.txt"); n != 1 {
		t.Error("/100-other/keep.txt was incorrectly deleted")
	}
}

func TestDeleteTree_UnderscoreInPath(t *testing.T) {
	repo, db := openTestDB(t)
	now := time.Now().Unix()

	// /a_b (target) vs /axb (must NOT match _ as single-char wildcard)
	repo.Upsert("a_b", "/a_b", true, 0, now)
	repo.Upsert("child.txt", "/a_b/child.txt", false, 100, now)
	repo.Upsert("axb", "/axb", true, 0, now)
	repo.Upsert("safe.txt", "/axb/safe.txt", false, 200, now)

	if err := repo.DeleteTree("/a_b"); err != nil {
		t.Fatalf("DeleteTree: %v", err)
	}

	if n := countByPath(t, db, "/a_b"); n != 0 {
		t.Error("/a_b should have been deleted")
	}
	if n := countByPath(t, db, "/a_b/child.txt"); n != 0 {
		t.Error("/a_b/child.txt should have been deleted")
	}
	if n := countByPath(t, db, "/axb"); n != 1 {
		t.Error("/axb was incorrectly deleted")
	}
	if n := countByPath(t, db, "/axb/safe.txt"); n != 1 {
		t.Error("/axb/safe.txt was incorrectly deleted")
	}
}

func TestUpdatePathPrefix_PercentInPath(t *testing.T) {
	repo, db := openTestDB(t)
	now := time.Now().Unix()

	// Rename /50% -> /fifty-percent
	repo.Upsert("50%", "/50%", true, 0, now)
	repo.Upsert("doc.txt", "/50%/doc.txt", false, 100, now)
	repo.Upsert("50-normal", "/50-normal", true, 0, now)
	repo.Upsert("other.txt", "/50-normal/other.txt", false, 200, now)

	if err := repo.UpdatePathPrefix("/50%", "/fifty-percent"); err != nil {
		t.Fatalf("UpdatePathPrefix: %v", err)
	}

	// Old paths should be gone, new paths should exist
	if n := countByPath(t, db, "/50%"); n != 0 {
		t.Error("/50% should have been renamed")
	}
	if n := countByPath(t, db, "/50%/doc.txt"); n != 0 {
		t.Error("/50%/doc.txt should have been renamed")
	}
	if n := countByPath(t, db, "/fifty-percent"); n != 1 {
		t.Error("/fifty-percent not found after rename")
	}
	if n := countByPath(t, db, "/fifty-percent/doc.txt"); n != 1 {
		t.Error("/fifty-percent/doc.txt not found after rename")
	}

	// /50-normal must be untouched
	if n := countByPath(t, db, "/50-normal"); n != 1 {
		t.Error("/50-normal was incorrectly renamed")
	}
	if n := countByPath(t, db, "/50-normal/other.txt"); n != 1 {
		t.Error("/50-normal/other.txt was incorrectly renamed")
	}
}
