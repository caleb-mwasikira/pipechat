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
	"os"
	"path/filepath"
	"pipechat/internal/client/music"
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

// Emit lets uploads and downloads report progress to the screen (it is files.Env).
func (s *Server) Emit(message any) { s.emit(message) }

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

func asAPIError(err error, target **apiError) bool { return errors.As(err, target) }

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

// Profile is what the server knows about a user that others may see.
type Profile struct {
	Handle string `json:"handle"`
	PubKey string `json:"pubkey"`
	Name   string `json:"name"`
	Bio    string `json:"bio"`
	Joined int64  `json:"joined"` // unix seconds
	Online bool   `json:"online"`
}

func (s *Server) LookupProfile(handle string) (Profile, error) {
	var profile Profile
	err := s.call("GET", "/api/users/"+handle, nil, &profile)
	return profile, err
}

func (s *Server) LookupUser(handle string) (publicKey string, err error) {
	profile, err := s.LookupProfile(handle)
	return profile.PubKey, err
}

// SetBio changes your own bio.
func (s *Server) SetBio(bio string) error {
	return s.call("PUT", "/api/profile", map[string]any{"bio": bio}, nil)
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
	ServerKey  string `json:"server_key"` // the server's public key, when it holds group keys
	NeedEscrow []struct {
		Group int64  `json:"group"`
		GID   string `json:"gid"`
	} `json:"need_escrow"` // groups whose key the server should be given a copy of
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

// SyncEscrow gives the server a copy of a group's key, sealed so only the server can open it.
func (s *Server) SyncEscrow(webGroup int64, sealed []byte) error {
	return s.call("POST", "/api/sync/escrow", map[string]any{"group": webGroup, "sealed": sealed}, nil)
}

// KickMember removes someone from a website group; only the group's creator may.
func (s *Server) KickMember(groupID string, handle string) error {
	return s.call("POST", "/api/sync/kick", map[string]any{"gid": groupID, "handle": handle}, nil)
}

func (s *Server) Music() ([]music.Track, error) {
	var tracks []music.Track
	err := s.call("GET", "/api/music", nil, &tracks)
	return tracks, err
}

// DownloadTrack saves a track to path. It is written to a temporary name first, so an interrupted
// download is never mistaken for a whole song later.
func (s *Server) DownloadTrack(id int64, path string) error {
	request, err := http.NewRequest("GET", s.url+"/api/music/"+strconv.FormatInt(id, 10)+"/file", nil)
	if err != nil {
		return err
	}
	request.Header.Set("Authorization", "Bearer "+s.Token())
	// A song can take longer to arrive than the 30 seconds that suit small requests.
	response, err := (&http.Client{Timeout: 10 * time.Minute}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		var failure struct{ Error string }
		json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&failure)
		if failure.Error == "" {
			failure.Error = response.Status
		}
		return &apiError{Status: response.StatusCode, Message: failure.Error}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary := path + ".part"
	file, err := os.OpenFile(temporary, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, err = io.Copy(file, response.Body)
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(temporary)
		return err
	}
	return os.Rename(temporary, path)
}

// OpenGroup is a public group: anyone can join it by name.
type OpenGroup struct {
	Name    string `json:"name"`
	Creator string `json:"creator"`
	Code    string `json:"code"` // only when asking for one group
}

func (s *Server) OpenGroups() ([]OpenGroup, error) {
	var groups []OpenGroup
	err := s.call("GET", "/api/open", nil, &groups)
	return groups, err
}

func (s *Server) GetOpenGroup(name string) (OpenGroup, error) {
	var group OpenGroup
	err := s.call("GET", "/api/open/"+url.PathEscape(name), nil, &group)
	return group, err
}

// ListOpenGroup makes a group you are in public under a name.
func (s *Server) ListOpenGroup(name string, code string) error {
	return s.call("POST", "/api/open", map[string]any{"name": name, "code": code}, nil)
}

func (s *Server) UnlistOpenGroup(name string) error {
	return s.call("DELETE", "/api/open/"+url.PathEscape(name), nil, nil)
}

// Online lists the handles that have a chat open right now.
func (s *Server) Online(groupID string) ([]string, error) {
	var handles []string
	err := s.call("GET", "/api/groups/"+groupID+"/online", nil, &handles)
	return handles, err
}

// ReportShare tells the server a file was shared in a chat, so a website group's page can list it.
// It is best effort: chats that are not website groups answer 404, which is fine.
func (s *Server) ReportShare(groupID string, name string, size int64) {
	s.call("POST", "/api/sync/shares", map[string]any{"gid": groupID, "name": name, "size": size}, nil)
}
