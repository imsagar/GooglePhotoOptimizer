package crypto_test

import (
	"testing"

	"github.com/user/gpoptimizer/internal/crypto"
)

func TestDeriveKeyDeterministic(t *testing.T) {
	token := []byte("test-token-abc123")
	k1 := crypto.DeriveKey(token)
	k2 := crypto.DeriveKey(token)
	if len(k1) != 32 {
		t.Fatalf("expected 32 bytes, got %d", len(k1))
	}
	for i := range k1 {
		if k1[i] != k2[i] {
			t.Fatal("same token must produce same key")
		}
	}
}

func TestDeriveKeyDifferentTokens(t *testing.T) {
	k1 := crypto.DeriveKey([]byte("token-a"))
	k2 := crypto.DeriveKey([]byte("token-b"))
	same := true
	for i := range k1 {
		if k1[i] != k2[i] {
			same = false
			break
		}
	}
	if same {
		t.Fatal("different tokens must produce different keys")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	key := crypto.DeriveKey([]byte("my-runner-token"))
	plaintext := []byte(`{"type":"progress","job_id":42,"percent":67}`)

	encoded, err := crypto.Encrypt(key, plaintext)
	if err != nil {
		t.Fatal(err)
	}

	decoded, err := crypto.Decrypt(key, encoded)
	if err != nil {
		t.Fatal(err)
	}

	if string(decoded) != string(plaintext) {
		t.Fatalf("got %q, want %q", decoded, plaintext)
	}
}

func TestDecryptWrongKey(t *testing.T) {
	k1 := crypto.DeriveKey([]byte("token-a"))
	k2 := crypto.DeriveKey([]byte("token-b"))

	encoded, _ := crypto.Encrypt(k1, []byte("secret"))
	_, err := crypto.Decrypt(k2, encoded)
	if err == nil {
		t.Fatal("expected decryption failure with wrong key")
	}
}

func TestEncryptProducesUniqueNonces(t *testing.T) {
	key := crypto.DeriveKey([]byte("token"))
	e1, _ := crypto.Encrypt(key, []byte("same"))
	e2, _ := crypto.Encrypt(key, []byte("same"))
	if e1 == e2 {
		t.Fatal("two encryptions of same plaintext must differ (random nonce)")
	}
}
