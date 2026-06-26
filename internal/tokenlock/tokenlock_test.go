package tokenlock_test

import (
	"testing"

	"github.com/yashikota/enbu/internal/tokenlock"
)

func TestRoundTrip(t *testing.T) {
	token := "gho_test_token_1234567890abcdef"
	plaintext := []byte("AGE-SECRET-KEY-1QFGQJ...")

	encrypted, err := tokenlock.Encrypt(plaintext, token)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	decrypted, err := tokenlock.Decrypt(encrypted, token)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Fatalf("got %q, want %q", decrypted, plaintext)
	}
}

func TestWrongToken(t *testing.T) {
	token := "gho_correct_token"
	wrongToken := "gho_wrong_token"
	plaintext := []byte("secret data")

	encrypted, err := tokenlock.Encrypt(plaintext, token)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	_, err = tokenlock.Decrypt(encrypted, wrongToken)
	if err == nil {
		t.Fatal("expected error decrypting with wrong token")
	}
}

func TestEmptyPlaintext(t *testing.T) {
	token := "gho_test_token"
	plaintext := []byte("")

	encrypted, err := tokenlock.Encrypt(plaintext, token)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	decrypted, err := tokenlock.Decrypt(encrypted, token)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if string(decrypted) != "" {
		t.Fatalf("got %q, want empty", decrypted)
	}
}

func TestCiphertextTooShort(t *testing.T) {
	_, err := tokenlock.Decrypt([]byte("short"), "token")
	if err == nil {
		t.Fatal("expected error for short ciphertext")
	}
}

func TestLargePlaintext(t *testing.T) {
	token := "gho_test_token"
	plaintext := make([]byte, 10000)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	encrypted, err := tokenlock.Encrypt(plaintext, token)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	decrypted, err := tokenlock.Decrypt(encrypted, token)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Fatal("large plaintext round-trip failed")
	}
}
