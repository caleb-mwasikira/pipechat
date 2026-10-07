package config

import "testing"

func TestWebsiteURL(t *testing.T) {
	for in, want := range map[string]string{
		"":                              "",
		"pipechat.kibandaskimart.com":   "https://pipechat.kibandaskimart.com", // as Laravel's APP_URL may be written
		"pipechat.kibandaskimart.com/":  "https://pipechat.kibandaskimart.com",
		"https://pipechat.example.com/": "https://pipechat.example.com",
		"http://localhost:8000":         "http://localhost:8000",
		"localhost:8000":                "http://localhost:8000",
		"192.168.1.5":                   "http://192.168.1.5",
		"example.com:8443":              "http://example.com:8443", // a port: run by hand, no certificate
		"  spaced.example.com  ":        "https://spaced.example.com",
	} {
		if got := websiteURL(in); got != want {
			t.Errorf("websiteURL(%q) = %q, want %q", in, got, want)
		}
	}
}
