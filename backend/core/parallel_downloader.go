package core

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"
)

type ParallelDownloader struct {
	node    *MochiNode
	booster *DownloadBooster
}

func NewParallelDownloader(node *MochiNode, booster *DownloadBooster) *ParallelDownloader {
	return &ParallelDownloader{
		node:    node,
		booster: booster,
	}
}

// DownloadFile downloads a file using IPFS streaming
// IPFS Bitswap internally handles parallel block fetching, so we use streaming for correctness
// progressCallback receives incremental byte counts when data is fetched
// totalSizeCallback is called once when total file size is determined (can be nil)
func (pd *ParallelDownloader) DownloadFile(ctx context.Context, cid string, dst io.Writer, progressCallback func(delta int64), totalSizeCallback func(total int64)) error {
	if pd.node == nil {
		return fmt.Errorf("node not initialized")
	}

	// Get file size first if callback provided
	if totalSizeCallback != nil {
		sizeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		if size, err := pd.node.GetFileSize(sizeCtx, cid); err == nil && size > 0 {
			totalSizeCallback(size)
		}
	}

	log.Printf("Streaming download for CID %s", cid)
	return pd.streamDownload(ctx, cid, dst, progressCallback)
}

// streamDownload uses IPFS streaming with progress tracking
func (pd *ParallelDownloader) streamDownload(ctx context.Context, cid string, dst io.Writer, progressCallback func(delta int64)) error {
	reader, err := pd.node.GetFile(ctx, cid)
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}

	buf := make([]byte, 256*1024) // 256KB buffer for efficient transfer
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		n, readErr := reader.Read(buf)
		if n > 0 {
			if _, writeErr := dst.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("failed to write: %w", writeErr)
			}
			if progressCallback != nil {
				progressCallback(int64(n))
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return fmt.Errorf("failed to read: %w", readErr)
		}
	}

	log.Printf("Streaming download completed for CID %s", cid)
	return nil
}
