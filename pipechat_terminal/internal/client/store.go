// Package client is the Pipechat terminal client: local storage, the
// connection to the server, invitation email, and the terminal screen.
package client

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"pipechat/internal/client/history"
)

// Store is what the client remembers between runs: which server, who logged in last, and the messages shown in
// each chat (sealed, see history.go). Secrets are not in it: the group codes live in the account's encrypted
// vault on the server.
type Store struct {
	path           string
	ServerURL      string       `json:"server_url"`      // the chat server connected to last time
	SkippedVersion string       `json:"skipped_version"` // an update the person said no to for good
	LastEmail      string       `json:"last_email"`      // pre-fills the login screen
	History        *history.Log `json:"-"`               // the messages shown in each chat, kept between runs (history.jsonl)
}

// DefaultStorePath is ~/.config/pipechat/client.json (or the platform equivalent).
func DefaultStorePath() (string, error) {
	directory, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(directory, "pipechat", "client.json"), nil
}

// LoadStore reads the file, or starts an empty store when there is none.
func LoadStore(path string) (*Store, error) {
	store := &Store{path: path}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, store); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	history, err := history.Load(filepath.Join(filepath.Dir(path), "history.jsonl"))
	if err != nil {
		return nil, err
	}
	store.History = history
	return store, nil
}

func (store *Store) Save() error {
	data, _ := json.MarshalIndent(store, "", "  ")
	if err := os.MkdirAll(filepath.Dir(store.path), 0o700); err != nil {
		return err
	}
	temporary := store.path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, store.path)
}
