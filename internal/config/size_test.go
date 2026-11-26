package config

import (
	"strings"
	"testing"
)

func TestParseByteSize(t *testing.T) {
	valid := []struct {
		raw  string
		want int64
	}{
		{"", 0},
		{"   ", 0},
		{"0", 0},
		{"0B", 0},
		{"0GB", 0},
		{"unlimited", 0},
		{"UNLIMITED", 0},
		{"1024", 1024},
		{"500B", 500},
		{"1K", 1 << 10},
		{"1KB", 1 << 10},
		{"1KiB", 1 << 10},
		{"500MB", 500 << 20},
		{"500 MB", 500 << 20},
		{"1024M", 1024 << 20},
		{"2GB", 2 << 30},
		{"1.5GB", (3 << 30) / 2},
		{"10GiB", 10 << 30},
		{"2TB", 2 << 40},
		{"1PiB", 1 << 50},
	}
	for _, tc := range valid {
		got, err := ParseByteSize(tc.raw)
		if err != nil || got != tc.want {
			t.Errorf("ParseByteSize(%q) = %d, %v; want %d", tc.raw, got, err, tc.want)
		}
	}
	invalid := []string{
		"-1", "-5GB", "+5GB", "abc", "GB", "5XB", "1.2.3GB", ".5GB", "5.GB",
		"0.1B", "5GB extra", "999999999999PB", "NaN", "Inf",
	}
	for _, raw := range invalid {
		if got, err := ParseByteSize(raw); err == nil {
			t.Errorf("ParseByteSize(%q) unexpectedly succeeded: %d", raw, got)
		}
	}
}

func TestNormalizeByteSize(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want string
		n    int64
	}{
		{"", "", 0},
		{"0", "", 0},
		{"unlimited", "", 0},
		{"500 mb", "500MB", 500 << 20},
		{"10gib", "10GiB", 10 << 30},
		{"1024m", "1024MB", 1024 << 20},
		{"4096", "4096", 4096},
	} {
		got, n, err := NormalizeByteSize(tc.raw)
		if err != nil || got != tc.want || n != tc.n {
			t.Errorf("NormalizeByteSize(%q) = %q, %d, %v; want %q, %d", tc.raw, got, n, err, tc.want, tc.n)
		}
	}
}

func TestGraveyardMaxSizeConfigRoundTrip(t *testing.T) {
	for _, text := range []string{"", Template, "[graveyard]\n", "[graveyard]\nmax_size = ''\n", "[graveyard]\nmax_size = '0'\n", "[graveyard]\nmax_size = 'unlimited'\n"} {
		cfg, err := Parse([]byte(text), "fixture.toml")
		if err != nil || cfg.Graveyard.MaxBytes() != 0 {
			t.Fatalf("unlimited default %q: %+v %v", text, cfg.Graveyard, err)
		}
	}
	cfg, err := Parse([]byte("[graveyard]\nmax_size = '5GB'\n"), "fixture.toml")
	if err != nil || cfg.Graveyard.MaxSize != "5GB" || cfg.Graveyard.MaxBytes() != 5<<30 {
		t.Fatalf("5GB config: %+v %v", cfg.Graveyard, err)
	}
	for _, bad := range []string{
		"[graveyard]\nmax_size = '-1GB'\n",
		"[graveyard]\nmax_size = 'bogus'\n",
		"[graveyard]\nmax_size = 123\n",
	} {
		if _, err := Parse([]byte(bad), "fixture.toml"); err == nil {
			t.Fatalf("invalid max_size accepted: %q", bad)
		} else if strings.Contains(bad, "'") && !strings.Contains(err.Error(), "graveyard.max_size") {
			t.Fatalf("error missing key name: %v", err)
		}
	}
}
