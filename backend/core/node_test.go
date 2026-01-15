package core

import (
	"bytes"
	"io"
	"testing"

	"mochibox-core/crypto"
)

func TestAddGetFile_Plain(t *testing.T) {
	n, err := NewNode(t.TempDir())
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	defer n.Stop()

	plain := []byte("hello mochibox")
	c, err := n.AddFile(t.Context(), bytes.NewReader(plain))
	if err != nil {
		t.Fatalf("AddFile: %v", err)
	}

	r, err := n.GetFile(t.Context(), c)
	if err != nil {
		t.Fatalf("GetFile: %v", err)
	}

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("plain mismatch: got %q want %q", string(got), string(plain))
	}
}

func TestAddGetFile_SymmetricEncryptedStream(t *testing.T) {
	n, err := NewNode(t.TempDir())
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	defer n.Stop()

	plain := bytes.Repeat([]byte("a"), 1024)
	salt := bytes.Repeat([]byte{0x01}, 16)
	key := crypto.DeriveKey("pw", salt)

	encReader, err := crypto.EncryptedReader(key, bytes.NewReader(plain))
	if err != nil {
		t.Fatalf("EncryptedReader: %v", err)
	}

	c, err := n.AddFile(t.Context(), encReader)
	if err != nil {
		t.Fatalf("AddFile(enc): %v", err)
	}

	encStream, err := n.GetFile(t.Context(), c)
	if err != nil {
		t.Fatalf("GetFile(enc): %v", err)
	}

	encBytes, err := io.ReadAll(encStream)
	if err != nil {
		t.Fatalf("ReadAll(enc): %v", err)
	}

	decReader, err := crypto.DecryptedReader(key, bytes.NewReader(encBytes))
	if err != nil {
		t.Fatalf("DecryptedReader: %v", err)
	}

	got, err := io.ReadAll(decReader)
	if err != nil {
		t.Fatalf("ReadAll(dec): %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("decrypt mismatch: got %d bytes want %d bytes", len(got), len(plain))
	}
}
