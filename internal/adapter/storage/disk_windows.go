//go:build windows

package storage

import "golang.org/x/sys/windows"

// diskUsage returns used and total bytes for the volume hosting path.
// freeBytesAvailable is the caller-quota-aware free count, matching what
// the current process can actually consume.
func diskUsage(path string) (used, total uint64, err error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, err
	}
	var freeAvail, totalBytes, totalFree uint64
	if err = windows.GetDiskFreeSpaceEx(p, &freeAvail, &totalBytes, &totalFree); err != nil {
		return 0, 0, err
	}
	if freeAvail > totalBytes {
		freeAvail = totalBytes
	}
	used = totalBytes - freeAvail
	total = totalBytes
	return used, total, nil
}
