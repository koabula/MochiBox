package core

import (
	"context"
	"fmt"
	"io"
	"log"

	"mochibox-core/crypto"
)

type EncryptedDownloader struct {
	parallelDownloader *ParallelDownloader
}

func NewEncryptedDownloader(pd *ParallelDownloader) *EncryptedDownloader {
	return &EncryptedDownloader{
		parallelDownloader: pd,
	}
}

// DownloadAndDecrypt downloads an encrypted file and decrypts it with the provided key
// The key should already be derived (from password+salt or decrypted session key)
// File format: [16B IV][AES-CTR encrypted data]
func (ed *EncryptedDownloader) DownloadAndDecrypt(ctx context.Context, cid string, key []byte, dst io.Writer, progressCallback func(downloaded int64)) error {
	if ed.parallelDownloader == nil {
		return fmt.Errorf("parallel downloader not initialized")
	}

	// Create a pipe for streaming decryption
	pr, pw := io.Pipe()

	// Channel to coordinate download completion
	downloadDone := make(chan error, 1)

	// Start download to pipe writer
	go func() {
		defer pw.Close()
		err := ed.parallelDownloader.DownloadFile(ctx, cid, pw, progressCallback, nil)
		if err != nil {
			pw.CloseWithError(err)
			downloadDone <- err
		} else {
			downloadDone <- nil
		}
	}()

	// Stream decrypt from pipe reader to destination
	// File format is [16B IV][encrypted data], matching NewAESCTRDecrypter expectations
	decryptReader, err := crypto.NewAESCTRDecrypter(pr, key)
	if err != nil {
		return fmt.Errorf("failed to create decrypt stream: %w", err)
	}

	// Copy decrypted data to destination
	_, copyErr := io.Copy(dst, decryptReader)

	// Wait for download to complete
	downloadErr := <-downloadDone

	// Return first error encountered
	if downloadErr != nil {
		return fmt.Errorf("download failed: %w", downloadErr)
	}
	if copyErr != nil {
		return fmt.Errorf("decryption failed: %w", copyErr)
	}

	log.Printf("Successfully downloaded and decrypted file %s", cid)
	return nil
}
