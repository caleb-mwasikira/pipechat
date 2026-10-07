package client

import "testing"

func TestParseServerURL(t *testing.T) {
	good := map[string]string{
		"192.168.1.5:7777":         "http://192.168.1.5:7777",
		" localhost:7777/ ":        "http://localhost:7777",
		"https://chat.example.com": "https://chat.example.com",
		"http://127.0.0.1:7777":    "http://127.0.0.1:7777",
		"chat.example.com":         "https://chat.example.com",
		" Chat.Example.com/ ":      "https://Chat.Example.com",
		"chat.example.com:7777":    "http://chat.example.com:7777",
		"192.168.1.5":              "http://192.168.1.5",
		"localhost":                "http://localhost",
		"http://chat.example.com":  "http://chat.example.com",
	}
	for typed, want := range good {
		if got, err := ParseServerURL(typed); err != nil || got != want {
			t.Errorf("%q: got %q, %v; want %q", typed, got, err, want)
		}
	}
	for _, bad := range []string{"", "   ", "ftp://example.com", "http://", "https://host/some/path", "http://:7777"} {
		if got, err := ParseServerURL(bad); err == nil {
			t.Errorf("accepted %q as %q", bad, got)
		}
	}
}
