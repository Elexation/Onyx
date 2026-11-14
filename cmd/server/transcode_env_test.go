package main

import "testing"

func TestParseByteSize(t *testing.T) {
	for in, want := range map[string]int64{
		"0":       0,
		"1048576": 1 << 20,
		"20GB":    20 << 30,
		"20gb":    20 << 30,
		" 512 MB": 512 << 20,
		"2TB":     2 << 40,
		"64KB":    64 << 10,
		"100B":    100,
	} {
		if got, err := parseByteSize(in); err != nil || got != want {
			t.Errorf("%q: got %d, %v, want %d", in, got, err, want)
		}
	}
	for _, in := range []string{"", "GB", "-1GB", "20 GiB", "1.5GB", "9999999999TB"} {
		if _, err := parseByteSize(in); err == nil {
			t.Errorf("%q accepted", in)
		}
	}
}
