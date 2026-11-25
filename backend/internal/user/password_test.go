package user

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestHashPasswordFormat(t *testing.T) {
	const password = "senhaSegura1"
	hash, err := hashPassword(password)
	if err != nil {
		t.Fatalf("hashPassword: %v", err)
	}
	const prefix = "$argon2id$v=19$m=65536,t=3,p=4$"
	if !strings.HasPrefix(hash, prefix) {
		t.Fatalf("hash %q does not start with %q", hash, prefix)
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		t.Fatalf("hash has %d $-separated parts, want 6", len(parts))
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil || len(salt) != 16 {
		t.Errorf("salt: len=%d err=%v, want 16 bytes of raw std base64", len(salt), err)
	}
	key, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(key) != 32 {
		t.Errorf("key: len=%d err=%v, want 32 bytes of raw std base64", len(key), err)
	}
	if strings.Contains(hash, password) {
		t.Error("hash contains the plain-text password")
	}
}

func TestHashPasswordUsesRandomSalt(t *testing.T) {
	a, err := hashPassword("senhaSegura1")
	if err != nil {
		t.Fatal(err)
	}
	b, err := hashPassword("senhaSegura1")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Error("two hashes of the same password are equal; the salt is not random")
	}
}
