package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"io"
	"testing"

	"mochibox-core/core"
	"mochibox-core/crypto"
	"mochibox-core/db"
)

func TestGetDecryptedStream_Symmetric_OwnerUsesLocalAuth(t *testing.T) {
	tmp := t.TempDir()

	node, err := core.NewNode(tmp)
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	defer node.Stop()

	database, err := db.InitDB("file:memdb_owner?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}

	plain := []byte("secret content for mochibox")
	salt := bytes.Repeat([]byte{0x02}, 16)
	key := crypto.DeriveKey("pw", salt)

	encReader, err := crypto.EncryptedReader(key, bytes.NewReader(plain))
	if err != nil {
		t.Fatalf("EncryptedReader: %v", err)
	}

	cid, err := node.AddFile(context.Background(), encReader)
	if err != nil {
		t.Fatalf("AddFile: %v", err)
	}

	rec := db.File{
		CID:            cid,
		Name:           "a.txt",
		Size:           int64(len(plain)),
		MimeType:       "text/plain",
		EncryptionType: db.EncryptionSymmetric,
		EncryptedKey:   base64.StdEncoding.EncodeToString(salt),
		LocalAuth:      "pw",
	}
	if err := database.Create(&rec).Error; err != nil {
		t.Fatalf("Create file rec: %v", err)
	}

	s := &Server{Node: node, DB: database}
	r, _, err := s.GetDecryptedStream(context.Background(), cid, DecryptOptions{})
	if err != nil {
		t.Fatalf("GetDecryptedStream: %v", err)
	}

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("decrypt mismatch: got %q want %q", string(got), string(plain))
	}
}

func TestGetDecryptedStream_Symmetric_SharedUsesProvidedPasswordAndSalt(t *testing.T) {
	tmp := t.TempDir()

	node, err := core.NewNode(tmp)
	if err != nil {
		t.Fatalf("NewNode: %v", err)
	}
	defer node.Stop()

	database, err := db.InitDB("file:memdb_shared?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("InitDB: %v", err)
	}

	plain := []byte("shared secret")
	salt := bytes.Repeat([]byte{0x03}, 16)
	key := crypto.DeriveKey("pw", salt)

	encReader, err := crypto.EncryptedReader(key, bytes.NewReader(plain))
	if err != nil {
		t.Fatalf("EncryptedReader: %v", err)
	}

	cid, err := node.AddFile(context.Background(), encReader)
	if err != nil {
		t.Fatalf("AddFile: %v", err)
	}

	s := &Server{Node: node, DB: database}
	r, _, err := s.GetDecryptedStream(context.Background(), cid, DecryptOptions{
		Password:     "pw",
		EncryptedKey: base64.StdEncoding.EncodeToString(salt),
	})
	if err != nil {
		t.Fatalf("GetDecryptedStream(shared): %v", err)
	}

	got, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if !bytes.Equal(got, plain) {
		t.Fatalf("decrypt mismatch: got %q want %q", string(got), string(plain))
	}
}
