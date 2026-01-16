package api

import (
	"fmt"
	"net/http"
	"time"

	"mochibox-core/db"

	"github.com/gin-gonic/gin"
)

func (s *Server) registerSharedRoutes(api *gin.RouterGroup) {
	shared := api.Group("/shared")
	{
		shared.POST("/history", s.handleAddSharedHistory)
		shared.GET("/history", s.handleListSharedHistory)
		shared.DELETE("/history/:id", s.handleDeleteSharedHistory)
		shared.DELETE("/history", s.handleClearSharedHistory)
		shared.POST("/pin", s.handlePinShared)
	}
}

func (s *Server) handlePinShared(c *gin.Context) {
	var req struct {
		CID string `json:"cid" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// This might block, but we want that for now so frontend knows when it's done
	// Ideally, for very large files, this should be async or we rely on IPFS background fetching.
	// But `ipfs pin add` blocks until complete.
	if err := s.Node.Pin(c.Request.Context(), req.CID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to pin: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "pinned", "cid": req.CID})
}

func (s *Server) handleListSharedHistory(c *gin.Context) {
	var history []db.SharedFile
	if err := s.DB.Order("created_at desc").Find(&history).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch history"})
		return
	}
	c.JSON(http.StatusOK, history)
}

func (s *Server) handleAddSharedHistory(c *gin.Context) {
	var req struct {
		CID  string `json:"cid" binding:"required"`
		Name string `json:"name"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if already exists? Maybe just update timestamp?
	// User might want to keep history of same file opened at different times?
	// But let's assume we want unique entries by CID to keep list clean.
	var existing db.SharedFile
	if err := s.DB.Where("cid = ?", req.CID).First(&existing).Error; err == nil {
		// Update timestamp
		existing.CreatedAt = time.Now()
		// Update name if provided and existing is empty or generic
		if req.Name != "" {
			existing.Name = req.Name
		}
		s.DB.Save(&existing)
		c.JSON(http.StatusOK, existing)
		return
	}

	// Create new
	file := db.SharedFile{
		CID:       req.CID,
		Name:      req.Name,
		CreatedAt: time.Now(),
		MimeType:  "application/octet-stream", // Default
	}

	if file.Name == "" {
		file.Name = "Shared-" + req.CID[:8]
	}

	// Try to fetch size from IPFS
	// We do this asynchronously or synchronously? 
	// Sync is better for UI to show size immediately.
	// But it might be slow if node is slow.
	// Let's try with a short timeout context
	// Actually Node.GetFileSize calls Unixfs().Get() which is fast for local/cached, 
	// but might hang if content is not found.
	// We'll proceed without size if it fails quickly, or just let it block a bit.
	
	size, err := s.Node.GetFileSize(c.Request.Context(), req.CID)
	if err == nil {
		file.Size = size
	} else {
		fmt.Printf("Warning: Failed to get size for %s: %v\n", req.CID, err)
	}

	if err := s.DB.Create(&file).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save history"})
		return
	}

	c.JSON(http.StatusOK, file)
}

func (s *Server) handleDeleteSharedHistory(c *gin.Context) {
	id := c.Param("id")
	if err := s.DB.Delete(&db.SharedFile{}, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete item"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

func (s *Server) handleClearSharedHistory(c *gin.Context) {
	// Delete all
	if err := s.DB.Exec("DELETE FROM shared_files").Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to clear history"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cleared"})
}
