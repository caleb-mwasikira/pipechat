package files

import (
	"archive/tar"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync/atomic"

	"pipechat/internal/crypto"
)

const maxExtractBytes = 4 << 30 // refuse archives that unpack to more than this

// WriteArchive writes source (a file, or a folder with everything in it) as
// tar.gz, encrypted, to out. Symbolic links and special files are skipped.
//
// It returns the MD5 of what was packed, in hex: of the file's own bytes when source is one file, so it is
// the number "md5sum" gives, and of the packed folder otherwise.
func WriteArchive(out io.Writer, source string, key []byte, packed *atomic.Int64) (int, string, error) {
	root, err := filepath.Abs(source)
	if err != nil {
		return 0, "", err
	}
	encryptor, err := crypto.NewFileEncryptor(out, key)
	if err != nil {
		return 0, "", err
	}
	compressor := gzip.NewWriter(encryptor)
	folderHash := md5.New() // of the whole tar stream, used when source is a folder
	archive := tar.NewWriter(io.MultiWriter(compressor, folderHash))
	fileHash := md5.New() // of the bytes of source, used when it is a single file
	singleFile := false

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
		var content io.Reader = file
		if current == root { // the thing being shared is this one file
			singleFile = true
			content = io.TeeReader(file, fileHash)
		}
		copied, err := io.CopyN(archive, content, header.Size)
		packed.Add(copied)
		count++
		return err
	})
	if err != nil {
		return count, "", err
	}
	if err := archive.Close(); err != nil {
		return count, "", err
	}
	if err := compressor.Close(); err != nil {
		return count, "", err
	}
	if err := encryptor.Close(); err != nil {
		return count, "", err
	}
	if singleFile {
		return count, hex.EncodeToString(fileHash.Sum(nil)), nil
	}
	return count, hex.EncodeToString(folderHash.Sum(nil)), nil
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

// ExtractArchive decrypts and unpacks into targetDir, returning the names it created there and the MD5 of
// what it unpacked (see WriteArchive). When want is not empty the MD5 must be it, or nothing is written.
func ExtractArchive(encrypted io.Reader, key []byte, targetDir string, want string) (entries []string, total int64, hash string, err error) {
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, 0, "", err
	}
	staging, err := os.MkdirTemp(targetDir, ".pipechat-download-")
	if err != nil {
		return nil, 0, "", err
	}
	defer os.RemoveAll(staging)

	decryptor, err := crypto.NewFileDecryptor(encrypted, key)
	if err != nil {
		return nil, 0, "", err
	}
	decompressor, err := gzip.NewReader(decryptor)
	if err != nil {
		return nil, 0, "", errors.New("file is corrupted, truncated, or the link's key is wrong")
	}
	folderHash := md5.New() // of the tar stream
	archive := tar.NewReader(io.TeeReader(decompressor, folderHash))
	regular, topLevel, fileHash := 0, 0, ""

	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, 0, "", err
		}
		name := path.Clean(header.Name)
		if name == "." || path.IsAbs(name) || name == ".." || strings.HasPrefix(name, "../") || strings.Contains(name, `\`) {
			return nil, 0, "", fmt.Errorf("refusing unsafe path %q in the archive", header.Name)
		}
		destination := filepath.Join(staging, filepath.FromSlash(name))
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destination, 0o755); err != nil {
				return nil, 0, "", err
			}
		case tar.TypeReg:
			regular++
			if !strings.Contains(name, "/") {
				topLevel++
			}
			if err := os.MkdirAll(filepath.Dir(destination), 0o755); err != nil {
				return nil, 0, "", err
			}
			// Keep the owner's permissions, but never setuid/setgid or group/other write.
			mode := os.FileMode(header.Mode).Perm()&^0o022 | 0o600
			file, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
			if err != nil {
				return nil, 0, "", err
			}
			allowed := maxExtractBytes - total
			contentHash := md5.New()
			copied, err := io.Copy(io.MultiWriter(file, contentHash), io.LimitReader(archive, allowed+1))
			fileHash = hex.EncodeToString(contentHash.Sum(nil))
			file.Close()
			total += copied
			if err == nil && total > maxExtractBytes {
				err = errors.New("archive is too large to unpack")
			}
			if err != nil {
				return nil, 0, "", err
			}
		}
		// Anything else (links, devices) is ignored.
	}
	// Read to the end so the encryption's final chunk and the gzip checksum are verified. What is left
	// still counts towards the hash of the whole archive.
	if _, err := io.Copy(io.Discard, io.TeeReader(decompressor, folderHash)); err != nil {
		return nil, 0, "", err
	}

	// A single file is hashed by its own bytes, like md5sum; anything else by the packed archive.
	hash = hex.EncodeToString(folderHash.Sum(nil))
	if regular == 1 && topLevel == 1 {
		hash = fileHash
	}
	if want != "" && !strings.EqualFold(hash, want) {
		return nil, 0, "", fmt.Errorf("the download does not match its MD5 (expected %s, got %s); nothing was written", want, hash)
	}

	top, err := os.ReadDir(staging)
	if err != nil {
		return nil, 0, "", err
	}
	for _, entry := range top {
		if _, err := os.Lstat(filepath.Join(targetDir, entry.Name())); err == nil {
			return nil, 0, "", fmt.Errorf("%q already exists in %s; nothing was written", entry.Name(), targetDir)
		}
	}
	for _, entry := range top {
		if err := os.Rename(filepath.Join(staging, entry.Name()), filepath.Join(targetDir, entry.Name())); err != nil {
			return nil, 0, "", err
		}
		entries = append(entries, entry.Name())
	}
	return entries, total, hash, nil
}
