package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
    "fmt"
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

// SeekableAESCTRDecrypter supports random access decryption
type SeekableAESCTRDecrypter struct {
	src    io.ReadSeeker
	block  cipher.Block
	iv     []byte // The original IV (from file header)
	offset int64  // Current logical offset (decrypted stream position)
	stream cipher.Stream
}

func NewSeekableAESCTRDecrypter(src io.ReadSeeker, key []byte) (*SeekableAESCTRDecrypter, error) {
	// Read IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(src, iv); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Initial stream at offset 0
	stream := cipher.NewCTR(block, iv)

	return &SeekableAESCTRDecrypter{
		src:    src,
		block:  block,
		iv:     iv,
		offset: 0,
		stream: stream,
	}, nil
}

func (r *SeekableAESCTRDecrypter) Read(p []byte) (n int, err error) {
	n, err = r.src.Read(p)
	if n > 0 {
		r.stream.XORKeyStream(p[:n], p[:n])
		r.offset += int64(n)
	}
	return n, err
}

func (r *SeekableAESCTRDecrypter) Seek(offset int64, whence int) (int64, error) {
	var abs int64
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = r.offset + offset
	case io.SeekEnd:
		// We don't know the size unless we seek to end of src
		// But src includes IV.
		// So we seek src to end
		size, err := r.src.Seek(0, io.SeekEnd)
		if err != nil {
			return 0, err
		}
		// Logical size = Physical size - 16
		if size < int64(aes.BlockSize) {
			return 0, fmt.Errorf("file too small")
		}
		logicalSize := size - int64(aes.BlockSize)
		abs = logicalSize + offset
	default:
		return 0, fmt.Errorf("invalid whence")
	}

	if abs < 0 {
		return 0, fmt.Errorf("negative position")
	}

	// 1. Seek underlying source
	// Physical offset = 16 (IV) + abs
	physOffset := int64(aes.BlockSize) + abs
	if _, err := r.src.Seek(physOffset, io.SeekStart); err != nil {
		return 0, err
	}

	// 2. Reset Cipher Stream
	// Calculate block index
	blockIndex := abs / int64(aes.BlockSize)
	byteOffset := abs % int64(aes.BlockSize)

	// Calculate new IV = original IV + blockIndex
	newIV := make([]byte, aes.BlockSize)
	copy(newIV, r.iv)
	addCounter(newIV, uint64(blockIndex))

	r.stream = cipher.NewCTR(r.block, newIV)

	// 3. Fast-forward stream to byte alignment
	if byteOffset > 0 {
		dummy := make([]byte, byteOffset)
		r.stream.XORKeyStream(dummy, dummy)
	}

	r.offset = abs
	return abs, nil
}

// addCounter adds val to the 128-bit big-endian integer counter
func addCounter(counter []byte, val uint64) {
	carry := val
	for i := len(counter) - 1; i >= 0; i-- {
		sum := uint64(counter[i]) + (carry & 0xFF)
		counter[i] = byte(sum)
		carry = (carry >> 8) + (sum >> 8)
		if carry == 0 {
			break
		}
	}
}
