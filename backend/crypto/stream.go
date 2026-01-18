package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
)

// NewAESCTRReader returns a reader that encrypts the source stream using AES-CTR.
// It prepends the IV (16 bytes) to the output.
func NewAESCTRReader(src io.Reader, key []byte) (io.Reader, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Generate IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	stream := cipher.NewCTR(block, iv)
	
	return &cipherStreamReader{
		src:    src,
		stream: stream,
		header: iv, // Send IV first
	}, nil
}

// NewAESCTRDecrypter returns a reader that decrypts the source stream.
// It assumes the first 16 bytes are the IV.
func NewAESCTRDecrypter(src io.Reader, key []byte) (io.Reader, error) {
    // Read IV
    iv := make([]byte, aes.BlockSize)
    if _, err := io.ReadFull(src, iv); err != nil {
        return nil, err
    }
    
    block, err := aes.NewCipher(key)
    if err != nil {
        return nil, err
    }
    
    stream := cipher.NewCTR(block, iv)
    
    return &cipherStreamReader{
        src:    src,
        stream: stream,
        header: nil,
    }, nil
}


type cipherStreamReader struct {
	src    io.Reader
	stream cipher.Stream
	header []byte // Pending header bytes to read
    headerOffset int
}

func (r *cipherStreamReader) Read(p []byte) (n int, err error) {
    // 1. Read Header first if any
    if len(r.header) > 0 {
        n = copy(p, r.header[r.headerOffset:])
        r.headerOffset += n
        if r.headerOffset >= len(r.header) {
            r.header = nil // Done with header
        }
        if n == len(p) {
            return n, nil
        }
        // Continue reading body
        p = p[n:]
    }
    
    // 2. Read Body
    m, readErr := r.src.Read(p)
    if m > 0 {
        r.stream.XORKeyStream(p[:m], p[:m])
    }
    return n + m, readErr
}
