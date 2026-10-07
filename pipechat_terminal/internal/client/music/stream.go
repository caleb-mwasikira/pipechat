package music

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
)

// Proxy lets any player stream a track without being given the login token. It listens on
// this computer only, under a secret path, and passes each request to the chat server with the
// token added. Seeking works, because the Range header goes through.
type Source interface {
	URL() string   // the chat server's address
	Token() string // the login token
}

type Proxy struct {
	source   Source
	secret   string
	listener net.Listener
}

// StartProxy begins listening for a player.
func StartProxy(source Source) (*Proxy, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}
	raw := make([]byte, 16)
	rand.Read(raw)
	proxy := &Proxy{source: source, secret: hex.EncodeToString(raw), listener: listener}
	go http.Serve(listener, proxy)
	return proxy, nil
}

// For says whether the proxy talks to this chat server.
func (p *Proxy) For(source Source) bool { return p.source == source }

// URL is the address a player can open to hear a track.
func (p *Proxy) URL(track Track) string {
	return fmt.Sprintf("http://%s/%s/%d.%s", p.listener.Addr(), p.secret, track.ID, strings.ToLower(track.Ext))
}

func (p *Proxy) Close() { p.listener.Close() }

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	name, found := strings.CutPrefix(r.URL.Path, "/"+p.secret+"/")
	id, _, _ := strings.Cut(name, ".")
	if !found || r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		http.NotFound(w, r)
		return
	}
	request, err := http.NewRequestWithContext(r.Context(), "GET", p.source.URL()+"/api/music/"+id+"/file", nil)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	request.Header.Set("Authorization", "Bearer "+p.source.Token())
	for _, header := range []string{"Range", "If-Range"} {
		if value := r.Header.Get(header); value != "" {
			request.Header.Set(header, value)
		}
	}
	response, err := (&http.Client{}).Do(request) // no overall timeout: a song plays for minutes
	if err != nil {
		http.Error(w, "the chat server did not answer", http.StatusBadGateway)
		return
	}
	defer response.Body.Close()
	for _, header := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "Last-Modified"} {
		if value := response.Header.Get(header); value != "" {
			w.Header().Set(header, value)
		}
	}
	w.WriteHeader(response.StatusCode)
	io.Copy(w, response.Body)
}
