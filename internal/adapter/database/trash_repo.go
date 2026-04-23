package database

import (
	"database/sql"
	"fmt"

	"github.com/Elexation/onyx/internal/domain"
)

type TrashRepo struct {
	db *sql.DB
}

func NewTrashRepo(db *sql.DB) *TrashRepo {
	return &TrashRepo{db: db}
}

func (r *TrashRepo) Insert(item *domain.TrashItem) error {
	_, err := r.db.Exec(
		"INSERT INTO trash_items (id, original_path, trash_path, deleted_at, size, is_dir) VALUES (?, ?, ?, ?, ?, ?)",
		item.ID, item.OriginalPath, item.TrashPath, item.DeletedAt, item.Size, item.IsDir,
	)
	if err != nil {
		return fmt.Errorf("insert trash item: %w", err)
	}
	return nil
}

func (r *TrashRepo) GetByID(id string) (*domain.TrashItem, error) {
	item := &domain.TrashItem{}
	var isDir int
	err := r.db.QueryRow(
		"SELECT id, original_path, trash_path, deleted_at, size, is_dir FROM trash_items WHERE id = ?",
		id,
	).Scan(&item.ID, &item.OriginalPath, &item.TrashPath, &item.DeletedAt, &item.Size, &isDir)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get trash item: %w", err)
	}
	item.IsDir = isDir != 0
	return item, nil
}

func (r *TrashRepo) List() ([]domain.TrashItem, error) {
	rows, err := r.db.Query(
		"SELECT id, original_path, trash_path, deleted_at, size, is_dir FROM trash_items ORDER BY deleted_at DESC",
	)
	if err != nil {
		return nil, fmt.Errorf("list trash items: %w", err)
	}
	defer rows.Close()

	var items []domain.TrashItem
	for rows.Next() {
		var item domain.TrashItem
		var isDir int
		if err := rows.Scan(&item.ID, &item.OriginalPath, &item.TrashPath, &item.DeletedAt, &item.Size, &isDir); err != nil {
			return nil, fmt.Errorf("scan trash item: %w", err)
		}
		item.IsDir = isDir != 0
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *TrashRepo) Delete(id string) error {
	_, err := r.db.Exec("DELETE FROM trash_items WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete trash item: %w", err)
	}
	return nil
}

func (r *TrashRepo) DeleteAll() ([]domain.TrashItem, error) {
	items, err := r.List()
	if err != nil {
		return nil, err
	}
	_, err = r.db.Exec("DELETE FROM trash_items")
	if err != nil {
		return nil, fmt.Errorf("delete all trash items: %w", err)
	}
	return items, nil
}

func (r *TrashRepo) Count() (int, error) {
	var count int
	err := r.db.QueryRow("SELECT COUNT(*) FROM trash_items").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count trash items: %w", err)
	}
	return count, nil
}

func (r *TrashRepo) TotalSize() (int64, error) {
	var total sql.NullInt64
	err := r.db.QueryRow("SELECT SUM(size) FROM trash_items").Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("total trash size: %w", err)
	}
	if !total.Valid {
		return 0, nil
	}
	return total.Int64, nil
}

func (r *TrashRepo) ListExpiredBefore(unixTime int64) ([]domain.TrashItem, error) {
	rows, err := r.db.Query(
		"SELECT id, original_path, trash_path, deleted_at, size, is_dir FROM trash_items WHERE deleted_at < ? ORDER BY deleted_at ASC",
		unixTime,
	)
	if err != nil {
		return nil, fmt.Errorf("list expired trash: %w", err)
	}
	defer rows.Close()

	var items []domain.TrashItem
	for rows.Next() {
		var item domain.TrashItem
		var isDir int
		if err := rows.Scan(&item.ID, &item.OriginalPath, &item.TrashPath, &item.DeletedAt, &item.Size, &isDir); err != nil {
			return nil, fmt.Errorf("scan expired trash item: %w", err)
		}
		item.IsDir = isDir != 0
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *TrashRepo) ListOldestFirst() ([]domain.TrashItem, error) {
	rows, err := r.db.Query(
		"SELECT id, original_path, trash_path, deleted_at, size, is_dir FROM trash_items ORDER BY deleted_at ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("list trash oldest first: %w", err)
	}
	defer rows.Close()

	var items []domain.TrashItem
	for rows.Next() {
		var item domain.TrashItem
		var isDir int
		if err := rows.Scan(&item.ID, &item.OriginalPath, &item.TrashPath, &item.DeletedAt, &item.Size, &isDir); err != nil {
			return nil, fmt.Errorf("scan trash item: %w", err)
		}
		item.IsDir = isDir != 0
		items = append(items, item)
	}
	return items, rows.Err()
}
