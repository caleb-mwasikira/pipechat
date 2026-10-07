package crypto

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"io"
)

// Files are encrypted as a stream of independently authenticated chunks:
//
//	"TCF1" || ( uint32 length || AES-256-GCM(chunk) )*
//
// The nonce holds the chunk number and whether it is the last chunk, so chunks
// cannot be reordered, dropped from the end or replayed. Every file has its own
// random key, which travels only inside the share link.

const (
	streamChunk = 64 << 10
	streamMagic = "TCF1"
	fileKeyLen  = 32
)

var errCorrupt = errors.New("file is corrupted, truncated, or the link's key is wrong")

// NewFileKey returns a random key, and its text form for share links.
func NewFileKey() (key []byte, text string, err error) {
	key = make([]byte, fileKeyLen)
	if _, err = rand.Read(key); err != nil {
		return nil, "", err
	}
	return key, base64.RawURLEncoding.EncodeToString(key), nil
}

func ParseFileKey(text string) ([]byte, error) {
	key, err := base64.RawURLEncoding.DecodeString(text)
	if err != nil || len(key) != fileKeyLen {
		return nil, errors.New("invalid key in link")
	}
	return key, nil
}

func chunkNonce(index uint64, final bool) []byte {
	nonce := make([]byte, 12)
	binary.BigEndian.PutUint64(nonce, index)
	if final {
		nonce[11] = 1
	}
	return nonce
}

func fileCipher(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// FileEncryptor encrypts what is written to it. Close must be called to write the final chunk.
type FileEncryptor struct {
	out     io.Writer
	gcm     cipher.AEAD
	pending []byte
	index   uint64
	started bool
}

func NewFileEncryptor(out io.Writer, key []byte) (*FileEncryptor, error) {
	gcm, err := fileCipher(key)
	if err != nil {
		return nil, err
	}
	return &FileEncryptor{out: out, gcm: gcm}, nil
}

func (e *FileEncryptor) Write(data []byte) (int, error) {
	e.pending = append(e.pending, data...)
	// The last chunk is held back until Close, so it can be marked final.
	for len(e.pending) > streamChunk {
		if err := e.emit(e.pending[:streamChunk], false); err != nil {
			return 0, err
		}
		e.pending = e.pending[streamChunk:]
	}
	return len(data), nil
}

func (e *FileEncryptor) Close() error {
	return e.emit(e.pending, true)
}

func (e *FileEncryptor) emit(chunk []byte, final bool) error {
	if !e.started {
		e.started = true
		if _, err := io.WriteString(e.out, streamMagic); err != nil {
			return err
		}
	}
	sealed := e.gcm.Seal(nil, chunkNonce(e.index, final), chunk, nil)
	e.index++
	header := binary.BigEndian.AppendUint32(nil, uint32(len(sealed)))
	if _, err := e.out.Write(header); err != nil {
		return err
	}
	_, err := e.out.Write(sealed)
	return err
}

// FileDecryptor reads the plain text of an encrypted stream.
type FileDecryptor struct {
	in      *bufio.Reader
	gcm     cipher.AEAD
	plain   []byte
	index   uint64
	started bool
	done    bool
}

func NewFileDecryptor(in io.Reader, key []byte) (*FileDecryptor, error) {
	gcm, err := fileCipher(key)
	if err != nil {
		return nil, err
	}
	return &FileDecryptor{in: bufio.NewReader(in), gcm: gcm}, nil
}

func (d *FileDecryptor) Read(buffer []byte) (int, error) {
	for len(d.plain) == 0 {
		if d.done {
			return 0, io.EOF
		}
		if err := d.nextChunk(); err != nil {
			return 0, err
		}
	}
	count := copy(buffer, d.plain)
	d.plain = d.plain[count:]
	return count, nil
}

func (d *FileDecryptor) nextChunk() error {
	if !d.started {
		d.started = true
		magic := make([]byte, len(streamMagic))
		if _, err := io.ReadFull(d.in, magic); err != nil || string(magic) != streamMagic {
			return errCorrupt
		}
	}
	var header [4]byte
	if _, err := io.ReadFull(d.in, header[:]); err != nil {
		return errCorrupt
	}
	length := binary.BigEndian.Uint32(header[:])
	if length < uint32(d.gcm.Overhead()) || length > streamChunk+uint32(d.gcm.Overhead()) {
		return errCorrupt
	}
	sealed := make([]byte, length)
	if _, err := io.ReadFull(d.in, sealed); err != nil {
		return errCorrupt
	}
	_, peekErr := d.in.Peek(1)
	final := peekErr == io.EOF
	plain, err := d.gcm.Open(nil, chunkNonce(d.index, final), sealed, nil)
	if err != nil {
		return errCorrupt
	}
	d.index++
	d.plain = plain
	d.done = final
	return nil
}
