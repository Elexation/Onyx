package storage

import (
	"os"
	"runtime"
	"syscall"
	"testing"
)

func TestIsCrossDevice(t *testing.T) {
	rename := func(err error) error { return &os.LinkError{Op: "rename", Old: "a", New: "b", Err: err} }
	if !IsCrossDevice(rename(syscall.EXDEV)) {
		t.Error("EXDEV not treated as cross-device")
	}
	// Windows reports a rename across drives as ERROR_NOT_SAME_DEVICE, which is EEXIST elsewhere
	if got, want := IsCrossDevice(rename(syscall.Errno(17))), runtime.GOOS == "windows"; got != want {
		t.Errorf("errno 17 on %s: got %v, want %v", runtime.GOOS, got, want)
	}
	if IsCrossDevice(rename(os.ErrPermission)) {
		t.Error("permission error treated as cross-device")
	}
	if IsCrossDevice(syscall.EXDEV) {
		t.Error("bare errno without a LinkError treated as cross-device")
	}
}
