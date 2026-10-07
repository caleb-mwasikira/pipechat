package client

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// The messages shown in each chat are kept on this computer, so they are still there the next time the
// terminal starts. They are stored as the server sent them, sealed with the group's key: the file
// holds no readable text, and the key is only in the account's vault.
//
// It is a log of lines, one JSON object per line, so saving a message is one small append.

const historyKept = 1000 // messages kept per chat

type historyRecord struct {
	Key   string `json:"k"` // "handle/group id"
	Seq   uint64 `json:"s"`
	TS    int64  `json:"t,omitempty"`
	CT    []byte `json:"c,omitempty"`
	Clear bool   `json:"x,omitempty"` // /clear: everything up to Seq is no longer shown
}

type History struct {
	mutex   sync.Mutex
	path    string
	records map[string][]historyRecord // oldest first, without what was cleared
	last    map[string]uint64          // newest sequence number ever seen, even if cleared
	lines   int                        // lines in the file, to know when it is worth tidying
}

// LoadHistory reads the file, or starts empty when there is none.
func LoadHistory(path string) (*History, error) {
	h := &History{path: path, records: map[string][]historyRecord{}, last: map[string]uint64{}}
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return h, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		var record historyRecord
		if json.Unmarshal(scanner.Bytes(), &record) != nil {
			continue // a half-written last line, or damage: skip it
		}
		h.lines++
		h.apply(record)
	}
	if h.lines > 3*h.total()+200 {
		h.rewrite() // best effort; the log still works if this fails
	}
	return h, nil
}

// apply updates the in-memory view with one record. Must be called with the mutex held or before sharing.
func (h *History) apply(record historyRecord) {
	h.last[record.Key] = max(h.last[record.Key], record.Seq)
	list := h.records[record.Key]
	if record.Clear {
		kept := list[:0]
		for _, old := range list {
			if old.Seq > record.Seq {
				kept = append(kept, old)
			}
		}
		h.records[record.Key] = kept
		return
	}
	position := sort.Search(len(list), func(i int) bool { return list[i].Seq >= record.Seq })
	if position < len(list) && list[position].Seq == record.Seq {
		return // already have it
	}
	list = append(list, historyRecord{})
	copy(list[position+1:], list[position:])
	list[position] = record
	if len(list) > historyKept {
		list = list[len(list)-historyKept:]
	}
	h.records[record.Key] = list
}

func (h *History) total() int {
	count := 0
	for _, list := range h.records {
		count += len(list)
	}
	return count
}

// Last is the newest message already seen in a chat: what to ask the server for more than.
func (h *History) Last(key string) uint64 {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	return h.last[key]
}

// Messages are a chat's saved messages, oldest first.
func (h *History) Messages(key string) []historyRecord {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	return append([]historyRecord(nil), h.records[key]...)
}

// Add saves one message. It is ignored if it is already there.
func (h *History) Add(key string, seq uint64, ts int64, ciphertext []byte) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	for _, old := range h.records[key] {
		if old.Seq == seq {
			return nil
		}
	}
	record := historyRecord{Key: key, Seq: seq, TS: ts, CT: ciphertext}
	h.apply(record)
	return h.append(record)
}

// Clear hides everything seen so far, but remembers how far we got, so the server does not send it again.
func (h *History) Clear(key string) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	record := historyRecord{Key: key, Seq: h.last[key], Clear: true}
	h.apply(record)
	return h.append(record)
}

// Forget removes a chat completely, for when we leave it.
func (h *History) Forget(key string) error {
	h.mutex.Lock()
	defer h.mutex.Unlock()
	delete(h.records, key)
	delete(h.last, key)
	return h.rewrite()
}

func (h *History) append(record historyRecord) error {
	if err := os.MkdirAll(filepath.Dir(h.path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(h.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	line, _ := json.Marshal(record)
	if _, err := file.Write(append(line, '\n')); err != nil {
		return err
	}
	h.lines++
	return nil
}

// rewrite saves the current state as a fresh, short file. Must be called with the mutex held or before sharing.
func (h *History) rewrite() error {
	var out bytes.Buffer
	keys := make([]string, 0, len(h.last))
	for key := range h.last {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	lines := 0
	for _, key := range keys {
		list := h.records[key]
		if len(list) == 0 {
			// Nothing to show, but remember how far this chat got.
			line, _ := json.Marshal(historyRecord{Key: key, Seq: h.last[key], Clear: true})
			out.Write(line)
			out.WriteByte('\n')
			lines++
		}
		for _, record := range list {
			line, _ := json.Marshal(record)
			out.Write(line)
			out.WriteByte('\n')
			lines++
		}
	}
	if err := os.MkdirAll(filepath.Dir(h.path), 0o700); err != nil {
		return err
	}
	temporary := h.path + ".tmp"
	if err := os.WriteFile(temporary, out.Bytes(), 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, h.path); err != nil {
		return err
	}
	h.lines = lines
	return nil
}
