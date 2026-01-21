package core

import (
	"context"
	"fmt"
	"io"
	"log"
	"sync"

	"github.com/ipfs/boxo/ipld/merkledag"
	"github.com/ipfs/boxo/path"
)

type ChunkTask struct {
	CID   string
	Index int
	Size  uint64
}

type ChunkResult struct {
	Index int
	Data  []byte
	Error error
}

type ParallelDownloader struct {
	node           *MochiNode
	booster        *DownloadBooster
	maxConcurrency int
}

func NewParallelDownloader(node *MochiNode, booster *DownloadBooster) *ParallelDownloader {
	return &ParallelDownloader{
		node:           node,
		booster:        booster,
		maxConcurrency: 8,
	}
}

// DownloadFile downloads a file with parallel chunk fetching for large files
func (pd *ParallelDownloader) DownloadFile(ctx context.Context, cid string, dst io.Writer) error {
	if pd.node == nil {
		return fmt.Errorf("node not initialized")
	}

	// Try to get file links (DAG structure)
	links, totalSize, err := pd.getFileLinks(ctx, cid)
	if err != nil || len(links) <= 1 {
		// Fallback to sequential download for small files or single-block files
		log.Printf("Using sequential download for CID %s (links: %d)", cid, len(links))
		return pd.fallbackDownload(ctx, cid, dst)
	}

	// Use parallel download for multi-chunk files
	log.Printf("Starting parallel download for CID %s: %d chunks, total %d bytes", cid, len(links), totalSize)
	return pd.parallelDownload(ctx, links, dst)
}

// getFileLinks extracts the DAG links (chunks) from a UnixFS file
func (pd *ParallelDownloader) getFileLinks(ctx context.Context, cidStr string) ([]ChunkTask, uint64, error) {
	cidPath, err := path.NewPath("/ipfs/" + cidStr)
	if err != nil {
		return nil, 0, err
	}

	// Get the DAG node
	dagAPI := pd.node.IPFS.Dag()

	// Resolve the path to get root node
	resolved, _, err := pd.node.IPFS.ResolvePath(ctx, cidPath)
	if err != nil {
		return nil, 0, err
	}

	rootCid := resolved.RootCid()

	// Get the node from DAG service
	node, err := dagAPI.Get(ctx, rootCid)
	if err != nil {
		return nil, 0, err
	}

	var links []ChunkTask
	var totalSize uint64

	// Try to decode as ProtoNode (most common for UnixFS files)
	protoNode, ok := node.(*merkledag.ProtoNode)
	if !ok {
		// Not a ProtoNode, probably a single-block file
		return nil, 0, fmt.Errorf("not a multi-block file")
	}

	nodeLinks := protoNode.Links()
	if len(nodeLinks) == 0 {
		return nil, 0, fmt.Errorf("no links found in DAG node")
	}

	for i, link := range nodeLinks {
		links = append(links, ChunkTask{
			CID:   link.Cid.String(),
			Index: i,
			Size:  link.Size,
		})
		totalSize += link.Size
	}

	return links, totalSize, nil
}

// parallelDownload fetches chunks in parallel and writes them sequentially
func (pd *ParallelDownloader) parallelDownload(ctx context.Context, tasks []ChunkTask, dst io.Writer) error {
	chunks := make(chan ChunkTask, len(tasks))
	results := make(chan ChunkResult, len(tasks))

	// Start worker pool
	var wg sync.WaitGroup
	for i := 0; i < pd.maxConcurrency; i++ {
		wg.Add(1)
		go pd.chunkWorker(ctx, chunks, results, &wg)
	}

	// Distribute tasks
	for _, task := range tasks {
		chunks <- task
	}
	close(chunks)

	// Wait for all workers to finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect and merge results
	return pd.mergeChunks(results, len(tasks), dst)
}

// chunkWorker fetches individual chunks
func (pd *ParallelDownloader) chunkWorker(ctx context.Context, tasks <-chan ChunkTask, results chan<- ChunkResult, wg *sync.WaitGroup) {
	defer wg.Done()

	for task := range tasks {
		select {
		case <-ctx.Done():
			results <- ChunkResult{Index: task.Index, Error: ctx.Err()}
			return
		default:
		}

		// Fetch chunk from IPFS
		reader, err := pd.node.GetFile(ctx, task.CID)
		if err != nil {
			log.Printf("Failed to fetch chunk %d (CID: %s): %v", task.Index, task.CID, err)
			results <- ChunkResult{Index: task.Index, Error: err}
			continue
		}

		// Read chunk data (chunks are typically 256KB-1MB, acceptable in memory)
		data, err := io.ReadAll(reader)
		if err != nil {
			log.Printf("Failed to read chunk %d: %v", task.Index, err)
			results <- ChunkResult{Index: task.Index, Error: err}
			continue
		}

		results <- ChunkResult{
			Index: task.Index,
			Data:  data,
		}
	}
}

// mergeChunks assembles chunks in order and writes to destination
func (pd *ParallelDownloader) mergeChunks(results <-chan ChunkResult, totalChunks int, dst io.Writer) error {
	chunkMap := make(map[int][]byte)
	var lastError error

	// Collect all results
	for result := range results {
		if result.Error != nil {
			lastError = result.Error
			continue
		}
		chunkMap[result.Index] = result.Data
	}

	// Check if we got all chunks
	if len(chunkMap) < totalChunks {
		if lastError != nil {
			return fmt.Errorf("incomplete download: got %d/%d chunks, last error: %w", len(chunkMap), totalChunks, lastError)
		}
		return fmt.Errorf("incomplete download: got %d/%d chunks", len(chunkMap), totalChunks)
	}

	// Write chunks in order
	for i := 0; i < totalChunks; i++ {
		data, ok := chunkMap[i]
		if !ok {
			return fmt.Errorf("missing chunk %d", i)
		}

		if _, err := dst.Write(data); err != nil {
			return fmt.Errorf("failed to write chunk %d: %w", i, err)
		}
	}

	log.Printf("Successfully merged %d chunks", totalChunks)
	return nil
}

// fallbackDownload uses sequential download from IPFS
func (pd *ParallelDownloader) fallbackDownload(ctx context.Context, cid string, dst io.Writer) error {
	reader, err := pd.node.GetFile(ctx, cid)
	if err != nil {
		return fmt.Errorf("failed to get file: %w", err)
	}

	_, err = io.Copy(dst, reader)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// SetMaxConcurrency sets the maximum number of parallel chunk downloads
func (pd *ParallelDownloader) SetMaxConcurrency(n int) {
	if n > 0 && n <= 32 {
		pd.maxConcurrency = n
	}
}
