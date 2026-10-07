package files

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"reflect"
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
	count, err := WriteArchive(&sealed, source, key, &packed)
	if err != nil || count != 2 {
		t.Fatalf("pack: %d %v", count, err)
	}

	target := t.TempDir()
	entries, total, err := ExtractArchive(bytes.NewReader(sealed.Bytes()), key, target)
	if err != nil || !reflect.DeepEqual(entries, []string{"photos"}) || total != 200005 {
		t.Fatalf("extract: %v %d %v", entries, total, err)
	}
	if data, _ := os.ReadFile(filepath.Join(target, "photos", "a.txt")); string(data) != "alpha" {
		t.Fatalf("content: %q", data)
	}
	if _, err := os.Lstat(filepath.Join(target, "photos", "link")); err == nil {
		t.Fatal("symlink was shared")
	}

	// Downloading again must not overwrite.
	if _, _, err := ExtractArchive(bytes.NewReader(sealed.Bytes()), key, target); err == nil {
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
		if _, _, err := ExtractArchive(bytes.NewReader(sealed), key, target); err == nil {
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
	if _, _, err := ExtractArchive(bytes.NewReader(sealed[:len(sealed)/2]), key, target); err == nil {
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
