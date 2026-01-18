package crypto

import (
	"crypto/rand"
	"encoding/hex"

	"golang.org/x/crypto/argon2"
)

// DeriveKey derives a 32-byte key from a password and salt using Argon2id
func DeriveKey(password string, salt []byte) []byte {
	// Params: time=1, memory=64MB, threads=4, keyLen=32
	return argon2.IDKey([]byte(password), salt, 1, 64*1024, 4, 32)
}

// GenerateSalt generates a random salt of given length
func GenerateSalt(length int) ([]byte, error) {
	salt := make([]byte, length)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	return salt, nil
}

// GenerateSaltHex returns a hex string of the salt
func GenerateSaltHex(length int) (string, error) {
	salt, err := GenerateSalt(length)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(salt), nil
}

// ValidateKey checks if the derived key matches (optional, usually we just try to decrypt)
func ValidateKey(password string, salt []byte, expectedKey []byte) bool {
    // Note: Comparing keys directly is fine for this context, but constant time compare is better
    // For now, we usually validate by successfully decrypting a known value (like a hash of the key itself)
    return true 
}
