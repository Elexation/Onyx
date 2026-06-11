//go:build !windows

package storage

import "syscall"

// diskUsage returns used and total bytes for the filesystem hosting path.
// Uses Bavail (free blocks for unprivileged users) so the "used" figure
// matches what a non-root process could actually fill.
func diskUsage(path string) (used, total uint64, err error) {
	var s syscall.Statfs_t
	if err = syscall.Statfs(path, &s); err != nil {
		return 0, 0, err
	}
	bsize := uint64(s.Bsize)
	total = bsize * uint64(s.Blocks)
	free := bsize * uint64(s.Bavail)
	if free > total {
		free = total
	}
	used = total - free
	return used, total, nil
}
