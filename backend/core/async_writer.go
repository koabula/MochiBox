package core

import (
	"bufio"
	"os"
	"sync"
)

// AsyncWriter provides buffered asynchronous disk writing
// It reduces disk I/O wait time by batching writes and flushing asynchronously
type AsyncWriter struct {
	file      *os.File
	buffered  *bufio.Writer
	flushChan chan struct{}
	errChan   chan error
	wg        sync.WaitGroup
	mu        sync.Mutex
	closed    bool
}

// NewAsyncWriter creates a new async writer with specified buffer size
func NewAsyncWriter(path string, bufferSize int) (*AsyncWriter, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}

	aw := &AsyncWriter{
		file:      file,
		buffered:  bufio.NewWriterSize(file, bufferSize),
		flushChan: make(chan struct{}, 1),
		errChan:   make(chan error, 1),
	}

	// Start async flush worker
	aw.wg.Add(1)
	go aw.flushWorker()

	return aw, nil
}

// OpenAsyncWriter opens an existing file for async writing (for resume support)
func OpenAsyncWriter(path string, bufferSize int, append bool) (*AsyncWriter, error) {
	var file *os.File
	var err error

	if append {
		file, err = os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0644)
	} else {
		file, err = os.OpenFile(path, os.O_WRONLY, 0644)
	}

	if err != nil {
		return nil, err
	}

	aw := &AsyncWriter{
		file:      file,
		buffered:  bufio.NewWriterSize(file, bufferSize),
		flushChan: make(chan struct{}, 1),
		errChan:   make(chan error, 1),
	}

	// Start async flush worker
	aw.wg.Add(1)
	go aw.flushWorker()

	return aw, nil
}

// Write implements io.Writer interface
func (aw *AsyncWriter) Write(p []byte) (n int, err error) {
	aw.mu.Lock()
	defer aw.mu.Unlock()

	if aw.closed {
		return 0, os.ErrClosed
	}

	n, err = aw.buffered.Write(p)
	if err != nil {
		return n, err
	}

	// Trigger async flush when buffer reaches 10MB
	if aw.buffered.Buffered() >= 10*1024*1024 {
		select {
		case aw.flushChan <- struct{}{}:
		default:
			// Already a flush pending, skip
		}
	}

	return n, nil
}

// flushWorker runs in a separate goroutine to handle async flushing
func (aw *AsyncWriter) flushWorker() {
	defer aw.wg.Done()

	for range aw.flushChan {
		aw.mu.Lock()
		if aw.closed {
			aw.mu.Unlock()
			return
		}

		err := aw.buffered.Flush()
		if err != nil {
			select {
			case aw.errChan <- err:
			default:
			}
			aw.mu.Unlock()
			return
		}

		// Sync to disk (optional for SSD, but ensures data integrity)
		// Comment out for better performance on SSD
		aw.file.Sync()

		aw.mu.Unlock()
	}
}

// Seek implements io.Seeker interface for resume support
func (aw *AsyncWriter) Seek(offset int64, whence int) (int64, error) {
	aw.mu.Lock()
	defer aw.mu.Unlock()

	// Flush buffered data before seeking
	if err := aw.buffered.Flush(); err != nil {
		return 0, err
	}

	return aw.file.Seek(offset, whence)
}

// Close flushes all buffered data and closes the file
func (aw *AsyncWriter) Close() error {
	aw.mu.Lock()
	if aw.closed {
		aw.mu.Unlock()
		return nil
	}
	aw.closed = true
	aw.mu.Unlock()

	// Stop flush worker
	close(aw.flushChan)
	aw.wg.Wait()

	// Check for flush errors
	select {
	case err := <-aw.errChan:
		aw.file.Close()
		return err
	default:
	}

	// Final flush
	if err := aw.buffered.Flush(); err != nil {
		aw.file.Close()
		return err
	}

	// Sync to ensure all data is written
	if err := aw.file.Sync(); err != nil {
		aw.file.Close()
		return err
	}

	return aw.file.Close()
}
