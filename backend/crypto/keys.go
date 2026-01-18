package crypto

import (
	"crypto/ed25519"
	"crypto/sha512"
	"crypto/rand"
	"fmt"

	"github.com/jorrizza/ed2curve25519"
	"golang.org/x/crypto/nacl/box"
)

// Ed25519PrivateKeyToCurve25519 converts an Ed25519 private key to a Curve25519 private key
func Ed25519PrivateKeyToCurve25519(edPriv ed25519.PrivateKey) ([]byte, error) {
	if len(edPriv) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key size")
	}

	// 1. Hash the seed (first 32 bytes)
	h := sha512.Sum512(edPriv[:32])

	// 2. Clamp
	s := h[:32]
	s[0] &= 248
	s[31] &= 127
	s[31] |= 64

	// This is the X25519 private key (scalar)
	return s, nil
}

// Ed25519PublicKeyToCurve25519 converts an Ed25519 public key to a Curve25519 public key
func Ed25519PublicKeyToCurve25519(edPub ed25519.PublicKey) ([]byte, error) {
	if len(edPub) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid public key size")
	}

	curvePub := ed2curve25519.Ed25519PublicKeyToCurve25519(edPub)
	return curvePub, nil
}

// EncryptSessionKey encrypts the session key using the receiver's Curve25519 public key
func EncryptSessionKey(curvePub []byte, sessionKey []byte) ([]byte, error) {
	if len(curvePub) != 32 {
		return nil, fmt.Errorf("invalid public key length")
	}
	var pubKey [32]byte
	copy(pubKey[:], curvePub)

	return box.SealAnonymous(nil, sessionKey, &pubKey, rand.Reader)
}

// DecryptBoxAnonymous decrypts an anonymously sealed box
func DecryptBoxAnonymous(encrypted []byte, pubKey *[32]byte, privKey *[32]byte) ([]byte, error) {
    out, ok := box.OpenAnonymous(nil, encrypted, pubKey, privKey)
    if !ok {
        return nil, fmt.Errorf("decryption failed")
    }
    return out, nil
}
