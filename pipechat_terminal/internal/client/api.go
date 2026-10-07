package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Server is the chat server we talk to: its address, our login session, and one
// method per request. Make one with NewServer, then call Connect to keep the
// connection alive. Stop ends it, for example to connect to another server.
type Server struct {
	url  string
	http *http.Client
	jobs chan func() // requests after logging in, run one at a time and in order

	report  func(tea.Msg) // hands events to the screen; Connect sets it
	done    chan struct{} // closed by Stop
	stopped atomic.Bool

	mutex      sync.Mutex // guards token and stopSocket
	token      string     // the session; empty when logged out
	stopSocket context.CancelFunc
	sessionSet chan struct{} // signalled when a session starts, to open the WebSocket
}

// ParseServerURL checks what the user typed. With no http:// or https://, a plain
// domain name such as "chat.example.com" means https://, because the login sends a
// password; an address with a port ("host:7777"), an IP address or localhost means
// http://, as that is how a server run by hand answers.
func ParseServerURL(address string) (string, error) {
	address = strings.TrimSpace(address)
	if !strings.Contains(address, "://") {
		scheme := "http"
		if host, err := url.Parse("//" + address); err == nil && host.Port() == "" && isDomainName(host.Hostname()) {
			scheme = "https"
		}
		address = scheme + "://" + address
	}
	address = strings.TrimRight(address, "/")
	parsed, err := url.Parse(address)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.Path != "" {
		return "", errors.New("that is not a server address; try chat.example.com or 192.168.1.5:7777")
	}
	return address, nil
}

// isDomainName is true for names like "chat.example.com", false for "localhost" and IP addresses.
func isDomainName(host string) bool {
	return strings.Contains(host, ".") && net.ParseIP(host) == nil
}

// NewServer makes a Server for an address that ParseServerURL accepted.
func NewServer(address string) *Server {
	s := &Server{
		url:        address,
		http:       &http.Client{Timeout: 30 * time.Second},
		jobs:       make(chan func(), 256),
		report:     func(tea.Msg) {},
		done:       make(chan struct{}),
		sessionSet: make(chan struct{}, 1),
	}
	go func() {
		for {
			select {
			case job := <-s.jobs:
				job()
			case <-s.done:
				return
			}
		}
	}()
	return s
}

// Stop ends the connection loop and the WebSocket. Nothing more is reported afterwards.
func (s *Server) Stop() {
	if s.stopped.Swap(true) {
		return
	}
	close(s.done)
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.stopSocket != nil {
		s.stopSocket()
	}
}

// emit hands an event to the screen, unless the server was stopped.
func (s *Server) emit(message tea.Msg) {
	if !s.stopped.Load() {
		s.report(message)
	}
}

func (s *Server) URL() string { return s.url }

func (s *Server) Token() string {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.token
}

// Logout forgets the session and closes its WebSocket.
func (s *Server) Logout() { s.endSession(s.Token()) }

func (s *Server) startSession(token string) {
	s.mutex.Lock()
	s.token = token
	s.mutex.Unlock()
	select {
	case s.sessionSet <- struct{}{}:
	default:
	}
}

// endSession forgets the session, if it is still the given one, and closes its WebSocket.
func (s *Server) endSession(token string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.token == token && token != "" {
		s.token = ""
		if s.stopSocket != nil {
			s.stopSocket()
		}
	}
}

// enqueue runs a request after the ones queued before it, so a message never
// overtakes the join before it. The job reports its own result to the screen.
func (s *Server) enqueue(job func()) error {
	select {
	case s.jobs <- job:
		return nil
	default:
		return fmt.Errorf("too many requests waiting")
	}
}

// ---- the API, one method per request ----

type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string { return e.Message }

// Conflict reports whether the server answered 409 (already exists).
func Conflict(err error) bool {
	var failure *apiError
	return errors.As(err, &failure) && failure.Status == http.StatusConflict
}

// What the server answers to a login.
type LoginResult struct {
	Token  string   `json:"token"`
	Handle string   `json:"handle"`
	Vault  []byte   `json:"vault"`
	PubKey string   `json:"pubkey"`
	Peers  []string `json:"peers"` // handles that asked for a direct chat
}

// Event is one message pushed on the WebSocket (and returned when joining a group).
type Event struct {
	Type   string `json:"t"` // "msg" or "dm"
	GID    string `json:"gid"`
	CT     []byte `json:"ct"`
	Seq    uint64 `json:"seq"`
	TS     int64  `json:"ts"`
	Handle string `json:"handle"` // dm: who wants to chat
}

// call makes one request with the session token. A 401 answer ends the session,
// so the connection loop logs in again.
func (s *Server) call(method string, path string, body any, into any) error {
	var reader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		reader = bytes.NewReader(data)
	}
	request, err := http.NewRequest(method, s.url+path, reader)
	if err != nil {
		return err
	}
	token := s.Token()
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := s.http.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode >= 300 {
		var failure struct{ Error string }
		json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&failure)
		if failure.Error == "" {
			failure.Error = response.Status
		}
		if response.StatusCode == http.StatusUnauthorized && token != "" {
			s.endSession(token)
		}
		return &apiError{Status: response.StatusCode, Message: failure.Error}
	}
	if into != nil {
		return json.NewDecoder(response.Body).Decode(into)
	}
	return nil
}

// Login logs in with the website's email and password, starts a session and
// returns the account's sealed vault (empty the first time).
func (s *Server) Login(email string, password string) (LoginResult, error) {
	var result LoginResult
	err := s.call("POST", "/api/auth/login", map[string]any{"email": email, "password": password}, &result)
	if err == nil {
		s.startSession(result.Token)
	}
	return result, err
}

// SaveIdentity stores a new public key with the vault sealed for it.
func (s *Server) SaveIdentity(publicKey string, vault []byte) error {
	return s.call("PUT", "/api/identity", map[string]any{"pubkey": publicKey, "vault": vault}, nil)
}

func (s *Server) SaveVault(vault []byte) error {
	return s.call("PUT", "/api/vault", map[string]any{"vault": vault}, nil)
}

func (s *Server) LookupUser(handle string) (publicKey string, err error) {
	var result struct{ PubKey string }
	err = s.call("GET", "/api/users/"+handle, nil, &result)
	return result.PubKey, err
}

func (s *Server) NotifyDirect(handle string) error {
	return s.call("POST", "/api/direct/"+handle, nil, nil)
}

func (s *Server) ClearDirect(handle string) error {
	return s.call("DELETE", "/api/direct/"+handle, nil, nil)
}

func (s *Server) CreateGroup(groupID string, joinToken string) error {
	return s.call("POST", "/api/groups", map[string]any{"gid": groupID, "tok": joinToken}, nil)
}

// JoinGroup subscribes to the group and returns the messages after since.
func (s *Server) JoinGroup(groupID string, joinToken string, since uint64) ([]Event, error) {
	var result struct{ Messages []Event }
	err := s.call("POST", "/api/groups/"+groupID+"/join", map[string]any{"tok": joinToken, "since": since}, &result)
	return result.Messages, err
}

func (s *Server) LeaveGroup(groupID string) error {
	return s.call("DELETE", "/api/groups/"+groupID+"/membership", nil, nil)
}

func (s *Server) SendMessage(groupID string, ciphertext []byte) error {
	return s.call("POST", "/api/groups/"+groupID+"/messages", map[string]any{"ct": ciphertext}, nil)
}

// InviteByEmail asks the server to email the group's invite code to someone.
// Unlike sharing the code yourself, this lets the server see the code.
func (s *Server) InviteByEmail(email string, code string) error {
	return s.call("POST", "/api/invitations", map[string]any{"email": email, "code": code}, nil)
}

// What the website wants this terminal to do about its groups.
type SyncResult struct {
	Create []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"create"` // groups I own that must be created
	Grant []struct {
		Group  int64  `json:"group"`
		GID    string `json:"gid"`
		Handle string `json:"handle"`
		PubKey string `json:"pubkey"`
	} `json:"grant"` // members waiting for a group key I may hold
	Incoming []struct {
		Group   int64  `json:"group"`
		Name    string `json:"name"`
		From    string `json:"from"`
		FromKey string `json:"from_key"`
		Sealed  []byte `json:"sealed"`
	} `json:"incoming"` // group keys sent to me
}

func (s *Server) Sync() (SyncResult, error) {
	var result SyncResult
	err := s.call("GET", "/api/sync", nil, &result)
	return result, err
}

func (s *Server) SyncGroupCreated(webGroup int64, groupID string) error {
	return s.call("POST", "/api/sync/groups", map[string]any{"group": webGroup, "gid": groupID}, nil)
}

func (s *Server) SyncGrant(webGroup int64, handle string, sealed []byte) error {
	return s.call("POST", "/api/sync/grants", map[string]any{"group": webGroup, "handle": handle, "sealed": sealed}, nil)
}

func (s *Server) SyncGrantReceived(webGroup int64) error {
	return s.call("DELETE", "/api/sync/grants/"+strconv.FormatInt(webGroup, 10), nil, nil)
}
