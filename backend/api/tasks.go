package api

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
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
	Phase  string // preparing, warming, fetching_size, downloading
	Error  string

	Loaded int64
	Total  int64
	Speed  float64

	CreatedAt time.Time
	UpdatedAt time.Time

	Password       string
	EncryptionType string // "password" or "private"
	EncryptionMeta string // Salt (hex) for password, EncryptedKey (base64) for private

	cancel context.CancelFunc
}

type downloadTaskDTO struct {
	ID        string  `json:"id"`
	FileID    uint    `json:"file_id"`
	CID       string  `json:"cid"`
	Name      string  `json:"name"`
	DestPath  string  `json:"dest_path"`
	Status    string  `json:"status"`
	Phase     string  `json:"phase,omitempty"`
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
		Phase:     t.Phase,
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
			download.GET("/:id/stream", s.handleDownloadTaskStream)
			download.POST("/:id/pause", s.handleDownloadTaskPause)
			download.POST("/:id/resume", s.handleDownloadTaskResume)
			download.POST("/:id/cancel", s.handleDownloadTaskCancel)
		}
	}
}

func (s *Server) handleDownloadTaskStart(c *gin.Context) {
	var req struct {
		FileID         uint   `json:"file_id"`
		CID            string `json:"cid"`
		Name           string `json:"name"`
		Password       string `json:"password"`
		EncryptionType string `json:"encryption_type"`
		EncryptionMeta string `json:"encryption_meta"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var file db.File
	if req.FileID > 0 {
		if err := s.DB.First(&file, req.FileID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
			return
		}
	} else if req.CID != "" {
		file.CID = req.CID
		file.Name = req.Name
		if file.Name == "" {
			file.Name = req.CID
		}
		file.EncryptionType = req.EncryptionType
		file.EncryptionMeta = req.EncryptionMeta
		file.Size = 0
	} else {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Either file_id or cid must be provided"})
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
		ID:             id,
		FileID:         req.FileID,
		CID:            file.CID,
		Name:           file.Name,
		DestPath:       dstPath,
		Status:         "running",
		Loaded:         0,
		Total:          file.Size,
		Speed:          0,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
		Password:       req.Password,
		EncryptionType: file.EncryptionType,
		EncryptionMeta: file.EncryptionMeta,
		cancel:         cancel,
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

func (s *Server) handleDownloadTaskStream(c *gin.Context) {
	id := strings.TrimSpace(c.Param("id"))
	task := s.getDownloadTask(id)
	if task == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Task not found"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			task := s.getDownloadTask(id)
			if task == nil {
				return
			}

			c.SSEvent("progress", task.snapshot())
			c.Writer.Flush()

			task.mu.Lock()
			status := task.Status
			task.mu.Unlock()

			if status == "completed" || status == "error" || status == "canceled" {
				return
			}
		}
	}
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
	// Helper to update phase
	setPhase := func(phase string) {
		task.mu.Lock()
		task.Phase = phase
		task.UpdatedAt = time.Now()
		task.mu.Unlock()
	}

	// Step 1: Preparing phase - boost connections
	setPhase("preparing")
	log.Printf("Task %s: Boosting connection limits for %s", task.ID, task.CID)
	if err := s.ConnectionManager.BoostForDownload(task.CID); err != nil {
		log.Printf("Warning: Failed to boost connections: %v", err)
	}
	defer s.ConnectionManager.RestoreDefaults(task.CID)

	// Step 2: Parallel warmup and size fetch for faster startup
	setPhase("connecting")
	log.Printf("Task %s: Warming up providers and fetching size for %s", task.ID, task.CID)

	var wg sync.WaitGroup
	warmupDone := make(chan struct{})

	// Check if providers are already cached (skip warmup on resume)
	alreadyCached := s.DownloadBooster.HasCachedProviders(task.CID)
	if alreadyCached {
		log.Printf("Task %s: Providers already cached, skipping warmup", task.ID)
	} else {
		// Warmup in background only if not cached
		wg.Add(1)
		go func() {
			defer wg.Done()
			warmupCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
			defer cancel()
			if err := s.DownloadBooster.WarmupCID(warmupCtx, task.CID); err != nil {
				log.Printf("Task %s: Warmup failed: %v, continuing anyway", task.ID, err)
			} else {
				log.Printf("Task %s: Warmup completed successfully", task.ID)
			}
		}()
	}

	// Fetch size in parallel if not already set
	if task.Total == 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sizeCtx, sizeCancel := context.WithTimeout(ctx, 10*time.Second)
			defer sizeCancel()
			if size, err := s.Node.GetFileSize(sizeCtx, task.CID); err == nil {
				task.mu.Lock()
				task.Total = size
				task.UpdatedAt = time.Now()
				task.mu.Unlock()
				log.Printf("Task %s: File size retrieved: %d bytes", task.ID, size)
			} else {
				log.Printf("Task %s: Failed to get file size: %v", task.ID, err)
			}
		}()
	}

	// Wait for both to complete
	go func() {
		wg.Wait()
		close(warmupDone)
	}()

	select {
	case <-warmupDone:
		// Both completed
	case <-ctx.Done():
		return
	}

	// Step 3: Check for existing partial download
	offset := int64(0)
	if st, err := os.Stat(task.DestPath); err == nil {
		offset = st.Size()
		log.Printf("Task %s: Resuming from offset %d", task.ID, offset)
	}

	task.mu.Lock()
	task.Loaded = offset
	task.UpdatedAt = time.Now()
	task.mu.Unlock()

	// Step 4: Determine download strategy based on encryption type
	useEncryptedDownload := false
	var decryptKey []byte

	if task.EncryptionType == "password" && task.Password != "" {
		// Password-based encryption: derive key from password + salt
		salt, err := hex.DecodeString(task.EncryptionMeta)
		if err != nil {
			log.Printf("Task %s: Invalid salt in encryption_meta: %v", task.ID, err)
			task.mu.Lock()
			task.Status = "error"
			task.Error = "Invalid encryption metadata"
			task.UpdatedAt = time.Now()
			task.mu.Unlock()
			return
		}
		decryptKey = crypto.DeriveKey(task.Password, salt)
		useEncryptedDownload = true
		log.Printf("Task %s: Using password-based decryption", task.ID)
	} else if task.EncryptionType == "private" {
		// Private encryption: decrypt session key with user's private key
		if s.AccountManager == nil || s.AccountManager.Wallet == nil {
			log.Printf("Task %s: Wallet not available for private decryption", task.ID)
			task.mu.Lock()
			task.Status = "error"
			task.Error = "Account not unlocked"
			task.UpdatedAt = time.Now()
			task.mu.Unlock()
			return
		}
		encKeyBytes, err := base64.StdEncoding.DecodeString(task.EncryptionMeta)
		if err != nil {
			log.Printf("Task %s: Invalid encrypted key: %v", task.ID, err)
			task.mu.Lock()
			task.Status = "error"
			task.Error = "Invalid encryption metadata"
			task.UpdatedAt = time.Now()
			task.mu.Unlock()
			return
		}
		decryptKey, err = s.AccountManager.Wallet.DecryptSessionKey(encKeyBytes)
		if err != nil {
			log.Printf("Task %s: Failed to decrypt session key: %v", task.ID, err)
			task.mu.Lock()
			task.Status = "error"
			task.Error = "Failed to decrypt with your private key"
			task.UpdatedAt = time.Now()
			task.mu.Unlock()
			return
		}
		useEncryptedDownload = true
		log.Printf("Task %s: Using private key decryption", task.ID)
	}

	// Step 5: Open destination file with AsyncWriter
	var dstWriter io.WriteCloser
	var err error

	if offset > 0 {
		// Resume mode not supported for parallel downloads yet
		log.Printf("Task %s: Resume not supported for parallel download, restarting", task.ID)
		offset = 0
		os.Remove(task.DestPath)
		// Reset loaded counter to avoid accumulation bug
		task.mu.Lock()
		task.Loaded = 0
		task.mu.Unlock()
	}

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

	// Create progress callback for tracking download progress
	progressCallback := func(delta int64) {
		if delta <= 0 {
			return
		}
		task.mu.Lock()
		task.Loaded += delta
		task.UpdatedAt = time.Now()
		task.mu.Unlock()
	}

	// Start speed monitor goroutine
	stopSpeedMonitor := make(chan struct{})
	speedMonitorDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		defer close(speedMonitorDone)

		var lastLoaded int64
		lastTime := time.Now()

		for {
			select {
			case <-stopSpeedMonitor:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				task.mu.Lock()
				now := time.Now()
				currentLoaded := task.Loaded
				dt := now.Sub(lastTime).Seconds()

				if dt > 0 {
					deltaBytes := currentLoaded - lastLoaded
					instantSpeed := float64(deltaBytes) / dt

					if task.Speed == 0 {
						task.Speed = instantSpeed
					} else {
						task.Speed = task.Speed*0.3 + instantSpeed*0.7
					}

					lastLoaded = currentLoaded
					lastTime = now
				}

				task.UpdatedAt = now
				task.mu.Unlock()
			}
		}
	}()

	// Callback for receiving total file size from parallel downloader
	totalSizeCallback := func(total int64) {
		task.mu.Lock()
		if task.Total == 0 && total > 0 {
			task.Total = total
			task.UpdatedAt = time.Now()
			log.Printf("Task %s: Total size updated from parallel downloader: %d bytes", task.ID, total)
		}
		task.mu.Unlock()
	}

	// Step 6: Execute download with appropriate strategy
	setPhase("downloading")
	var downloadErr error

	if useEncryptedDownload {
		// Encrypted download with pre-derived key
		log.Printf("Task %s: Starting encrypted download", task.ID)
		encryptedDL := core.NewEncryptedDownloader(s.ParallelDownloader)
		downloadErr = encryptedDL.DownloadAndDecrypt(ctx, task.CID, decryptKey, dstWriter, progressCallback)
	} else {
		// Standard download (works for files >= 1MB)
		log.Printf("Task %s: Starting download", task.ID)
		downloadErr = s.ParallelDownloader.DownloadFile(ctx, task.CID, dstWriter, progressCallback, totalSizeCallback)
	}

	// Stop speed monitor
	close(stopSpeedMonitor)
	<-speedMonitorDone

	// Step 7: Handle result
	if downloadErr != nil {
		if ctx.Err() != nil {
			// Context cancelled (user paused/cancelled)
			return
		}

		log.Printf("Task %s: Download failed: %v", task.ID, downloadErr)
		task.mu.Lock()
		task.Status = "error"
		task.Error = downloadErr.Error()
		task.UpdatedAt = time.Now()
		task.mu.Unlock()
		return
	}

	// Success
	task.mu.Lock()
	task.Status = "completed"
	task.Phase = ""
	task.Speed = 0
	task.UpdatedAt = time.Now()
	task.mu.Unlock()

	log.Printf("Task %s: Download completed successfully", task.ID)
}

func urlQueryEscape(s string) string {
	return url.QueryEscape(s)
}
