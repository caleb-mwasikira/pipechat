// Package crypto implements group secrets, invite codes and message sealing.
//
// The "termchat/..." labels below are fixed protocol strings from before the project was renamed.
// They are mixed into every derived key, so changing them would make existing accounts, groups and
// direct chats undecryptable.
//
// A group is defined by a random 32-byte secret that never leaves the
// members' machines. Everything the server needs is derived from it with
// HKDF under distinct labels:
//
//	join token  - proves knowledge of the secret to the server (server stores only its SHA-256)
//	message key - AES-256-GCM key for message bodies
//
// The invite code is base32(version || gid || secret || name), so entering the
// code is all an invitee has to do.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"errors"
	"fmt"
	"golang.org/x/crypto/argon2"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	codeVersion = 1
	gidLen      = 8
	keyLen      = 32
	MaxNameLen  = 24
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

type Secret struct {
	GID  [gidLen]byte
	Key  [keyLen]byte
	Name string
}

func NewSecret(name string) (*Secret, error) {
	if err := validName(name); err != nil {
		return nil, err
	}
	s := &Secret{Name: name}
	if _, err := rand.Read(s.GID[:]); err != nil {
		return nil, err
	}
	if _, err := rand.Read(s.Key[:]); err != nil {
		return nil, err
	}
	return s, nil
}

func validName(n string) error {
	if n == "" || utf8.RuneCountInString(n) > MaxNameLen || !utf8.ValidString(n) {
		return errors.New("group name must be 1-24 characters")
	}
	for _, r := range n {
		if r < 0x20 || r == 0x7f || (r >= 0x80 && r < 0xa0) {
			return errors.New("group name contains control characters")
		}
	}
	return nil
}

func (s *Secret) GIDHex() string { return hex.EncodeToString(s.GID[:]) }

func (s *Secret) derive(label string) []byte {
	k, err := hkdf.Key(sha256.New, s.Key[:], s.GID[:], "termchat/"+label, 32)
	if err != nil {
		panic(err) // only fails for absurd output lengths
	}
	return k
}

// JoinToken is sent to the server to authorise joining. It cannot be used to
// derive the message key.
func (s *Secret) JoinToken() string { return hex.EncodeToString(s.derive("join")) }

// Seal returns nonce || ciphertext, bound to the group id as associated data.
func (s *Secret) Seal(plaintext []byte) ([]byte, error) {
	return seal(s.derive("msg"), s.GID[:], plaintext)
}

func (s *Secret) Open(sealed []byte) ([]byte, error) {
	return open(s.derive("msg"), s.GID[:], sealed)
}

func seal(key []byte, associated []byte, plaintext []byte) ([]byte, error) {
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize(), gcm.NonceSize()+len(plaintext)+gcm.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, associated), nil
}

func open(key []byte, associated []byte, sealed []byte) ([]byte, error) {
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	if len(sealed) < gcm.NonceSize()+gcm.Overhead() {
		return nil, errors.New("ciphertext too short")
	}
	return gcm.Open(nil, sealed[:gcm.NonceSize()], sealed[gcm.NonceSize():], associated)
}

// ValidHandle reports whether handle is 3-20 characters of a-z, 0-9 and _.
func ValidHandle(handle string) bool {
	if len(handle) < 3 || len(handle) > 20 {
		return false
	}
	for _, character := range handle {
		if !(character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '_') {
			return false
		}
	}
	return true
}

// Account holds what a password turns into: the key that protects the vault.
// It never leaves this machine.
type Account struct {
	VaultKey []byte
}

// DeriveAccount stretches the password with Argon2id (salted by the email) into the vault key.
func DeriveAccount(email string, password string) Account {
	salt := sha256.Sum256([]byte("termchat/account/" + strings.ToLower(email)))
	return Account{VaultKey: argon2.IDKey([]byte(password), salt[:], 3, 64*1024, 4, 32)}
}

// SealVault and OpenVault protect the user's list of group codes.
func SealVault(vaultKey []byte, handle string, plaintext []byte) ([]byte, error) {
	return seal(vaultKey, []byte(handle), plaintext)
}

func OpenVault(vaultKey []byte, handle string, sealed []byte) ([]byte, error) {
	return open(vaultKey, []byte(handle), sealed)
}

// Code renders the invite code in dash-separated groups for easy copying.
func (s *Secret) Code() string {
	raw := append([]byte{codeVersion}, s.GID[:]...)
	raw = append(raw, s.Key[:]...)
	raw = append(raw, s.Name...)
	enc := b32.EncodeToString(raw)
	var sb strings.Builder
	for i := 0; i < len(enc); i += 6 {
		if i > 0 {
			sb.WriteByte('-')
		}
		sb.WriteString(enc[i:min(i+6, len(enc))])
	}
	return sb.String()
}

func ParseCode(code string) (*Secret, error) {
	clean := strings.ToUpper(strings.Map(func(r rune) rune {
		if r == '-' || r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, code))
	raw, err := b32.DecodeString(clean)
	if err != nil || len(raw) < 1+gidLen+keyLen+1 || raw[0] != codeVersion {
		return nil, errors.New("invalid invite code")
	}
	s := &Secret{Name: string(raw[1+gidLen+keyLen:])}
	copy(s.GID[:], raw[1:])
	copy(s.Key[:], raw[1+gidLen:])
	if err := validName(s.Name); err != nil {
		return nil, errors.New("invalid invite code")
	}
	return s, nil
}

// NewIdentity makes the key pair that identifies a user. The public half is
// registered with the server; the private half only ever leaves this machine
// inside the sealed vault.
func NewIdentity() (*ecdh.PrivateKey, error) {
	return ecdh.X25519().GenerateKey(rand.Reader)
}

func ParsePrivateKey(encoded string) (*ecdh.PrivateKey, error) {
	raw, err := hex.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	return ecdh.X25519().NewPrivateKey(raw)
}

func ParsePublicKey(encoded string) (*ecdh.PublicKey, error) {
	raw, err := hex.DecodeString(encoded)
	if err != nil {
		return nil, err
	}
	return ecdh.X25519().NewPublicKey(raw)
}

// Fingerprint is a short readable form of a public key, for people to compare.
func Fingerprint(publicKey string) string {
	sum := sha256.Sum256([]byte(publicKey))
	text := hex.EncodeToString(sum[:16])
	var groups []string
	for i := 0; i < len(text); i += 4 {
		groups = append(groups, text[i:i+4])
	}
	return strings.Join(groups, " ")
}

// DirectSecret is the secret of a two-person chat. Both people compute the same
// one from their own private key and the other's public key, so nobody else
// (the server included) can. The group id comes from the secret too, so
// strangers cannot claim it first.
func DirectSecret(own *ecdh.PrivateKey, ownHandle string, theirPublic *ecdh.PublicKey, theirHandle string) (*Secret, error) {
	shared, err := own.ECDH(theirPublic)
	if err != nil {
		return nil, err
	}
	pair := []string{ownHandle, theirHandle}
	sort.Strings(pair)
	info := fmt.Sprintf("termchat/direct/%s/%s", pair[0], pair[1])
	key, err := hkdf.Key(sha256.New, shared, nil, info+"/key", 32)
	if err != nil {
		return nil, err
	}
	id, err := hkdf.Key(sha256.New, shared, nil, info+"/id", gidLen)
	if err != nil {
		return nil, err
	}
	secret := &Secret{Name: "@" + theirHandle}
	copy(secret.Key[:], key)
	copy(secret.GID[:], id)
	return secret, nil
}

// SealFor seals a group's invite code to one member: only they (and the sender) can
// open it, because both ends compute the same shared key from their own private key
// and the other's public key. The server just carries the blob.
func SealFor(own *ecdh.PrivateKey, theirPublic *ecdh.PublicKey, secret *Secret) ([]byte, error) {
	key, err := grantKey(own, theirPublic)
	if err != nil {
		return nil, err
	}
	return seal(key, []byte("termchat/grant"), []byte(secret.Code()))
}

// OpenFrom is the other half of SealFor.
func OpenFrom(own *ecdh.PrivateKey, senderPublic *ecdh.PublicKey, sealed []byte) (*Secret, error) {
	key, err := grantKey(own, senderPublic)
	if err != nil {
		return nil, err
	}
	code, err := open(key, []byte("termchat/grant"), sealed)
	if err != nil {
		return nil, err
	}
	return ParseCode(string(code))
}

func grantKey(own *ecdh.PrivateKey, other *ecdh.PublicKey) ([]byte, error) {
	shared, err := own.ECDH(other)
	if err != nil {
		return nil, err
	}
	return hkdf.Key(sha256.New, shared, nil, "termchat/grant/key", 32)
}

// SealAnon seals a group's invite code to a public key without revealing who sealed it:
// a fresh one-time key pair is made for each call and its public half goes in front of
// the sealed bytes. The server uses it to hold groups' keys for instant invitations.
func SealAnon(recipient *ecdh.PublicKey, secret *Secret) ([]byte, error) {
	temporary, err := NewIdentity()
	if err != nil {
		return nil, err
	}
	sealed, err := SealFor(temporary, recipient, secret)
	if err != nil {
		return nil, err
	}
	return append(temporary.PublicKey().Bytes(), sealed...), nil
}

// OpenAnon opens what SealAnon made.
func OpenAnon(own *ecdh.PrivateKey, blob []byte) (*Secret, error) {
	if len(blob) <= 32 {
		return nil, errors.New("sealed key too short")
	}
	sender, err := ecdh.X25519().NewPublicKey(blob[:32])
	if err != nil {
		return nil, err
	}
	return OpenFrom(own, sender, blob[32:])
}

// EscrowIdentity derives the chat server's own key pair from its secret.
func EscrowIdentity(secret []byte) (*ecdh.PrivateKey, error) {
	raw, err := hkdf.Key(sha256.New, secret, nil, "termchat/escrow/x25519", 32)
	if err != nil {
		return nil, err
	}
	return ecdh.X25519().NewPrivateKey(raw)
}
