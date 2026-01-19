package core

import (
	"bytes"
	"context"
	"os"
	"testing"
)

func TestAddDirectory(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "TestAddDirectory")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	node, err := NewNode(tempDir, "")
	if err != nil {
		// If node creation fails (e.g. no daemon), we skip
		t.Skipf("Skipping test, failed to create node: %v", err)
	}
	
	// Check if IPFS is responsive
	ctx := context.Background()
	if err := node.Start(ctx); err != nil {
		t.Skipf("Skipping test, IPFS not available: %v", err)
	}

	entries := []FileEntry{
		{Path: "file1.txt", Reader: bytes.NewReader([]byte("content1"))},
		{Path: "sub/file2.txt", Reader: bytes.NewReader([]byte("content2"))},
		{Path: "sub/nested/file3.txt", Reader: bytes.NewReader([]byte("content3"))},
	}

	cidStr, err := node.AddDirectory(ctx, entries)
	if err != nil {
		t.Fatalf("AddDirectory failed: %v", err)
	}

	t.Logf("Added Directory CID: %s", cidStr)

	// Verify sub file
	r, err := node.GetFile(ctx, cidStr+"/sub/file2.txt")
	if err != nil {
		t.Fatalf("Failed to get sub file: %v", err)
	}

	buf := new(bytes.Buffer)
	buf.ReadFrom(r)
	if buf.String() != "content2" {
		t.Fatalf("Content mismatch: got %s, want content2", buf.String())
	}
}
