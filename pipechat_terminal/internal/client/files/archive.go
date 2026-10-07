package files

import (
	"archive/tar"
	"compress/gzip"
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
func WriteArchive(out io.Writer, source string, key []byte, packed *atomic.Int64) (int, error) {
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
