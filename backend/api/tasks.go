package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

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
	offset := int64(0)
	if st, err := os.Stat(task.DestPath); err == nil {
		offset = st.Size()
	}

	task.mu.Lock()
	task.Loaded = offset
	task.UpdatedAt = time.Now()
	task.mu.Unlock()

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
		_ = os.Truncate(task.DestPath, 0)
	}

	dst, err := os.OpenFile(task.DestPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		task.mu.Lock()
		task.Status = "error"
		task.Error = err.Error()
		task.UpdatedAt = time.Now()
		task.mu.Unlock()
		return
	}
	defer dst.Close()

	if _, err := dst.Seek(offset, io.SeekStart); err != nil {
		task.mu.Lock()
		task.Status = "error"
		task.Error = err.Error()
		task.UpdatedAt = time.Now()
		task.mu.Unlock()
		return
	}

	buf := make([]byte, 256*1024)
	lastSpeedAt := time.Now()
	lastSpeedLoaded := offset

	for {
		if ctx.Err() != nil {
			return
		}
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
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

func urlQueryEscape(s string) string {
	return url.QueryEscape(s)
}
