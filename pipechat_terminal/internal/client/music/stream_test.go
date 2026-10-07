package music

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// ---- the copy kept while a song streams ----

func cacheServer(t *testing.T, song []byte) (*Proxy, Track) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	chat := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "", time.Time{}, bytes.NewReader(song))
	}))
	t.Cleanup(chat.Close)
	proxy, err := StartProxy(fakeSource{url: chat.URL, token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(proxy.Close)
	sum := md5.Sum(song)
	return proxy, Track{ID: 5, Ext: "mp3", Size: int64(len(song)), MD5: hex.EncodeToString(sum[:])}
}

func TestAWholeStreamLeavesACopyForNextTime(t *testing.T) {
	song := bytes.Repeat([]byte("la la la "), 4000)
	proxy, track := cacheServer(t, song)
	if _, ok := Cached(track); ok {
		t.Fatal("nothing is cached before the first play")
	}
	response, err := http.Get(proxy.URL(track))
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, response.Body)
	response.Body.Close()

	// The copy is moved into place just after the last byte is sent.
	path, ok := Cached(track)
	for wait := 0; !ok && wait < 100; wait++ {
		time.Sleep(10 * time.Millisecond)
		path, ok = Cached(track)
	}
	if !ok {
		t.Fatal("a song that was streamed whole should be kept")
	}
	if got, _ := os.ReadFile(path); !bytes.Equal(got, song) {
		t.Fatal("the copy is not the song")
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".part") {
			t.Fatalf("a finished copy leaves no half-made file: %s", entry.Name())
		}
	}
}

func TestOnlyACompleteCorrectStreamIsKept(t *testing.T) {
	// Big enough that the server cannot hand it all to the network buffers before the player hangs up.
	song := bytes.Repeat([]byte("0123456789"), 1<<20)

	// A player that asks for a piece (seeking) does not make a copy.
	proxy, track := cacheServer(t, song)
	request, _ := http.NewRequest("GET", proxy.URL(track), nil)
	request.Header.Set("Range", "bytes=100-200")
	ranged, _ := http.DefaultClient.Do(request)
	io.Copy(io.Discard, ranged.Body)
	ranged.Body.Close()
	if _, ok := Cached(track); ok {
		t.Fatal("a part of a song is not a copy of it")
	}

	// A player that stops early leaves nothing.
	response, _ := http.Get(proxy.URL(track))
	io.CopyN(io.Discard, response.Body, 1000)
	response.Body.Close()
	time.Sleep(200 * time.Millisecond)
	if path, _ := CachePath(track); fileExists(path) {
		t.Fatal("a song cut short must not be kept")
	}

	// A song that is not the one the library says (wrong hash) is thrown away.
	liar, wrongTrack := cacheServer(t, song)
	wrongTrack.MD5 = strings.Repeat("0", 32)
	response, _ = http.Get(liar.URL(wrongTrack))
	io.Copy(io.Discard, response.Body)
	response.Body.Close()
	if _, ok := Cached(wrongTrack); ok {
		t.Fatal("a song that fails its MD5 must not be kept")
	}
}

func fileExists(path string) bool { _, err := os.Stat(path); return err == nil }

func TestPruneRemovesTheLeastRecentlyPlayed(t *testing.T) {
	dir := t.TempDir()
	now := time.Now()
	for index, name := range []string{"old.mp3", "middle.mp3", "new.mp3"} {
		path := filepath.Join(dir, name)
		os.WriteFile(path, bytes.Repeat([]byte("x"), 1000), 0o600)
		when := now.Add(time.Duration(index-3) * time.Hour)
		os.Chtimes(path, when, when)
	}
	os.WriteFile(filepath.Join(dir, "busy.mp3.1-1.part"), bytes.Repeat([]byte("x"), 5000), 0o600) // a copy in progress

	Prune(dir, 2000)
	if fileExists(filepath.Join(dir, "old.mp3")) || !fileExists(filepath.Join(dir, "middle.mp3")) || !fileExists(filepath.Join(dir, "new.mp3")) {
		t.Fatal("the oldest song should go first, and only as many as needed")
	}
	if !fileExists(filepath.Join(dir, "busy.mp3.1-1.part")) {
		t.Fatal("a copy still being made must be left alone")
	}
	Prune(dir, 0)
	if fileExists(filepath.Join(dir, "middle.mp3")) || fileExists(filepath.Join(dir, "new.mp3")) {
		t.Fatal("everything goes when nothing fits")
	}
}

func TestPlayingACachedSongRefreshesItsPlace(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	track := Track{ID: 9, Ext: "mp3", Size: 4}
	path, _ := CachePath(track)
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, []byte("song"), 0o600)
	old := time.Now().Add(-48 * time.Hour)
	os.Chtimes(path, old, old)
	if _, ok := Cached(track); !ok {
		t.Fatal("the copy should be found")
	}
	if info, _ := os.Stat(path); time.Since(info.ModTime()) > time.Minute {
		t.Fatal("playing a song should make it the most recent")
	}
	if _, ok := Cached(Track{ID: 9, Ext: "mp3", Size: 5}); ok {
		t.Fatal("a copy of the wrong size is not the song")
	}
}
