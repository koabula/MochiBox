package api

import (
	"context"
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
		shared.GET("/search/:cid", s.handleSearchShared)
		shared.POST("/connect", s.handleSharedConnect)
	}
}

func (s *Server) handleSearchShared(c *gin.Context) {
	cid := c.Param("cid")

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")

	ctx := c.Request.Context()

	provs, err := s.Node.FindProviders(ctx, cid)
	if err != nil {
		c.SSEvent("error", err.Error())
		return
	}

	count := 0
	// Notify start
	c.SSEvent("status", "searching")
	c.Writer.Flush()

	for p := range provs {
		count++
		data := map[string]interface{}{
			"peers": count,
			"found": p.ID.String(),
		}
		c.SSEvent("update", data)
		c.Writer.Flush()
	}

	c.SSEvent("done", gin.H{"total": count})
}

func (s *Server) handleSharedConnect(c *gin.Context) {
	var req struct {
		Peers []string `json:"peers"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	count := 0
	for _, p := range req.Peers {
		// Async connect to avoid blocking
		go func(addr string) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			s.Node.Connect(ctx, addr)
		}(p)
		count++
	}

	c.JSON(http.StatusOK, gin.H{"status": "connecting", "count": count})
}

func (s *Server) handlePinShared(c *gin.Context) {
	var req struct {
		CID            string `json:"cid" binding:"required"`
		EncryptionType string `json:"encryption_type"`
		EncryptionMeta string `json:"encryption_meta"`
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

	// Add to My Files (DB) if not exists
	var count int64
	s.DB.Model(&db.File{}).Where("cid = ?", req.CID).Count(&count)
	if count == 0 {
		newFile := db.File{
			CID:            req.CID,
			CreatedAt:      time.Now(),
			MimeType:       "application/octet-stream",
			EncryptionType: req.EncryptionType,
			EncryptionMeta: req.EncryptionMeta,
		}
        
        if newFile.EncryptionType == "" {
            newFile.EncryptionType = "public"
        }

		// Try to find name from Shared History
		var sharedFile db.SharedFile
		if err := s.DB.Where("cid = ?", req.CID).First(&sharedFile).Error; err == nil {
			newFile.Name = sharedFile.Name
			if sharedFile.MimeType != "" {
				newFile.MimeType = sharedFile.MimeType
			}
			if sharedFile.Size > 0 {
				newFile.Size = sharedFile.Size
			}
		}

		// If still no name, use default
		if newFile.Name == "" {
			newFile.Name = "Pinned-" + req.CID[:8]
		}

		// Get Size
		if newFile.Size == 0 {
			size, err := s.Node.GetFileSize(c.Request.Context(), req.CID)
			if err == nil {
				newFile.Size = size
			}
		}

		if err := s.DB.Create(&newFile).Error; err != nil {
			fmt.Printf("Warning: Failed to add pinned file to DB: %v\n", err)
		}
	} else {
        // Update encryption metadata if existing record is missing it (e.g. was public, now known private)
        // Or if we re-pin a shared file we now have keys for.
        // Let's update it if provided.
        if req.EncryptionType != "" {
            s.DB.Model(&db.File{}).Where("cid = ?", req.CID).Updates(map[string]interface{}{
                "encryption_type": req.EncryptionType,
                "encryption_meta": req.EncryptionMeta,
            })
        }
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
		CID            string `json:"cid" binding:"required"`
		Name           string `json:"name"`
		Size           int64  `json:"size"`
		MimeType       string `json:"mime_type"`
		EncryptionType string `json:"encryption_type"`
		EncryptionMeta string `json:"encryption_meta"`
		OriginalLink   string `json:"original_link"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Check if already exists? Maybe just update timestamp?
	// User might want to keep history of same file opened at different times?
	// But let's assume we want unique entries by CID to keep list clean.
	var existing db.SharedFile
	// Suppress record not found error check or handle explicitly
	err := s.DB.Where("cid = ?", req.CID).First(&existing).Error
	if err == nil {
		// Update timestamp
		existing.CreatedAt = time.Now()
		// Update fields if provided
		if req.Name != "" {
			existing.Name = req.Name
		}
		if req.Size > 0 {
			existing.Size = req.Size
		}
		if req.MimeType != "" {
			existing.MimeType = req.MimeType
		}
		if req.EncryptionType != "" {
			existing.EncryptionType = req.EncryptionType
			existing.EncryptionMeta = req.EncryptionMeta
		}
		if req.OriginalLink != "" {
			existing.OriginalLink = req.OriginalLink
		}
		
		s.DB.Save(&existing)
		c.JSON(http.StatusOK, existing)
		return
	}

	// Create new
	file := db.SharedFile{
		CID:            req.CID,
		Name:           req.Name,
		Size:           req.Size,
		CreatedAt:      time.Now(),
		MimeType:       req.MimeType,
		EncryptionType: req.EncryptionType,
		EncryptionMeta: req.EncryptionMeta,
		OriginalLink:   req.OriginalLink,
	}

	if file.MimeType == "" {
		file.MimeType = "application/octet-stream"
	}
	
	if file.EncryptionType == "" {
	    file.EncryptionType = "public"
	}

	if file.Name == "" {
		file.Name = "Shared-" + req.CID[:8]
	}

	if err := s.DB.Create(&file).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save history"})
		return
	}

	// Async fetch size if unknown
	if file.Size == 0 {
		go func(cid string) {
			// Use a background context with timeout
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			size, err := s.Node.GetFileSize(ctx, cid)
			if err == nil && size > 0 {
				s.DB.Model(&db.SharedFile{}).Where("cid = ?", cid).Update("size", size)
			} else if err != nil {
				fmt.Printf("Warning: Failed to get size for %s: %v\n", cid, err)
			}
		}(file.CID)
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
