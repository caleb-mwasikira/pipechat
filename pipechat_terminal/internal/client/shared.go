package client

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"pipechat/internal/client/files"
	"pipechat/internal/client/lookup"
)

// The files shared in chats that this account knows about, listed with a number and an MD5 hash, so any of
// them can be named by its number, its name or the start of its hash.

// sharedFile is one entry of the list.
type sharedFile struct {
	Number int64
	Name   string
	fileRecord
}

// sharedList is the known files in the order they were shared, numbered from 1.
func (m *Model) sharedList() []sharedFile {
	list := make([]sharedFile, 0, len(m.vault.Files))
	for name, record := range m.vault.Files {
		list = append(list, sharedFile{Name: name, fileRecord: record})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].At != list[j].At {
			return list[i].At < list[j].At
		}
		return list[i].Name < list[j].Name
	})
	for index := range list {
		list[index].Number = int64(index + 1)
	}
	return list
}

// findShared resolves what the person typed: a number, a name, or the start of a hash.
func (m *Model) findShared(query string) (sharedFile, error) {
	list := m.sharedList()
	if len(list) == 0 {
		return sharedFile{}, fmt.Errorf("no shared files yet; use /upload PATH")
	}
	items := make([]lookup.Item, len(list))
	for index, file := range list {
		items[index] = lookup.Item{Number: file.Number, Hash: file.Hash, Names: []string{file.Name}}
	}
	index, err := lookup.Find(items, query)
	if err != nil {
		return sharedFile{}, fmt.Errorf("%w; see /files", err)
	}
	return list[index], nil
}

// showShared prints the list: number, hash, name, then the details.
func (m *Model) showShared(current *groupState) {
	list := m.sharedList()
	if len(list) == 0 {
		m.notice(current, "no shared files yet; use /upload PATH")
		return
	}
	m.notice(current, "Shared files. Get one with /download NAME, /download HASH (the start is enough) or /download NUMBER:")
	for _, file := range list {
		who := "you"
		if file.From != "" {
			who = "@" + file.From
		}
		m.notice(current, fmt.Sprintf("  #%-3d %s  %s  (%s, from %s, %s)", file.Number, lookup.Short(file.Hash), file.Name,
			files.FormatSize(file.Size), who, time.Unix(file.At, 0).Format("2006-01-02 15:04")))
	}
}

// hashNote is " md5 9e107d9d" when a hash is known, for messages.
func hashNote(hash string) string {
	if hash == "" {
		return ""
	}
	return " md5 " + lookup.Short(hash)
}

// downloadMessage says how a download went.
func downloadMessage(result files.Downloaded) string {
	text := fmt.Sprintf("downloaded %s (%s) to %s: %s", result.Name, files.FormatSize(result.Bytes), result.Dir, strings.Join(result.Entries, ", "))
	switch {
	case result.Checked:
		return text + fmt.Sprintf(" — md5 %s matches", result.Hash)
	case result.Hash != "":
		return text + fmt.Sprintf(" — md5 %s (the sender gave none to check against)", result.Hash)
	}
	return text
}
