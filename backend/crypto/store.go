package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// AppSecret should be hidden or constructed at runtime in a real app
// We use a fixed key for local obfuscation to allow "Remember Me"
// This is security-by-obscurity against remote attackers, but valid for local convenience
var appSecret = []byte{
    0x4d, 0x6f, 0x63, 0x68, 0x69, 0x42, 0x6f, 0x78, // MochiBox
    0x5f, 0x53, 0x65, 0x63, 0x75, 0x72, 0x65, 0x5f, // _Secure_
    0x4b, 0x65, 0x79, 0x5f, 0x32, 0x30, 0x32, 0x34, // Key_2024
    0x21, 0x40, 0x23, 0x24, 0x25, 0x5e, 0x26, 0x2a, // !@#$%^&*
}

type AuthLock struct {
    Salt         []byte `json:"salt"`
    EncryptedKey []byte `json:"encrypted_key"` // Encrypted Master Password (or derivative)
    Nonce        []byte `json:"nonce"`
}

// SaveAuthLock saves the master password (or key) to a lock file
func SaveAuthLock(data []byte, dir string) error {
    block, err := aes.NewCipher(appSecret)
    if err != nil {
        return err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return err
    }

    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return err
    }

    // Encrypt
    encrypted := gcm.Seal(nil, nonce, data, nil)

    lock := AuthLock{
        EncryptedKey: encrypted,
        Nonce:        nonce,
    }

    bytes, err := json.Marshal(lock)
    if err != nil {
        return err
    }

    path := filepath.Join(dir, "auth.lock")
    return os.WriteFile(path, bytes, 0600)
}

// LoadAuthLock loads and decrypts the master password from the lock file
func LoadAuthLock(dir string) ([]byte, error) {
    path := filepath.Join(dir, "auth.lock")
    bytes, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }

    var lock AuthLock
    if err := json.Unmarshal(bytes, &lock); err != nil {
        return nil, err
    }

    block, err := aes.NewCipher(appSecret)
    if err != nil {
        return nil, err
    }

    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return nil, err
    }

    plaintext, err := gcm.Open(nil, lock.Nonce, lock.EncryptedKey, nil)
    if err != nil {
        return nil, fmt.Errorf("failed to decrypt auth lock")
    }

    return plaintext, nil
}

// ClearAuthLock removes the lock file
func ClearAuthLock(dir string) error {
    path := filepath.Join(dir, "auth.lock")
    return os.Remove(path)
}
