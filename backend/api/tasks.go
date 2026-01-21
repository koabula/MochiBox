package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"mochibox-core/core"
	"mochibox-core/crypto"
	"mochibox-core/db"

	"github.com/gin-gonic/gin"
)

type DownloadTask struct {
	mu sync.Mutex

	ID       string
	FileID   uint
	CID      string
	Name     string
	DestPath string

	Status string
	Error  string

	Loaded int64
	Total  int64
	Speed  float64

	CreatedAt time.Time
	UpdatedAt time.Time

	Password string

	cancel context.CancelFunc
}

type downloadTaskDTO struct {
	ID        string  `json:"id"`
	FileID    uint    `json:"file_id"`
	CID       string  `json:"cid"`
	Name      string  `json:"name"`
	DestPath  string  `json:"dest_path"`
	Status    string  `json:"status"`
	Error     string  `json:"error,omitempty"`
	Loaded    int64   `json:"loaded"`
	Total     int64   `json:"total"`
	Speed     float64 `json:"speed"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

func (t *DownloadTask) snapshot() downloadTaskDTO {
	t.mu.Lock()
	defer t.mu.Unlock()

	return downloadTaskDTO{
		ID:        t.ID,
		FileID:    t.FileID,
		CID:       t.CID,
		Name:      t.Name,
		DestPath:  t.DestPath,
		Status:    t.Status,
		Error:     t.Error,
		Loaded:    t.Loaded,
		Total:     t.Total,
		Speed:     t.Speed,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
		UpdatedAt: t.UpdatedAt.Format(time.RFC3339),
	}
}

func newTaskID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func localBaseURL() string {
	port := strings.TrimSpace(os.Getenv("MOCHIBOX_PORT"))
	if port == "" {
		port = "3666"
	}
	if _, err := strconv.Atoi(port); err != nil {
		port = "3666"
	}
	return "http://127.0.0.1:" + port
}

func (s *Server) registerTaskRoutes(api *gin.RouterGroup) {
	tasks := api.Group("/tasks")
	{
		download := tasks.Group("/download")
		{
			download.POST("/start", s.handleDownloadTaskStart)
			download.GET("/:id", s.handleDownloadTaskGet)
			download.POST("/:id/pause", s.handleDownloadTaskPause)
			download.POST("/:id/resume", s.handleDownloadTaskResume)
			download.POST("/:id/cancel", s.handleDownloadTaskCancel)
		}
	}
}

func (s *Server) handleDownloadTaskStart(c *gin.Context) {
	var req struct {
		FileID   uint   `json:"file_id" binding:"required"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var file db.File
	if err := s.DB.First(&file, req.FileID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	var settings db.Settings
	s.DB.First(&settings)
	saveDir := settings.DownloadPath
	if saveDir == "" {
		home, _ := os.UserHomeDir()
		saveDir = filepath.Join(home, "Downloads")
	}
	if err := os.MkdirAll(saveDir, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create download directory"})
		return
	}

	dstPath := ensureUniquePath(filepath.Join(saveDir, file.Name))
	if file.MimeType == "inode/directory" && !strings.HasSuffix(strings.ToLower(dstPath), ".zip") {
		dstPath += ".zip"
	}

	id, err := newTaskID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to allocate task id"})
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	task := &DownloadTask{
		ID:        id,
		FileID:    req.FileID,
		CID:       file.CID,
		Name:      file.Name,
		DestPath:  dstPath,
		Status:    "running",
		Loaded:    0,
		Total:     file.Size,
		Speed:     0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Password:  req.Password,
		cancel:    cancel,
	}

	s.DownloadTasksMu.Lock()
	s.DownloadTasks[id] = task
	s.DownloadTasksMu.Unlock()

	go s.runDownloadTask(ctx, task)

	c.JSON(http.StatusOK, task.snapshot())
}

func (s *Server) handleDownloadTaskGet(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	task := s.getDownloadTask(id)
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}
	c.JSON(http.StatusOK, task.snapshot())
}

func (s *Server) handleDownloadTaskPause(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	task := s.getDownloadTask(id)
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	task.mu.Lock()
	if task.Status == "running" {
		task.Status = "paused"
		task.UpdatedAt = time.Now()
		if task.cancel != nil {
			task.cancel()
		}
	}
	task.mu.Unlock()

	c.JSON(http.StatusOK, task.snapshot())
}

func (s *Server) handleDownloadTaskResume(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	task := s.getDownloadTask(id)
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	task.mu.Lock()
	if task.Status != "paused" && task.Status != "error" {
		task.mu.Unlock()
		c.JSON(http.StatusBadRequest, gin.H{"error": "Task is not resumable in current state"})
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	task.cancel = cancel
	task.Status = "running"
	task.Error = ""
	task.Speed = 0
	task.UpdatedAt = time.Now()
	task.mu.Unlock()

	go s.runDownloadTask(ctx, task)

	c.JSON(http.StatusOK, task.snapshot())
}

func (s *Server) handleDownloadTaskCancel(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	task := s.getDownloadTask(id)
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	task.mu.Lock()
	if task.Status == "running" || task.Status == "paused" {
		task.Status = "canceled"
		task.UpdatedAt = time.Now()
		if task.cancel != nil {
			task.cancel()
		}
	}
	task.mu.Unlock()

	_ = os.Remove(task.DestPath)

	c.JSON(http.StatusOK, task.snapshot())
}

func (s *Server) getDownloadTask(id string) *DownloadTask {
	s.DownloadTasksMu.Lock()
	defer s.DownloadTasksMu.Unlock()
	return s.DownloadTasks[id]
}

func (s *Server) runDownloadTask(ctx context.Context, task *DownloadTask) {
	// Boost connection limits for this download
	if err := s.ConnectionManager.BoostForDownload(task.CID); err != nil {
		log.Printf("Warning: Failed to boost connections: %v", err)
	}
	defer s.ConnectionManager.RestoreDefaults(task.CID)

	// Warmup providers if not already done
	go func() {
		warmupCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.DownloadBooster.WarmupCID(warmupCtx, task.CID); err != nil {
			log.Printf("Warning: Provider warmup failed: %v", err)
		}
	}()

	// Check for existing partial download
	offset := int64(0)
	if st, err := os.Stat(task.DestPath); err == nil {
		offset = st.Size()
	}

	task.mu.Lock()
	task.Loaded = offset
	task.UpdatedAt = time.Now()
	task.mu.Unlock()

	// Get file from IPFS (try parallel download for large files)
	var file db.File
	useParallel := false
	useOptimizedEncryption := false
	if err := s.DB.First(&file, task.FileID).Error; err == nil {
		// Use parallel download for files > 10MB without encryption
		if file.Size > 10*1024*1024 && offset == 0 && task.Password == "" {
			useParallel = true
		}
		// Use optimized encryption for password-protected files
		if task.Password != "" {
			useOptimizedEncryption = true
		}
	}

	// Open destination file with AsyncWriter for better performance
	var dstWriter io.WriteCloser
	var err error
	if offset > 0 {
		// Resume: use AsyncWriter in append mode
		dstWriter, err = core.OpenAsyncWriter(task.DestPath, 4*1024*1024, true)
	} else {
		// New download: create AsyncWriter
		dstWriter, err = core.NewAsyncWriter(task.DestPath, 4*1024*1024)
	}

	if err != nil {
		task.mu.Lock()
		task.Status = "error"
		task.Error = err.Error()
		task.UpdatedAt = time.Now()
		task.mu.Unlock()
		return
	}
	defer dstWriter.Close()

	// Create progress tracking writer
	progressWriter := &progressWriter{
		dst:  dstWriter,
		task: task,
		ctx:  ctx,
	}

	// Try parallel download for large files
	if useParallel {
		log.Printf("Using parallel download for task %s (CID: %s)", task.ID, task.CID)
		if err := s.ParallelDownloader.DownloadFile(ctx, task.CID, progressWriter); err != nil {
			log.Printf("Parallel download failed, falling back to sequential: %v", err)
			// Fallback to sequential download
			useParallel = false
		} else {
			// Success
			task.mu.Lock()
			task.Status = "completed"
			task.Speed = 0
			task.UpdatedAt = time.Now()
			task.mu.Unlock()
			return
		}
	}

	// Try optimized direct decryption for encrypted files
	if useOptimizedEncryption && offset == 0 {
		log.Printf("Using optimized encryption for task %s (CID: %s)", task.ID, task.CID)

		// Get encrypted stream directly from IPFS
		reader, err := s.Node.GetFile(ctx, task.CID)
		if err != nil {
			log.Printf("Failed to get encrypted stream: %v, falling back to HTTP", err)
			goto HttpFallback
		}

		// Create optimized decryption stream
		decryptedReader, err := crypto.DecryptStream(reader, task.Password)
		if err != nil {
			log.Printf("Failed to create decryption stream: %v, falling back to HTTP", err)
			goto HttpFallback
		}

		// Stream decrypted data to disk with progress tracking
		buf := make([]byte, 1024*1024) // 1MB buffer
		lastSpeedAt := time.Now()
		lastSpeedLoaded := task.Loaded

		for {
			if ctx.Err() != nil {
				return
			}

			n, rerr := decryptedReader.Read(buf)
			if n > 0 {
				if _, werr := progressWriter.Write(buf[:n]); werr != nil {
					task.mu.Lock()
					task.Status = "error"
					task.Error = werr.Error()
					task.UpdatedAt = time.Now()
					task.mu.Unlock()
					return
				}

				task.mu.Lock()
				now := time.Now()
				if now.Sub(lastSpeedAt) >= 500*time.Millisecond {
					dt := now.Sub(lastSpeedAt).Seconds()
					dl := task.Loaded - lastSpeedLoaded
					instant := float64(dl) / dt
					if task.Speed == 0 {
						task.Speed = instant
					} else {
						task.Speed = task.Speed*0.7 + instant*0.3
					}
					lastSpeedAt = now
					lastSpeedLoaded = task.Loaded
				}
				task.UpdatedAt = now
				task.mu.Unlock()
			}

			if rerr == io.EOF {
				task.mu.Lock()
				task.Status = "completed"
				task.Speed = 0
				task.UpdatedAt = time.Now()
				task.mu.Unlock()
				return
			}
			if rerr != nil {
				if ctx.Err() != nil {
					return
				}
				task.mu.Lock()
				task.Status = "error"
				task.Error = rerr.Error()
				task.UpdatedAt = time.Now()
				task.mu.Unlock()
				return
			}
		}
	}

HttpFallback:
	// Sequential download via HTTP (maintains compatibility with encryption/password)
	reqURL := fmt.Sprintf("%s/api/preview/%s?download=true", localBaseURL(), task.CID)
	if task.Password != "" {
		reqURL += "&password=" + urlQueryEscape(task.Password)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		task.mu.Lock()
		task.Status = "error"
		task.Error = err.Error()
		task.UpdatedAt = time.Now()
		task.mu.Unlock()
		return
	}

	if offset > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return
		}
		task.mu.Lock()
		task.Status = "error"
		task.Error = err.Error()
		task.UpdatedAt = time.Now()
		task.mu.Unlock()
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		task.mu.Lock()
		task.Status = "error"
		task.Error = fmt.Sprintf("upstream status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
		task.UpdatedAt = time.Now()
		task.mu.Unlock()
		return
	}

	if offset > 0 && resp.StatusCode == http.StatusOK {
		offset = 0
		task.mu.Lock()
		task.Loaded = 0
		task.UpdatedAt = time.Now()
		task.mu.Unlock()

		// For AsyncWriter, we need to seek and truncate
		if asyncWriter, ok := dstWriter.(io.Seeker); ok {
			asyncWriter.Seek(0, io.SeekStart)
			if f, ok := asyncWriter.(*core.AsyncWriter); ok {
				// Close and recreate to truncate
				f.Close()
				dstWriter, err = core.NewAsyncWriter(task.DestPath, 4*1024*1024)
				if err != nil {
					task.mu.Lock()
					task.Status = "error"
					task.Error = err.Error()
					task.UpdatedAt = time.Now()
					task.mu.Unlock()
					return
				}
				defer dstWriter.Close()
				progressWriter.dst = dstWriter
			}
		}
	}

	// Use larger buffer for better performance
	buf := make([]byte, 1024*1024) // 1MB buffer
	lastSpeedAt := time.Now()
	lastSpeedLoaded := task.Loaded

	for {
		if ctx.Err() != nil {
			return
		}

		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := dstWriter.Write(buf[:n]); werr != nil {
				task.mu.Lock()
				task.Status = "error"
				task.Error = werr.Error()
				task.UpdatedAt = time.Now()
				task.mu.Unlock()
				return
			}

			task.mu.Lock()
			task.Loaded += int64(n)
			now := time.Now()
			if now.Sub(lastSpeedAt) >= 500*time.Millisecond {
				dt := now.Sub(lastSpeedAt).Seconds()
				dl := task.Loaded - lastSpeedLoaded
				instant := float64(dl) / dt
				if task.Speed == 0 {
					task.Speed = instant
				} else {
					task.Speed = task.Speed*0.7 + instant*0.3
				}
				lastSpeedAt = now
				lastSpeedLoaded = task.Loaded
			}
			task.UpdatedAt = now
			task.mu.Unlock()
		}

		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			if ctx.Err() != nil {
				return
			}
			task.mu.Lock()
			task.Status = "error"
			task.Error = rerr.Error()
			task.UpdatedAt = time.Now()
			task.mu.Unlock()
			return
		}
	}

	task.mu.Lock()
	task.Status = "completed"
	task.Speed = 0
	task.UpdatedAt = time.Now()
	task.mu.Unlock()
}

// progressWriter wraps a writer to track download progress
type progressWriter struct {
	dst  io.Writer
	task *DownloadTask
	ctx  context.Context

	lastUpdate time.Time
	lastLoaded int64
}

func (pw *progressWriter) Write(p []byte) (n int, err error) {
	// Check context cancellation
	select {
	case <-pw.ctx.Done():
		return 0, pw.ctx.Err()
	default:
	}

	n, err = pw.dst.Write(p)
	if n > 0 {
		pw.task.mu.Lock()
		pw.task.Loaded += int64(n)
		now := time.Now()

		// Update speed every 500ms
		if now.Sub(pw.lastUpdate) >= 500*time.Millisecond {
			dt := now.Sub(pw.lastUpdate).Seconds()
			dl := pw.task.Loaded - pw.lastLoaded
			instant := float64(dl) / dt
			if pw.task.Speed == 0 {
				pw.task.Speed = instant
			} else {
				pw.task.Speed = pw.task.Speed*0.7 + instant*0.3
			}
			pw.lastUpdate = now
			pw.lastLoaded = pw.task.Loaded
		}

		pw.task.UpdatedAt = now
		pw.task.mu.Unlock()
	}
	return n, err
}

func urlQueryEscape(s string) string {
	return url.QueryEscape(s)
}
