package core

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"os"
	"testing"
)

func TestAddGetFile_LargePlain(t *testing.T) {
    // Setup Node
	tempDir, err := os.MkdirTemp("", "TestAddGetFile_LargePlain")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	node, err := NewNode(tempDir)
	if err != nil {
		t.Fatal(err)
	}
	defer node.Stop()

	// 1MB file
	content := make([]byte, 1024*1024) 
	rand.Read(content)

	ctx := context.Background()
	cidStr, err := node.AddFile(ctx, bytes.NewReader(content))
	if err != nil {
		t.Fatalf("AddFile failed: %v", err)
	}
    
    // Check if it is ProtoNode
    // We can't easily check type here without exposing internal, but logs will show.
    
	// Get
	reader, err := node.GetFile(ctx, cidStr)
	if err != nil {
		t.Fatalf("GetFile failed: %v", err)
	}

	readBack, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll failed: %v", err)
	}

	if !bytes.Equal(content, readBack) {
		t.Fatal("Content mismatch")
	}
}
