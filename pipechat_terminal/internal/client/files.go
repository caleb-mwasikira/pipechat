package client

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"pipechat/internal/crypto"
)

// Sharing files and folders. An upload is a tar.gz of the path, encrypted with a
// fresh random key, streamed to the server. The share link is
//
//	http(s)://server:port/api/files/<id>#<key>
//
// The key is in the fragment, which is never sent to the server, and the link
// itself only travels inside end-to-end encrypted chat messages.

const maxExtractBytes = 4 << 30 // refuse archives that unpack to more than this

var (
	shortNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,40}$`)
	linkPathPattern  = regexp.MustCompile(`^/api/files/[0-9a-f]{32}$`)
)

// Events from the transfer goroutines.
type TransferProgress struct{ Text string }
type UploadDone struct {
	Name  string
	Link  string
	Bytes int64
	Files int
	Err   error
}
type DownloadDone struct {
	Name    string
	Dir     string
	Entries []string
	Bytes   int64
	Err     error
}

// ShortName turns a file name into a valid short name.
func ShortName(name string) string {
	name = regexp.MustCompile(`[^A-Za-z0-9._-]`).ReplaceAllString(name, "_")
	if len(name) > 40 {
		name = name[:40]
	}
	return name
}

// SplitArguments splits a command line on spaces, keeping "quoted parts" together.
func SplitArguments(text string) []string {
	var arguments []string
	var current strings.Builder
	var quote rune
	started := false
	for _, character := range text {
		switch {
		case quote != 0 && character == quote:
			quote = 0
		case quote != 0:
			current.WriteRune(character)
		case character == '"' || character == '\'':
			quote, started = character, true
		case character == ' ' || character == '\t':
			if started || current.Len() > 0 {
				arguments = append(arguments, current.String())
				current.Reset()
				started = false
			}
		default:
			current.WriteRune(character)
		}
	}
	if started || current.Len() > 0 {
		arguments = append(arguments, current.String())
	}
	return arguments
}

func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

func formatSize(bytes int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	size := float64(bytes)
	unit := 0
	for size >= 1024 && unit < len(units)-1 {
		size /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%d B", bytes)
	}
	return fmt.Sprintf("%.1f %s", size, units[unit])
}

// ---- upload ----

// UploadPath packs, encrypts and uploads a file or folder, then returns the share link.
func (s *Server) UploadPath(source string, name string) UploadDone {
	result := UploadDone{Name: name}
	key, keyText, err := crypto.NewFileKey()
	if err != nil {
		result.Err = err
		return result
	}

	var packed atomic.Int64
	reader, writer := io.Pipe()
	go func() {
		var err error
		result.Files, err = writeArchive(writer, source, key, &packed)
		writer.CloseWithError(err)
	}()

	stop := make(chan struct{})
	defer close(stop)
	go s.reportProgress("uploading "+name, &packed, stop)

	request, _ := http.NewRequest("POST", s.url+"/api/files", reader)
	request.Header.Set("Content-Type", "application/octet-stream")
	request.Header.Set("Authorization", "Bearer "+s.Token())
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		reader.CloseWithError(err)
		result.Err = err
		return result
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		var failure struct{ Error string }
		json.NewDecoder(io.LimitReader(response.Body, 4096)).Decode(&failure)
		result.Err = fmt.Errorf("server said: %s (%s)", failure.Error, response.Status)
		return result
	}
	var stored struct{ ID string }
	if err := json.NewDecoder(response.Body).Decode(&stored); err != nil || stored.ID == "" {
		result.Err = errors.New("unexpected answer from the server")
		return result
	}
	result.Bytes = packed.Load()
	result.Link = s.url + "/api/files/" + stored.ID + "#" + keyText
	return result
}

func (s *Server) reportProgress(label string, counter *atomic.Int64, stop chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			s.emit(TransferProgress{Text: label + ": " + formatSize(counter.Load())})
		case <-stop:
			return
		}
	}
}

// writeArchive writes source (a file, or a folder with everything in it) as
// tar.gz, encrypted, to out. Symbolic links and special files are skipped.
func writeArchive(out io.Writer, source string, key []byte, packed *atomic.Int64) (int, error) {
	root, err := filepath.Abs(source)
	if err != nil {
		return 0, err
	}
	encryptor, err := crypto.NewFileEncryptor(out, key)
	if err != nil {
		return 0, err
	}
	compressor := gzip.NewWriter(encryptor)
	archive := tar.NewWriter(compressor)

	count := 0
	baseName := filepath.Base(root)
	err = filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return nil
		}
		relative, _ := filepath.Rel(root, current)
		name := baseName
		if relative != "." {
			name = path.Join(baseName, filepath.ToSlash(relative))
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = name
		header.Uid, header.Gid, header.Uname, header.Gname = 0, 0, "", ""
		if info.IsDir() {
			header.Name += "/"
			return archive.WriteHeader(header)
		}
		if err := archive.WriteHeader(header); err != nil {
			return err
		}
		file, err := os.Open(current)
		if err != nil {
			return err
		}
		defer file.Close()
		copied, err := io.CopyN(archive, file, header.Size)
		packed.Add(copied)
		count++
		return err
	})
	if err != nil {
		return count, err
	}
	if err := archive.Close(); err != nil {
		return count, err
	}
	if err := compressor.Close(); err != nil {
		return count, err
	}
	return count, encryptor.Close()
}

// ---- download ----

// ParseLink checks a share link and returns the address to fetch and the key.
func ParseLink(link string) (address string, key []byte, err error) {
	parsed, err := url.Parse(strings.TrimSpace(link))
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || !linkPathPattern.MatchString(parsed.Path) {
		return "", nil, errors.New("not a valid file link")
	}
	key, err = crypto.ParseFileKey(parsed.Fragment)
	if err != nil {
		return "", nil, err
	}
	parsed.Fragment, parsed.RawQuery = "", ""
	return parsed.String(), key, nil
}

// DownloadLink fetches, decrypts and unpacks a shared file or folder into
// targetDir. Nothing is written there unless the whole download verifies, and
// nothing existing is overwritten.
func (s *Server) DownloadLink(link string, targetDir string, name string) DownloadDone {
	result := DownloadDone{Name: name, Dir: targetDir}
	address, key, err := ParseLink(link)
	if err != nil {
		result.Err = err
		return result
	}
	response, err := http.Get(address)
	if err != nil {
		result.Err = err
		return result
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		result.Err = errors.New("the file is gone from the server (files expire)")
		return result
	}
	if response.StatusCode != http.StatusOK {
		result.Err = fmt.Errorf("server answered %s", response.Status)
		return result
	}

	var received atomic.Int64
	stop := make(chan struct{})
	defer close(stop)
	go s.reportProgress("downloading "+name, &received, stop)

	counted := &countingReader{reader: response.Body, total: &received}
	result.Entries, result.Bytes, result.Err = ExtractArchive(counted, key, targetDir)
	return result
}

type countingReader struct {
	reader io.Reader
	total  *atomic.Int64
}

func (c *countingReader) Read(buffer []byte) (int, error) {
	count, err := c.reader.Read(buffer)
	c.total.Add(int64(count))
	return count, err
}

// ExtractArchive decrypts and unpacks into targetDir, returning the names it created there.
func ExtractArchive(encrypted io.Reader, key []byte, targetDir string) (entries []string, total int64, err error) {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, 0, err
	}
	staging, err := os.MkdirTemp(targetDir, ".pipechat-download-")
	if err != nil {
		return nil, 0, err
	}
	defer os.RemoveAll(staging)

	decryptor, err := crypto.NewFileDecryptor(encrypted, key)
	if err != nil {
		return nil, 0, err
	}
	decompressor, err := gzip.NewReader(decryptor)
	if err != nil {
		return nil, 0, errors.New("file is corrupted, truncated, or the link's key is wrong")
	}
	archive := tar.NewReader(decompressor)

	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, err
		}
		name := path.Clean(header.Name)
		if name == "." || path.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") || strings.Contains(name, `\`) {
			return nil, 0, fmt.Errorf("refusing unsafe path %q in the archive", header.Name)
		}
		destination := filepath.Join(staging, filepath.FromSlash(name))
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return nil, 0, err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				return nil, 0, err
			}
			// Keep the owner's permissions, but never setuid/setgid or group/other write.
			mode := os.FileMode(header.Mode).Perm()&^0o022 | 0o600
			file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
			if err != nil {
				return nil, 0, err
			}
			allowed := maxExtractBytes - total
			copied, err := io.Copy(file, io.LimitReader(archive, allowed+1))
			file.Close()
			total += copied
			if err == nil && total > maxExtractBytes {
				err = errors.New("archive is too large to unpack")
			}
			if err != nil {
				return nil, 0, err
			}
		}
		// Anything else (links, devices) is ignored.
	}
	// Read to the end so the encryption's final chunk and the gzip checksum are verified.
	if _, err := io.Copy(io.Discard, decompressor); err != nil {
		return nil, 0, err
	}

	top, err := os.ReadDir(staging)
	if err != nil {
		return nil, 0, err
	}
	for _, entry := range top {
		if _, err := os.Lstat(filepath.Join(targetDir, entry.Name())); err == nil {
			return nil, 0, fmt.Errorf("%q already exists in %s; nothing was written", entry.Name(), targetDir)
		}
	}
	for _, entry := range top {
		if err := os.Rename(filepath.Join(staging, entry.Name()), filepath.Join(targetDir, entry.Name())); err != nil {
			return nil, 0, err
		}
		entries = append(entries, entry.Name())
	}
	return entries, total, nil
}
