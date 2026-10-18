package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"golang.org/x/crypto/argon2"
	"strings"
)

func Random(n int) (string, error) {
	b := make([]byte, n)
	if _, e := rand.Read(b); e != nil {
		return "", e
	}
	return hex.EncodeToString(b), nil
}
func Argon2id(password string, salt []byte) string {
	return hex.EncodeToString(argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32))
}
func VerifyArgon2id(password, stored string) bool {
	parts := strings.Split(stored, "$")
	if len(parts) != 6 {
		return false
	}
	var salt []byte
	salt, _ = hex.DecodeString(parts[4])
	got := Argon2id(password, salt)
	return strings.EqualFold(got, parts[5])
}
func HashAdmin(password string) (string, error) {
	salt := make([]byte, 16)
	if _, e := rand.Read(salt); e != nil {
		return "", e
	}
	return fmt.Sprintf("$argon2id$v=19$m=65536,t=1,p=4$%s$%s", hex.EncodeToString(salt), Argon2id(password, salt)), nil
}
func SHA256Hex(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
