package crypto

import (
	"crypto/sha256"
	"io"

	"golang.org/x/crypto/hkdf"
)

const salt = "gpoptimizer-ws-v1"

// DeriveKey derives a 32-byte AES-256 key from a runner token via HKDF-SHA256.
func DeriveKey(token []byte) []byte {
	r := hkdf.New(sha256.New, token, []byte(salt), nil)
	key := make([]byte, 32)
	if _, err := io.ReadFull(r, key); err != nil {
		panic("hkdf: " + err.Error())
	}
	return key
}
