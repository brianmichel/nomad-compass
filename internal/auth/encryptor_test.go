package auth

import (
	"crypto/rand"
	"testing"
)

func TestEncryptorRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand read: %v", err)
	}

	enc, err := NewEncryptor(key)
	if err != nil {
		t.Fatalf("new encryptor: %v", err)
	}

	plaintext := []byte("super secret token")
	cipher, err := enc.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if len(cipher) <= len(plaintext) {
		t.Fatal("ciphertext not larger than plaintext")
	}

	out, err := enc.Decrypt(cipher)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	if string(out) != string(plaintext) {
		t.Fatalf("got %q want %q", out, plaintext)
	}
}
