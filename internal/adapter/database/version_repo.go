package database

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/Elexation/onyx/internal/domain"
)

type VersionRepo struct {
	db *sql.DB
}

func NewVersionRepo(db *sql.DB) *VersionRepo {
	return &VersionRepo{db: db}
}

func (r *VersionRepo) Insert(v *domain.FileVersion) error {
	res, err := r.db.Exec(
		"INSERT INTO file_versions (file_path, version_path, created_at, size) VALUES (?, ?, ?, ?)",
		v.FilePath, v.VersionPath, v.CreatedAt, v.Size,
	)
	if err != nil {
		return fmt.Errorf("insert file version: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	v.ID = id
	return nil
}

func (r *VersionRepo) GetByID(id int64) (*domain.FileVersion, error) {
	v := &domain.FileVersion{}
	err := r.db.QueryRow(
		"SELECT id, file_path, version_path, created_at, size FROM file_versions WHERE id = ?",
		id,
	).Scan(&v.ID, &v.FilePath, &v.VersionPath, &v.CreatedAt, &v.Size)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get file version: %w", err)
	}
	return v, nil
}

func (r *VersionRepo) ListByPath(filePath string) ([]domain.FileVersion, error) {
	rows, err := r.db.Query(
		"SELECT id, file_path, version_path, created_at, size FROM file_versions WHERE file_path = ? ORDER BY created_at DESC",
		filePath,
	)
	if err != nil {
		return nil, fmt.Errorf("list versions for path: %w", err)
	}
	defer rows.Close()
	return scanVersions(rows)
}

func (r *VersionRepo) CountByPath(filePath string) (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM file_versions WHERE file_path = ?", filePath).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count versions for path: %w", err)
	}
	return count, nil
}

func (r *VersionRepo) Delete(id int64) error {
	_, err := r.db.Exec("DELETE FROM file_versions WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete file version: %w", err)
	}
	return nil
}

func (r *VersionRepo) ListAllPaths() ([]string, error) {
	rows, err := r.db.Query("SELECT DISTINCT file_path FROM file_versions")
	if err != nil {
		return nil, fmt.Errorf("list version paths: %w", err)
	}
	defer rows.Close()

	var paths []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, fmt.Errorf("scan path: %w", err)
		}
		paths = append(paths, p)
	}
	return paths, rows.Err()
}

func (r *VersionRepo) TotalSize() (int64, error) {
	var total sql.NullInt64
	err := r.db.QueryRow("SELECT SUM(size) FROM file_versions").Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("total version size: %w", err)
	}
	if !total.Valid {
		return 0, nil
	}
	return total.Int64, nil
}

func (r *VersionRepo) ListOldestFirst() ([]domain.FileVersion, error) {
	rows, err := r.db.Query(
		"SELECT id, file_path, version_path, created_at, size FROM file_versions ORDER BY created_at ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("list versions oldest first: %w", err)
	}
	defer rows.Close()
	return scanVersions(rows)
}

// RenameFile updates the file_path and version_path for all versions of a
// single file that was renamed or moved. oldPath and newPath are the
// leading-slash data-relative paths of the file.
func (r *VersionRepo) RenameFile(oldPath, newPath string) error {
	versions, err := r.ListByPath(oldPath)
	if err != nil {
		return err
	}
	if len(versions) == 0 {
		return nil
	}

	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin rename tx: %w", err)
	}
	for _, v := range versions {
		suffix := strings.TrimPrefix(v.VersionPath, versionPathFor(oldPath))
		newVersionPath := versionPathFor(newPath) + suffix
		if _, err := tx.Exec(
			"UPDATE file_versions SET file_path = ?, version_path = ? WHERE id = ?",
			newPath, newVersionPath, v.ID,
		); err != nil {
			tx.Rollback()
			return fmt.Errorf("update version path: %w", err)
		}
	}
	return tx.Commit()
}

// RenameDir updates versions whose file_path is inside oldDir to the new
// directory. Both args are leading-slash data-relative dir paths.
func (r *VersionRepo) RenameDir(oldDir, newDir string) error {
	oldPrefix := strings.TrimSuffix(oldDir, "/") + "/"
	newPrefix := strings.TrimSuffix(newDir, "/") + "/"
	oldVersionPrefix := strings.TrimPrefix(oldPrefix, "/")
	newVersionPrefix := strings.TrimPrefix(newPrefix, "/")

	_, err := r.db.Exec(
		`UPDATE file_versions
		 SET file_path = ? || substr(file_path, ?),
		     version_path = ? || substr(version_path, ?)
		 WHERE file_path LIKE ? || '%'`,
		newPrefix, len(oldPrefix)+1,
		newVersionPrefix, len(oldVersionPrefix)+1,
		oldPrefix,
	)
	if err != nil {
		return fmt.Errorf("rename dir versions: %w", err)
	}
	return nil
}

// versionPathFor returns the version-store-relative prefix for a file path.
// e.g. "/Documents/report.docx" -> "Documents/report.docx"
func versionPathFor(filePath string) string {
	return strings.TrimPrefix(filePath, "/")
}

func scanVersions(rows *sql.Rows) ([]domain.FileVersion, error) {
	var versions []domain.FileVersion
	for rows.Next() {
		var v domain.FileVersion
		if err := rows.Scan(&v.ID, &v.FilePath, &v.VersionPath, &v.CreatedAt, &v.Size); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		versions = append(versions, v)
	}
	return versions, rows.Err()
}
