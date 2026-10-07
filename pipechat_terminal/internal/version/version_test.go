package version

import "testing"

func TestCompare(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.1.0", -1},
		{"1.10.0", "1.9.0", 1}, // numbers, not text
		{"v1.2.3", "1.2.3", 0},
		{"2.0.0", "1.99.99", 1},
		{"1.2.3-beta.1", "1.2.3", -1}, // a pre-release comes before its release
		{"1.2.3", "1.2.3-rc.1", 1},
		{"1.2.3-beta.2", "1.2.3-beta.10", -1},
		{"1.2.3-alpha", "1.2.3-beta", -1},
		{"1.2.3-beta", "1.2.3-beta.1", -1},
		{"dev", "0.0.1", -1},
		{"1.0.0", "dev", 1},
		{"dev", "dev", 0},
	} {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
		if got := Compare(c.b, c.a); got != -c.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", c.b, c.a, got, -c.want)
		}
	}
}

func TestValid(t *testing.T) {
	for _, good := range []string{"0.0.1", "1.2.3", "v1.2.3", "1.2.3-beta.1"} {
		if !Valid(good) {
			t.Errorf("%q should be valid", good)
		}
	}
	for _, bad := range []string{"", "dev", "1.2", "1.2.3.4", "a.b.c", "1.2.-3"} {
		if Valid(bad) {
			t.Errorf("%q should not be valid", bad)
		}
	}
}
