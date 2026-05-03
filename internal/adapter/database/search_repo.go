package database

import (
	"database/sql"
	"strings"
	"time"

	"github.com/Elexation/onyx/internal/domain"
)

type SearchRepo struct {
	db *sql.DB
}

func NewSearchRepo(db *sql.DB) *SearchRepo {
	return &SearchRepo{db: db}
}

type FileEntry struct {
	Name    string
	Path    string
	IsDir   bool
	Size    int64
	ModTime int64
}

func (r *SearchRepo) Upsert(name, path string, isDir bool, size, modTime int64) error {
	_, err := r.db.Exec(`
		INSERT INTO files (name, path, is_dir, size, mod_time, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			name=excluded.name, is_dir=excluded.is_dir, size=excluded.size,
			mod_time=excluded.mod_time, indexed_at=excluded.indexed_at`,
		name, path, isDir, size, modTime, time.Now().Unix(),
	)
	return err
}

func (r *SearchRepo) UpsertBatch(items []FileEntry) error {
	tx, err := r.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO files (name, path, is_dir, size, mod_time, indexed_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(path) DO UPDATE SET
			name=excluded.name, is_dir=excluded.is_dir, size=excluded.size,
			mod_time=excluded.mod_time, indexed_at=excluded.indexed_at`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	now := time.Now().Unix()
	for _, item := range items {
		if _, err := stmt.Exec(item.Name, item.Path, item.IsDir, item.Size, item.ModTime, now); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (r *SearchRepo) Delete(path string) error {
	_, err := r.db.Exec("DELETE FROM files WHERE path = ?", path)
	return err
}

func (r *SearchRepo) DeleteTree(path string) error {
	_, err := r.db.Exec("DELETE FROM files WHERE path = ? OR path LIKE ? || '/%'", path, path)
	return err
}

func (r *SearchRepo) Search(query string, limit int) ([]domain.SearchResult, int, error) {
	ftsQuery := buildFTSQuery(query)
	if ftsQuery == "" {
		return nil, 0, nil
	}

	rows, err := r.db.Query(`
		SELECT f.name, f.path, f.is_dir
		FROM file_search s
		JOIN files f ON f.id = s.rowid
		WHERE file_search MATCH ?
		ORDER BY rank
		LIMIT ?`, ftsQuery, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []domain.SearchResult
	for rows.Next() {
		var sr domain.SearchResult
		if err := rows.Scan(&sr.Name, &sr.Path, &sr.IsDir); err != nil {
			return nil, 0, err
		}
		results = append(results, sr)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	var total int
	err = r.db.QueryRow("SELECT COUNT(*) FROM file_search WHERE file_search MATCH ?", ftsQuery).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	return results, total, nil
}

func (r *SearchRepo) DeleteStale(olderThan int64) (int64, error) {
	res, err := r.db.Exec("DELETE FROM files WHERE indexed_at < ?", olderThan)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *SearchRepo) UpdatePath(oldPath, newPath, newName string) error {
	_, err := r.db.Exec("UPDATE files SET path = ?, name = ? WHERE path = ?", newPath, newName, oldPath)
	return err
}

func (r *SearchRepo) UpdatePathPrefix(oldPrefix, newPrefix string) error {
	newName := newPrefix[strings.LastIndex(newPrefix, "/")+1:]
	_, err := r.db.Exec(`
		UPDATE files SET
			path = ? || substr(path, ?),
			name = CASE WHEN path = ? THEN ? ELSE name END
		WHERE path = ? OR path LIKE ? || '/%'`,
		newPrefix, len(oldPrefix)+1, oldPrefix, newName, oldPrefix, oldPrefix,
	)
	return err
}

// buildFTSQuery sanitizes user input and builds an FTS5 prefix query.
// "report doc" becomes "report* doc*" for prefix matching.
func buildFTSQuery(input string) string {
	// Strip FTS5 special characters
	replacer := strings.NewReplacer(
		`"`, "", `*`, "", `(`, "", `)`, "",
		`+`, "", `-`, " ", `^`, "", `{`, "",
		`}`, "", `:`, "",
	)
	cleaned := replacer.Replace(input)

	tokens := strings.Fields(cleaned)
	if len(tokens) == 0 {
		return ""
	}

	for i, t := range tokens {
		tokens[i] = t + "*"
	}
	return strings.Join(tokens, " ")
}
