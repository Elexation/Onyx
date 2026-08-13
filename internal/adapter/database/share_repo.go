package database

import (
	"database/sql"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/Elexation/onyx/internal/domain"
)

type ShareRepo struct {
	db *sql.DB
}

func NewShareRepo(db *sql.DB) *ShareRepo {
	return &ShareRepo{db: db}
}

func (r *ShareRepo) Create(tokenHash, tokenLast8, filePath string, isDir bool, createdAt int64, expiresAt *int64, passwordHash *string) (int64, error) {
	res, err := r.db.Exec(
		"INSERT INTO share_links (token_hash, token_last8, file_path, is_dir, created_at, expires_at, password_hash) VALUES (?, ?, ?, ?, ?, ?, ?)",
		tokenHash, tokenLast8, filePath, isDir, createdAt, expiresAt, passwordHash,
	)
	if err != nil {
		return 0, fmt.Errorf("insert share link: %w", err)
	}
	return res.LastInsertId()
}

func (r *ShareRepo) GetByTokenHash(tokenHash string) (*domain.ShareLink, *string, error) {
	var link domain.ShareLink
	var isDir int
	var expiresAt sql.NullInt64
	var passwordHash sql.NullString
	err := r.db.QueryRow(
		"SELECT id, token_last8, file_path, is_dir, created_at, expires_at, password_hash, download_count FROM share_links WHERE token_hash = ?",
		tokenHash,
	).Scan(&link.ID, &link.TokenLast8, &link.FilePath, &isDir, &link.CreatedAt, &expiresAt, &passwordHash, &link.DownloadCount)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("get share by token: %w", err)
	}
	link.IsDir = isDir != 0
	if expiresAt.Valid {
		link.ExpiresAt = expiresAt.Int64
	}
	link.HasPassword = passwordHash.Valid
	var pwHash *string
	if passwordHash.Valid {
		pwHash = &passwordHash.String
	}
	return &link, pwHash, nil
}

func (r *ShareRepo) GetByPath(filePath string) (*domain.ShareLink, error) {
	var link domain.ShareLink
	var isDir int
	var expiresAt sql.NullInt64
	var passwordHash sql.NullString
	err := r.db.QueryRow(
		"SELECT id, token_last8, file_path, is_dir, created_at, expires_at, password_hash, download_count FROM share_links WHERE file_path = ?",
		filePath,
	).Scan(&link.ID, &link.TokenLast8, &link.FilePath, &isDir, &link.CreatedAt, &expiresAt, &passwordHash, &link.DownloadCount)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get share by path: %w", err)
	}
	link.IsDir = isDir != 0
	if expiresAt.Valid {
		link.ExpiresAt = expiresAt.Int64
	}
	link.HasPassword = passwordHash.Valid
	return &link, nil
}

func (r *ShareRepo) List() ([]domain.ShareLink, error) {
	rows, err := r.db.Query(
		"SELECT id, token_last8, file_path, is_dir, created_at, expires_at, password_hash, download_count FROM share_links ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("list shares: %w", err)
	}
	defer rows.Close()

	links := make([]domain.ShareLink, 0)
	for rows.Next() {
		var link domain.ShareLink
		var isDir int
		var expiresAt sql.NullInt64
		var passwordHash sql.NullString
		if err := rows.Scan(&link.ID, &link.TokenLast8, &link.FilePath, &isDir, &link.CreatedAt, &expiresAt, &passwordHash, &link.DownloadCount); err != nil {
			return nil, fmt.Errorf("scan share link: %w", err)
		}
		link.IsDir = isDir != 0
		if expiresAt.Valid {
			link.ExpiresAt = expiresAt.Int64
		}
		link.HasPassword = passwordHash.Valid
		links = append(links, link)
	}
	return links, rows.Err()
}

func (r *ShareRepo) Delete(id int64) error {
	res, err := r.db.Exec("DELETE FROM share_links WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete share link: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("share link not found")
	}
	return nil
}

func (r *ShareRepo) IncrementDownloadCount(id int64) error {
	_, err := r.db.Exec("UPDATE share_links SET download_count = download_count + 1 WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("increment download count: %w", err)
	}
	return nil
}

func (r *ShareRepo) DeleteAll() (int64, error) {
	res, err := r.db.Exec("DELETE FROM share_links")
	if err != nil {
		return 0, fmt.Errorf("delete all shares: %w", err)
	}
	return res.RowsAffected()
}

func (r *ShareRepo) Count() (int64, error) {
	var n int64
	err := r.db.QueryRow("SELECT COUNT(*) FROM share_links").Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count shares: %w", err)
	}
	return n, nil
}

func (r *ShareRepo) DeleteExpired(now int64) (int64, error) {
	res, err := r.db.Exec("DELETE FROM share_links WHERE expires_at IS NOT NULL AND expires_at < ?", now)
	if err != nil {
		return 0, fmt.Errorf("delete expired shares: %w", err)
	}
	return res.RowsAffected()
}

// DeleteByPath removes the share whose file_path is exactly p. Returns the
// row count (0 if no share existed for that path).
func (r *ShareRepo) DeleteByPath(p string) (int64, error) {
	res, err := r.db.Exec("DELETE FROM share_links WHERE file_path = ?", p)
	if err != nil {
		return 0, fmt.Errorf("delete share by path: %w", err)
	}
	return res.RowsAffected()
}

// DeleteByPathRecursive removes the share for dirPath itself AND every share
// whose file_path is a descendant of dirPath. Used when a directory is
// trashed/deleted to cascade-clean shares pointing inside it.
func (r *ShareRepo) DeleteByPathRecursive(dirPath string) (int64, error) {
	prefix := strings.TrimSuffix(dirPath, "/") + "/"
	res, err := r.db.Exec(
		`DELETE FROM share_links WHERE file_path = ? OR file_path LIKE ? || '%' ESCAPE '\'`,
		dirPath, escapeLike(prefix),
	)
	if err != nil {
		return 0, fmt.Errorf("delete shares recursive: %w", err)
	}
	return res.RowsAffected()
}

// UpdatePath rewrites the file_path of the share matching oldPath exactly.
// No-op if no share exists. Errors propagate (e.g. UNIQUE collision if a
// share already exists at newPath — should not happen in valid rename flows
// since the storage rename would have been rejected).
func (r *ShareRepo) UpdatePath(oldPath, newPath string) (int64, error) {
	res, err := r.db.Exec("UPDATE share_links SET file_path = ? WHERE file_path = ?", newPath, oldPath)
	if err != nil {
		return 0, fmt.Errorf("update share path: %w", err)
	}
	return res.RowsAffected()
}

// UpdatePathRecursive rewrites file_path for the directory itself AND every
// descendant share, replacing the oldDir prefix with newDir. Used when a
// directory is renamed or moved.
func (r *ShareRepo) UpdatePathRecursive(oldDir, newDir string) (int64, error) {
	oldPrefix := strings.TrimSuffix(oldDir, "/") + "/"
	newPrefix := strings.TrimSuffix(newDir, "/") + "/"

	tx, err := r.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("begin update tx: %w", err)
	}

	// Exact-match update for the dir itself (if shared).
	res1, err := tx.Exec("UPDATE share_links SET file_path = ? WHERE file_path = ?", newDir, oldDir)
	if err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("update share dir path: %w", err)
	}
	n1, _ := res1.RowsAffected()

	// Prefix update for descendants. substr is 1-indexed; cut after the old prefix.
	res2, err := tx.Exec(
		`UPDATE share_links
		 SET file_path = ? || substr(file_path, ?)
		 WHERE file_path LIKE ? || '%' ESCAPE '\'`,
		newPrefix, utf8.RuneCountInString(oldPrefix)+1, escapeLike(oldPrefix),
	)
	if err != nil {
		tx.Rollback()
		return 0, fmt.Errorf("update share descendant paths: %w", err)
	}
	n2, _ := res2.RowsAffected()

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit update tx: %w", err)
	}
	return n1 + n2, nil
}
