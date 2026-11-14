package main

import (
	"testing"
	"time"
)

func TestParseRetentionInterval(t *testing.T) {
	if d, err := parseRetentionInterval(""); err != nil || d != 24*time.Hour {
		t.Fatalf("unset: got %v, %v", d, err)
	}
	if d, err := parseRetentionInterval("90m"); err != nil || d != 90*time.Minute {
		t.Fatalf("90m: got %v, %v", d, err)
	}
	for _, v := range []string{"0", "0s", "-1h", "soon"} {
		if _, err := parseRetentionInterval(v); err == nil {
			t.Errorf("%q accepted", v)
		}
	}
}

func TestValidateRedirectPort(t *testing.T) {
	if err := validateRedirectPort("80", "8443"); err != nil {
		t.Fatalf("80 with listen 8443 rejected: %v", err)
	}
	for _, c := range []struct{ redirect, listen string }{
		{"8443", "8443"},
		{"0", "8443"},
		{"65536", "8443"},
		{"http", "8443"},
	} {
		if err := validateRedirectPort(c.redirect, c.listen); err == nil {
			t.Errorf("redirect %q with listen %q accepted", c.redirect, c.listen)
		}
	}
}
