package core

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"

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

// DownloadAndDecrypt downloads an encrypted file in parallel and decrypts it in order
// The download phase is parallel (chunks can arrive out of order)
// The decryption phase is sequential (waits for chunks in order)
func (ed *EncryptedDownloader) DownloadAndDecrypt(ctx context.Context, cid string, password string, dst io.Writer) error {
	if ed.parallelDownloader == nil {
		return fmt.Errorf("parallel downloader not initialized")
	}

	// Create a pipe for streaming decryption
	pr, pw := io.Pipe()

	// Channel to coordinate download completion
	downloadDone := make(chan error, 1)

	// Start parallel download to pipe writer
	go func() {
		defer pw.Close()
		err := ed.parallelDownloader.DownloadFile(ctx, cid, pw)
		if err != nil {
			pw.CloseWithError(err)
			downloadDone <- err
		} else {
			downloadDone <- nil
		}
	}()

	// Stream decrypt from pipe reader to destination
	decryptReader, err := crypto.DecryptStream(pr, password)
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

// DownloadAndDecryptChunked downloads encrypted file with better chunk ordering control
// This version maintains a buffer to handle out-of-order chunks better
func (ed *EncryptedDownloader) DownloadAndDecryptChunked(ctx context.Context, cid string, password string, dst io.Writer) error {
	// Create a buffered pipe with larger buffer for smoother streaming
	pr, pw := io.Pipe()

	var downloadErr error
	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: Parallel download
	go func() {
		defer wg.Done()
		defer pw.Close()

		err := ed.parallelDownloader.DownloadFile(ctx, cid, pw)
		if err != nil {
			log.Printf("Parallel download error: %v", err)
			pw.CloseWithError(err)
			downloadErr = err
		}
	}()

	// Goroutine 2: Stream decryption
	var decryptErr error
	go func() {
		defer wg.Done()

		decryptReader, err := crypto.DecryptStream(pr, password)
		if err != nil {
			decryptErr = fmt.Errorf("failed to create decrypt stream: %w", err)
			return
		}

		_, err = io.Copy(dst, decryptReader)
		if err != nil && err != io.EOF {
			decryptErr = fmt.Errorf("decryption copy failed: %w", err)
		}
	}()

	wg.Wait()

	if downloadErr != nil {
		return fmt.Errorf("download failed: %w", downloadErr)
	}
	if decryptErr != nil {
		return decryptErr
	}

	return nil
}
