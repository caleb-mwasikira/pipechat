package crypto

import (
	"bytes"
	"encoding/hex"
	"io"
	"testing"
)

func TestCodeRoundTripAndSeal(t *testing.T) {
	a, err := NewSecret("team ✓")
	if err != nil {
		t.Fatal(err)
	}
	b, err := ParseCode(" " + a.Code() + "\n")
	if err != nil {
		t.Fatal(err)
	}
	if *a != *b {
		t.Fatal("secret mismatch after code round trip")
	}
	ct, _ := a.Seal([]byte("hello"))
	pt, err := b.Open(ct)
	if err != nil || string(pt) != "hello" {
		t.Fatalf("open: %q %v", pt, err)
	}
	ct[len(ct)-1] ^= 1
	if _, err := b.Open(ct); err == nil {
		t.Fatal("tampered ciphertext accepted")
	}
	other, _ := NewSecret("x")
	ct2, _ := a.Seal([]byte("hi"))
	if _, err := other.Open(ct2); err == nil {
		t.Fatal("wrong key accepted")
	}
	if a.JoinToken() == "" || a.JoinToken() == b.GIDHex() {
		t.Fatal("bad join token")
	}
}

func TestBadCodes(t *testing.T) {
	for _, c := range []string{"", "abc", "AAAA-BBBB", "!!!!"} {
		if _, err := ParseCode(c); err == nil {
			t.Errorf("accepted %q", c)
		}
	}
}

func TestDirectSecretMatches(t *testing.T) {
	alice, _ := NewIdentity()
	bob, _ := NewIdentity()
	mallory, _ := NewIdentity()

	forAlice, err := DirectSecret(alice, "alice", bob.PublicKey(), "bob")
	if err != nil {
		t.Fatal(err)
	}
	forBob, _ := DirectSecret(bob, "bob", alice.PublicKey(), "alice")
	if forAlice.Key != forBob.Key || forAlice.GID != forBob.GID {
		t.Fatal("both sides must derive the same secret")
	}
	forMallory, _ := DirectSecret(mallory, "mallory", alice.PublicKey(), "alice")
	if forMallory.Key == forAlice.Key || forMallory.GID == forAlice.GID {
		t.Fatal("a third party derived the same secret")
	}
	restored, err := ParsePrivateKey(hex.EncodeToString(alice.Bytes()))
	if err != nil || !restored.Equal(alice) {
		t.Fatal("private key did not survive hex round trip")
	}
}

func TestFileStream(t *testing.T) {
	key, _, _ := NewFileKey()
	for _, size := range []int{0, 1, streamChunk - 1, streamChunk, streamChunk + 1, 3*streamChunk + 17} {
		data := make([]byte, size)
		for i := range data {
			data[i] = byte(i * 7)
		}
		var sealed bytes.Buffer
		encryptor, _ := NewFileEncryptor(&sealed, key)
		encryptor.Write(data)
		if err := encryptor.Close(); err != nil {
			t.Fatal(err)
		}
		decryptor, _ := NewFileDecryptor(bytes.NewReader(sealed.Bytes()), key)
		got, err := io.ReadAll(decryptor)
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("size %d: round trip failed: %v", size, err)
		}

		if size > streamChunk {
			// Dropping the last chunk must be detected, not look like a shorter file.
			cut := sealed.Len() - (size%streamChunk + 16 + 4)
			if cut > len(streamMagic) {
				short, _ := NewFileDecryptor(bytes.NewReader(sealed.Bytes()[:cut]), key)
				if _, err := io.ReadAll(short); err == nil {
					t.Fatalf("size %d: truncation not detected", size)
				}
			}
		}
		tampered := bytes.Clone(sealed.Bytes())
		tampered[len(tampered)-1] ^= 1
		bad, _ := NewFileDecryptor(bytes.NewReader(tampered), key)
		if _, err := io.ReadAll(bad); err == nil {
			t.Fatalf("size %d: tampering not detected", size)
		}
		otherKey, _, _ := NewFileKey()
		wrong, _ := NewFileDecryptor(bytes.NewReader(sealed.Bytes()), otherKey)
		if _, err := io.ReadAll(wrong); err == nil {
			t.Fatalf("size %d: wrong key accepted", size)
		}
	}
}

func TestSealForOpensOnlyForTheRecipient(t *testing.T) {
	alice, _ := NewIdentity()
	bob, _ := NewIdentity()
	eve, _ := NewIdentity()
	secret, _ := NewSecret("crew")

	sealed, err := SealFor(alice, bob.PublicKey(), secret)
	if err != nil {
		t.Fatal(err)
	}
	opened, err := OpenFrom(bob, alice.PublicKey(), sealed)
	if err != nil || opened.Code() != secret.Code() {
		t.Fatalf("recipient could not open it: %v", err)
	}
	if _, err := OpenFrom(eve, alice.PublicKey(), sealed); err == nil {
		t.Fatal("a third party opened it")
	}
}
