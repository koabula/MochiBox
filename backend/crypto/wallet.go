package crypto

import (
	"crypto/ed25519"
	"fmt"

	"github.com/tyler-smith/go-bip39"
)

type Wallet struct {
	Mnemonic string
	Seed     []byte
	// In memory keys
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

// NewWallet creates a new random wallet
func NewWallet() (*Wallet, error) {
	entropy, err := bip39.NewEntropy(256) // 24 words for high security
	if err != nil {
		return nil, err
	}
	mnemonic, _ := bip39.NewMnemonic(entropy)

	return RecoverWallet(mnemonic)
}

// RecoverWallet recovers a wallet from mnemonic
func RecoverWallet(mnemonic string) (*Wallet, error) {
	if !bip39.IsMnemonicValid(mnemonic) {
		return nil, fmt.Errorf("invalid mnemonic")
	}
	seed := bip39.NewSeed(mnemonic, "") // No passphrase for BIP39 seed

	// Derive Ed25519 Key
	// BIP-39 seed is 64 bytes. Ed25519 NewKeyFromSeed requires 32 bytes.
	// We use the first 32 bytes of the seed.
	if len(seed) < 32 {
		return nil, fmt.Errorf("seed too short")
	}

	privKey := ed25519.NewKeyFromSeed(seed[:32])
	pubKey := privKey.Public().(ed25519.PublicKey)

	return &Wallet{
		Mnemonic:   mnemonic,
		Seed:       seed,
		PrivateKey: privKey,
		PublicKey:  pubKey,
	}, nil
}

// Sign signs the message with the private key
func (w *Wallet) Sign(message []byte) []byte {
	return ed25519.Sign(w.PrivateKey, message)
}

// DecryptSessionKey decrypts an encrypted session key using the wallet's private key
// The encrypted data is expected to be sealed with box.SealAnonymous using our public key
func (w *Wallet) DecryptSessionKey(encrypted []byte) ([]byte, error) {
	// Convert Ed25519 keys to Curve25519 for box decryption
	curvePriv, err := Ed25519PrivateKeyToCurve25519(w.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to convert private key: %w", err)
	}
	curvePub, err := Ed25519PublicKeyToCurve25519(w.PublicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to convert public key: %w", err)
	}

	var pubKey [32]byte
	var privKey [32]byte
	copy(pubKey[:], curvePub)
	copy(privKey[:], curvePriv)

	return DecryptBoxAnonymous(encrypted, &pubKey, &privKey)
}
