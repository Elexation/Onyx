package storage

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/KarpelesLab/reflink"
)

// VersionStore manages the .versions/ directory. It operates outside of
// LocalStorage's os.Root sandbox because reflink requires filesystem paths.
// All inputs come from trusted server code, not user input.
type VersionStore struct {
	dataDir     string
	versionsDir string
}

func NewVersionStore(dataDir, versionsDir string) (*VersionStore, error) {
	if err := os.MkdirAll(versionsDir, 0755); err != nil {
		return nil, fmt.Errorf("create versions directory: %w", err)
	}
	return &VersionStore{
		dataDir:     dataDir,
		versionsDir: versionsDir,
	}, nil
}

// TestReflink probes whether the versions directory filesystem supports
// reflinks by creating a temp file and attempting reflink.Always. Logs the
// result. Returns true if supported.
func (s *VersionStore) TestReflink() bool {
	src := filepath.Join(s.versionsDir, ".reflink_test")
	dst := filepath.Join(s.versionsDir, ".reflink_test.ref")

	if err := os.WriteFile(src, []byte("probe"), 0644); err != nil {
		slog.Info("reflink test: cannot write probe file", "error", err)
		return false
	}
	defer os.Remove(src)

	err := reflink.Always(src, dst)
	if err != nil {
		os.Remove(dst)
		slog.Info("reflinks unsupported; versioning will fall back to io.Copy", "error", err)
		return false
	}
	os.Remove(dst)
	slog.Info("reflinks supported on versions filesystem")
	return true
}

// StoreVersion copies the file at data-relative filePath into the versions
// directory using reflink.Auto (reflink → copy_file_range → io.Copy). Returns
// the versions-relative path of the new version file.
func (s *VersionStore) StoreVersion(filePath string, timestampNs int64) (string, error) {
	relPath := strings.TrimPrefix(filePath, "/")
	srcAbs := filepath.Join(s.dataDir, filepath.FromSlash(relPath))

	versionRel := fmt.Sprintf("%s.%d", relPath, timestampNs)
	dstAbs := filepath.Join(s.versionsDir, filepath.FromSlash(versionRel))

	if err := os.MkdirAll(filepath.Dir(dstAbs), 0755); err != nil {
		return "", fmt.Errorf("create version parent dir: %w", err)
	}

	if err := reflink.Auto(srcAbs, dstAbs); err != nil {
		return "", fmt.Errorf("copy version: %w", err)
	}

	return versionRel, nil
}

// RestoreVersion copies the version file back to the data path using
// reflink.Auto. Overwrites the current file.
func (s *VersionStore) RestoreVersion(filePath, versionRel string) error {
	srcAbs := filepath.Join(s.versionsDir, filepath.FromSlash(versionRel))
	dstRel := strings.TrimPrefix(filePath, "/")
	dstAbs := filepath.Join(s.dataDir, filepath.FromSlash(dstRel))

	if err := os.MkdirAll(filepath.Dir(dstAbs), 0755); err != nil {
		return fmt.Errorf("create data parent dir: %w", err)
	}

	// reflink.Auto requires the destination to not exist; remove first.
	if err := os.Remove(dstAbs); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove current file: %w", err)
	}

	if err := reflink.Auto(srcAbs, dstAbs); err != nil {
		return fmt.Errorf("restore version: %w", err)
	}
	return nil
}

// DeleteVersion removes a version file from disk.
func (s *VersionStore) DeleteVersion(versionRel string) error {
	abs := filepath.Join(s.versionsDir, filepath.FromSlash(versionRel))
	if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete version file: %w", err)
	}
	return nil
}

// RenameFile moves all version files for a renamed data file. Operates on
// versions-relative paths returned from StoreVersion.
func (s *VersionStore) RenameFile(oldFilePath, newFilePath string) error {
	oldRel := strings.TrimPrefix(oldFilePath, "/")
	newRel := strings.TrimPrefix(newFilePath, "/")
	oldDir := filepath.Join(s.versionsDir, filepath.FromSlash(filepath.Dir(oldRel)))

	oldBase := filepath.Base(oldRel) + "."
	newBase := filepath.Base(newRel) + "."
	newParent := filepath.Join(s.versionsDir, filepath.FromSlash(filepath.Dir(newRel)))

	if err := os.MkdirAll(newParent, 0755); err != nil {
		return fmt.Errorf("create new version parent: %w", err)
	}

	entries, err := os.ReadDir(oldDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read version dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasPrefix(name, oldBase) {
			continue
		}
		suffix := strings.TrimPrefix(name, oldBase)
		// Version filenames are "{base}.{nanosecondTimestamp}". The suffix
		// after oldBase must be all digits, otherwise this is actually a
		// version of a different file whose name shares the prefix
		// (e.g. versions of "report.docx.bak" would match oldBase "report.docx.").
		if !isAllDigits(suffix) {
			continue
		}
		src := filepath.Join(oldDir, name)
		dst := filepath.Join(newParent, newBase+suffix)
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("rename version file: %w", err)
		}
	}
	return nil
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// RenameDir moves the versions subtree for a renamed directory.
func (s *VersionStore) RenameDir(oldDirPath, newDirPath string) error {
	oldRel := strings.TrimPrefix(oldDirPath, "/")
	newRel := strings.TrimPrefix(newDirPath, "/")
	src := filepath.Join(s.versionsDir, filepath.FromSlash(oldRel))
	dst := filepath.Join(s.versionsDir, filepath.FromSlash(newRel))

	if _, err := os.Stat(src); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat version dir: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("create new version parent dir: %w", err)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("rename version dir: %w", err)
	}
	return nil
}
