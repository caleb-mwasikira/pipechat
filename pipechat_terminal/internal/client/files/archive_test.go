package files

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"pipechat/internal/crypto"
)

func TestSplitArguments(t *testing.T) {
	got := SplitArguments(`"/my dir/a b.txt"  -name   docs`)
	want := []string{"/my dir/a b.txt", "-name", "docs"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q", got)
	}
}

func encrypt(t *testing.T, key []byte, build func(*tar.Writer)) []byte {
	var out bytes.Buffer
	encryptor, _ := crypto.NewFileEncryptor(&out, key)
	compressor := gzip.NewWriter(encryptor)
	archive := tar.NewWriter(compressor)
	build(archive)
	archive.Close()
	compressor.Close()
	encryptor.Close()
	return out.Bytes()
}

func TestFolderRoundTrip(t *testing.T) {
	source := filepath.Join(t.TempDir(), "photos")
	os.MkdirAll(filepath.Join(source, "sub"), 0o755)
	os.WriteFile(filepath.Join(source, "a.txt"), []byte("alpha"), 0o644)
	os.WriteFile(filepath.Join(source, "sub", "b.txt"), bytes.Repeat([]byte("b"), 200000), 0o600)
	os.Symlink("/etc/passwd", filepath.Join(source, "link"))

	key, _, _ := crypto.NewFileKey()
	var sealed bytes.Buffer
	var packed atomic.Int64
	count, packedHash, err := WriteArchive(&sealed, source, key, &packed)
	if err != nil || count != 2 {
		t.Fatalf("pack: %d %v", count, err)
	}

	target := t.TempDir()
	entries, total, hash, err := ExtractArchive(bytes.NewReader(sealed.Bytes()), key, target, "")
	if err != nil || !reflect.DeepEqual(entries, []string{"photos"}) || total != 200005 {
		t.Fatalf("extract: %v %d %v", entries, total, err)
	}
	if data, _ := os.ReadFile(filepath.Join(target, "photos", "a.txt")); string(data) != "alpha" {
		t.Fatalf("content: %q", data)
	}
	if _, err := os.Lstat(filepath.Join(target, "photos", "link")); err == nil {
		t.Fatal("symlink was shared")
	}
	if len(packedHash) != 32 || hash != packedHash {
		t.Fatalf("both ends should agree on the folder's hash: packed %q, unpacked %q", packedHash, hash)
	}

	// With the right hash it passes; with a wrong one nothing is written.
	other := t.TempDir()
	if _, _, _, err := ExtractArchive(bytes.NewReader(sealed.Bytes()), key, other, strings.ToUpper(packedHash)); err != nil {
		t.Fatalf("the right hash was refused: %v", err)
	}
	wrong := t.TempDir()
	if _, _, _, err := ExtractArchive(bytes.NewReader(sealed.Bytes()), key, wrong, strings.Repeat("0", 32)); err == nil || !strings.Contains(err.Error(), "does not match its MD5") {
		t.Fatalf("a wrong hash must be refused: %v", err)
	}
	if left, _ := os.ReadDir(wrong); len(left) != 0 {
		t.Fatalf("a failed check left files behind: %v", left)
	}

	// Downloading again must not overwrite.
	if _, _, _, err := ExtractArchive(bytes.NewReader(sealed.Bytes()), key, target, ""); err == nil {
		t.Fatal("existing folder overwritten")
	}
	left, _ := os.ReadDir(target)
	if len(left) != 1 {
		t.Fatalf("staging directory left behind: %v", left)
	}
}

func TestUnsafeArchivesRejected(t *testing.T) {
	key, _, _ := crypto.NewFileKey()
	for _, name := range []string{"../evil", "/etc/evil", "a/../../evil"} {
		sealed := encrypt(t, key, func(archive *tar.Writer) {
			archive.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: 1, Typeflag: tar.TypeReg})
			archive.Write([]byte("x"))
		})
		parent := t.TempDir()
		target := filepath.Join(parent, "out")
		if _, _, _, err := ExtractArchive(bytes.NewReader(sealed), key, target, ""); err == nil {
			t.Fatalf("%q accepted", name)
		}
		if _, err := os.Stat(filepath.Join(parent, "evil")); err == nil {
			t.Fatalf("%q escaped", name)
		}
	}

	// A truncated download writes nothing.
	sealed := encrypt(t, key, func(archive *tar.Writer) {
		archive.WriteHeader(&tar.Header{Name: "big", Mode: 0o644, Size: 300000, Typeflag: tar.TypeReg})
		archive.Write(bytes.Repeat([]byte("z"), 300000))
	})
	target := t.TempDir()
	if _, _, _, err := ExtractArchive(bytes.NewReader(sealed[:len(sealed)/2]), key, target, ""); err == nil {
		t.Fatal("truncated archive accepted")
	}
	if left, _ := os.ReadDir(target); len(left) != 0 {
		t.Fatalf("partial files left: %v", left)
	}
}

func TestParseLink(t *testing.T) {
	_, keyText := func() ([]byte, string) { k, s, _ := crypto.NewFileKey(); return k, s }()
	good := "http://host:7778/api/files/0123456789abcdef0123456789abcdef#" + keyText
	if address, _, err := ParseLink(good); err != nil || address != "http://host:7778/api/files/0123456789abcdef0123456789abcdef" {
		t.Fatalf("good link: %q %v", address, err)
	}
	for _, bad := range []string{"", "ftp://h/api/files/0123456789abcdef0123456789abcdef#" + keyText, "http://h/other#" + keyText, "http://h/api/files/0123456789abcdef0123456789abcdef", "http://h/api/files/zz#" + keyText} {
		if _, _, err := ParseLink(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}

func TestFileIDIsTheStoredFileBehindALink(t *testing.T) {
	_, keyText := func() ([]byte, string) { k, s, _ := crypto.NewFileKey(); return k, s }()
	if got := FileID("http://host/api/files/0123456789abcdef0123456789abcdef#" + keyText); got != "0123456789abcdef0123456789abcdef" {
		t.Fatalf("id: %q", got)
	}
	if got := FileID("not a link"); got != "" {
		t.Fatalf("a bad link has no id: %q", got)
	}
}

// A single file's hash is the one "md5sum" prints for it, so people can check it themselves.
func TestASingleFileIsHashedByItsOwnBytes(t *testing.T) {
	content := bytes.Repeat([]byte("hello hash\n"), 5000)
	source := filepath.Join(t.TempDir(), "notes.txt")
	os.WriteFile(source, content, 0o644)
	sum := md5.Sum(content)
	want := hex.EncodeToString(sum[:])

	key, _, _ := crypto.NewFileKey()
	var sealed bytes.Buffer
	var packed atomic.Int64
	_, hash, err := WriteArchive(&sealed, source, key, &packed)
	if err != nil || hash != want {
		t.Fatalf("packed hash %q, md5sum says %q (%v)", hash, want, err)
	}
	target := t.TempDir()
	entries, _, got, err := ExtractArchive(bytes.NewReader(sealed.Bytes()), key, target, want)
	if err != nil || got != want || !reflect.DeepEqual(entries, []string{"notes.txt"}) {
		t.Fatalf("unpacked hash %q (%v) %v", got, err, entries)
	}
	// A sender who lies about the hash is caught.
	if _, _, _, err := ExtractArchive(bytes.NewReader(sealed.Bytes()), key, t.TempDir(), strings.Repeat("a", 32)); err == nil {
		t.Fatal("a wrong hash for a file must be refused")
	}
}
