package music

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type fakeSource struct{ url, token string }

func (f fakeSource) URL() string   { return f.url }
func (f fakeSource) Token() string { return f.token }

func TestProxyStreamsWithTheTokenAndSeeks(t *testing.T) {
	song := bytes.Repeat([]byte("0123456789"), 20)
	var seenAuthorization string
	chatServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuthorization = r.Header.Get("Authorization")
		if r.URL.Path != "/api/music/7/file" {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(song)) // answers Range like the real server
	}))
	defer chatServer.Close()

	source := fakeSource{url: chatServer.URL, token: "secret-token"}
	proxy, err := StartProxy(source)
	if err != nil {
		t.Fatal(err)
	}
	defer proxy.Close()
	if !proxy.For(source) {
		t.Fatal("the proxy should know its server")
	}
	address := proxy.URL(Track{ID: 7, Ext: "MP3"})
	if !strings.HasPrefix(address, "http://127.0.0.1:") || !strings.HasSuffix(address, "/7.mp3") {
		t.Fatalf("address %q", address)
	}
	if strings.Contains(address, "secret-token") {
		t.Fatal("the token must not be in the address a player is given")
	}

	response, err := http.Get(address)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if !bytes.Equal(body, song) || seenAuthorization != "Bearer secret-token" {
		t.Fatalf("got %d bytes, authorization %q", len(body), seenAuthorization)
	}

	request, _ := http.NewRequest("GET", address, nil)
	request.Header.Set("Range", "bytes=10-19")
	ranged, _ := http.DefaultClient.Do(request)
	part, _ := io.ReadAll(ranged.Body)
	ranged.Body.Close()
	if ranged.StatusCode != http.StatusPartialContent || string(part) != "0123456789" {
		t.Fatalf("range: %d %q", ranged.StatusCode, part)
	}

	// Only whoever has the secret address can use it.
	wrong := strings.Replace(address, "/7.mp3", "/../7.mp3", 1)
	for _, bad := range []string{strings.Replace(address, proxy.secret, "nope", 1), wrong} {
		if res, err := http.Get(bad); err == nil && res.StatusCode == http.StatusOK {
			t.Errorf("%q should not stream", bad)
		}
	}
	if post, _ := http.Post(address, "text/plain", nil); post != nil && post.StatusCode == http.StatusOK {
		t.Error("only GET streams")
	}
}
