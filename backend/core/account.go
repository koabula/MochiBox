package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"

	"mochibox-core/crypto"
	"mochibox-core/db"

	"gorm.io/gorm"
)

type AccountManager struct {
	DB      *gorm.DB
	DataDir string

	// In-memory wallet (unlocked)
	Wallet *crypto.Wallet
	Mutex  sync.RWMutex
}

func NewAccountManager(database *gorm.DB, dataDir string) *AccountManager {
	return &AccountManager{
		DB:      database,
		DataDir: dataDir,
	}
}

// IsLocked checks if the wallet is loaded
func (m *AccountManager) IsLocked() bool {
	m.Mutex.RLock()
	defer m.Mutex.RUnlock()
	return m.Wallet == nil
}

// IsConfigured checks if an account exists in DB
func (m *AccountManager) IsConfigured() bool {
	var count int64
	m.DB.Model(&db.Account{}).Count(&count)
	return count > 0
}

// GetProfile returns the public profile
func (m *AccountManager) GetProfile() (*db.Account, error) {
	var acc db.Account
	if err := m.DB.First(&acc).Error; err != nil {
		return nil, err
	}
	acc.IsInitialized = true
	return &acc, nil
}

// InitAccount creates a new account
func (m *AccountManager) InitAccount(mnemonic, password, name string) error {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	// 1. Recover Wallet from Mnemonic
	wallet, err := crypto.RecoverWallet(mnemonic)
	if err != nil {
		return err
	}

	// 2. Encrypt Seed with Password
	// Generate Salt for KDF
	salt, err := crypto.GenerateSalt(16)
	if err != nil {
		return err
	}
	
	// Derive Key
	key := crypto.DeriveKey(password, salt)

	// Encrypt Mnemonic (AES-GCM)
	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)
	nonce := make([]byte, gcm.NonceSize())
	io.ReadFull(rand.Reader, nonce)
	encryptedData := gcm.Seal(nil, nonce, []byte(mnemonic), nil)
	
	// Store nonce + ciphertext
	fullBlob := append(nonce, encryptedData...)

	// 3. Save to DB
	// Clear existing if any (reset)
	m.DB.Exec("DELETE FROM accounts")
	
	acc := db.Account{
		PublicKey:     hex.EncodeToString(wallet.PublicKey),
		Name:          name,
		Avatar:        fmt.Sprintf("https://api.dicebear.com/7.x/identicon/svg?seed=%s", hex.EncodeToString(wallet.PublicKey)),
		EncryptedSeed: base64.StdEncoding.EncodeToString(fullBlob),
		Salt:          hex.EncodeToString(salt),
	}

	if err := m.DB.Create(&acc).Error; err != nil {
		return err
	}

	// Set as unlocked
	m.Wallet = wallet
	return nil
}

// Unlock unlocks the account
func (m *AccountManager) Unlock(password string) error {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()

	var acc db.Account
	if err := m.DB.First(&acc).Error; err != nil {
		return errors.New("no account found")
	}

	// 1. Decode Salt and EncryptedSeed
	salt, _ := hex.DecodeString(acc.Salt)
	blob, _ := base64.StdEncoding.DecodeString(acc.EncryptedSeed)

	// 2. Derive Key
	key := crypto.DeriveKey(password, salt)

	// 3. Decrypt
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	nonceSize := gcm.NonceSize()
	if len(blob) < nonceSize {
		return errors.New("invalid encrypted data")
	}

	nonce, ciphertext := blob[:nonceSize], blob[nonceSize:]
	mnemonicBytes, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return errors.New("invalid password")
	}

	// 4. Reconstruct Wallet
	wallet, err := crypto.RecoverWallet(string(mnemonicBytes))
	if err != nil {
		return err
	}

	m.Wallet = wallet
	
	return nil
}

// Lock clears memory
func (m *AccountManager) Lock() {
	m.Mutex.Lock()
	defer m.Mutex.Unlock()
	m.Wallet = nil
}

// Reset deletes the account and lock file
func (m *AccountManager) Reset() error {
    m.Mutex.Lock()
    defer m.Mutex.Unlock()
    
    // Clear Wallet
    m.Wallet = nil
    
    // Delete from DB
    if err := m.DB.Exec("DELETE FROM accounts").Error; err != nil {
        return err
    }
    
    // Clear Lock File
    crypto.ClearAuthLock(m.DataDir)
    
    return nil
}

// Sign signs data using the wallet's private key
func (m *AccountManager) Sign(data []byte) ([]byte, error) {
	m.Mutex.RLock()
	defer m.Mutex.RUnlock()

	if m.Wallet == nil {
		return nil, errors.New("wallet locked")
	}

	return m.Wallet.Sign(data), nil
}

// Verify verifies a signature
func (m *AccountManager) Verify(data, signature, publicKey []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(publicKey, data, signature)
}

// DecryptBox decrypts a sealed box using the account's private key
func (m *AccountManager) DecryptBox(encrypted []byte) ([]byte, error) {
	m.Mutex.RLock()
	defer m.Mutex.RUnlock()

	if m.Wallet == nil {
		return nil, errors.New("wallet locked")
	}

	// 1. Convert Ed25519 Private Key -> X25519 Private Key
	privKey, err := crypto.Ed25519PrivateKeyToCurve25519(m.Wallet.PrivateKey)
	if err != nil {
		return nil, err
	}
	
	// 2. Open Box
	// golang.org/x/crypto/nacl/box.OpenAnonymous expects:
	// out, box, publicKey, privateKey
	// SealAnonymous prepends the ephemeral public key to the ciphertext.
	// OpenAnonymous handles reading that.
	
	// We need to pass the receiver's public key (our public key) as the 3rd arg?
	// box.OpenAnonymous(out, box, publicKey, privateKey)
	// publicKey here is the Sender's public key. But for Anonymous seal, the sender is ephemeral.
	// Actually, OpenAnonymous takes: (out, box, publicKey, privateKey)
	// "The publicKey argument is the public key of the recipient." -> Wait, doc says:
	// "OpenAnonymous opens a box sealed by SealAnonymous. It returns the plaintext... publicKey is the public key of the recipient..."
	
	// Let's check `box` package docs or usage.
	// SealAnonymous(out, message, recipient, rand) -> returns ephemeralPub + encryptedMsg
	// OpenAnonymous(out, box, recipientPub, recipientPriv) -> expects box to contain ephemeralPub
	
	// So we need our Curve25519 Public Key too.
	pubKey, err := crypto.Ed25519PublicKeyToCurve25519(m.Wallet.PublicKey)
	if err != nil {
		return nil, err
	}
	
	var pubKeyArr [32]byte
	var privKeyArr [32]byte
	copy(pubKeyArr[:], pubKey)
	copy(privKeyArr[:], privKey)
	
	return crypto.DecryptBoxAnonymous(encrypted, &pubKeyArr, &privKeyArr)
}

// ExportMnemonic validates password and returns mnemonic
func (m *AccountManager) ExportMnemonic(password string) (string, error) {
    m.Mutex.RLock()
    defer m.Mutex.RUnlock()
    
    // We reuse the Unlock logic but return the mnemonic instead of setting m.Wallet
    // Or if wallet is already unlocked, we could just return m.Wallet.Mnemonic?
    // No, for security, always require password re-entry to view sensitive data.
    
    var acc db.Account
    if err := m.DB.First(&acc).Error; err != nil {
        return "", errors.New("no account found")
    }

    salt, _ := hex.DecodeString(acc.Salt)
    blob, _ := base64.StdEncoding.DecodeString(acc.EncryptedSeed)

    key := crypto.DeriveKey(password, salt)

    block, err := aes.NewCipher(key)
    if err != nil {
        return "", err
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }

    nonceSize := gcm.NonceSize()
    if len(blob) < nonceSize {
        return "", errors.New("invalid encrypted data")
    }

    nonce, ciphertext := blob[:nonceSize], blob[nonceSize:]
    mnemonicBytes, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", errors.New("invalid password")
    }
    
    return string(mnemonicBytes), nil
}

// ChangePassword re-encrypts the mnemonic with a new password
func (m *AccountManager) ChangePassword(oldPassword, newPassword string) error {
    m.Mutex.Lock()
    defer m.Mutex.Unlock()
    
    // 1. Verify Old Password & Get Mnemonic
    var acc db.Account
    if err := m.DB.First(&acc).Error; err != nil {
        return errors.New("no account found")
    }

    salt, _ := hex.DecodeString(acc.Salt)
    blob, _ := base64.StdEncoding.DecodeString(acc.EncryptedSeed)

    key := crypto.DeriveKey(oldPassword, salt)

    block, err := aes.NewCipher(key)
    if err != nil {
        return err
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return err
    }

    nonceSize := gcm.NonceSize()
    if len(blob) < nonceSize {
        return errors.New("invalid encrypted data")
    }

    nonce, ciphertext := blob[:nonceSize], blob[nonceSize:]
    mnemonicBytes, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return errors.New("invalid old password")
    }
    
    mnemonic := string(mnemonicBytes)
    
    // 2. Encrypt with New Password
    newSalt, err := crypto.GenerateSalt(16)
    if err != nil {
        return err
    }
    
    newKey := crypto.DeriveKey(newPassword, newSalt)
    
    block, _ = aes.NewCipher(newKey)
    gcm, _ = cipher.NewGCM(block)
    newNonce := make([]byte, gcm.NonceSize())
    io.ReadFull(rand.Reader, newNonce)
    encryptedData := gcm.Seal(nil, newNonce, []byte(mnemonic), nil)
    
    fullBlob := append(newNonce, encryptedData...)
    
    // 3. Update DB
    acc.EncryptedSeed = base64.StdEncoding.EncodeToString(fullBlob)
    acc.Salt = hex.EncodeToString(newSalt)
    
    if err := m.DB.Save(&acc).Error; err != nil {
        return err
    }
    
    // 4. Update Lock File if Remember Me was active? 
    // We should probably clear it to force re-login or update it.
    // Safer to clear it.
    crypto.ClearAuthLock(m.DataDir)
    
    return nil
}
