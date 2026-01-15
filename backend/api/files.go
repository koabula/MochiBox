package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mochibox-core/db"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (s *Server) registerFileRoutes(db *gorm.DB) {
	api := s.Router.Group("/api/files")
	{
		api.POST("/upload", func(c *gin.Context) {
			s.handleUpload(c, db)
		})
		api.GET("", func(c *gin.Context) {
			s.handleListFiles(c, db)
		})
		api.DELETE("/:id", func(c *gin.Context) {
			s.handleDeleteFile(c, db)
		})
        api.POST("/:id/download", s.handleDownloadToDisk)
        api.POST("/download/shared", s.handleDownloadShared)
	}
}

// ensureUniquePath checks if the file exists and appends (1), (2), etc. if it does.
func ensureUniquePath(path string) string {
    if _, err := os.Stat(path); os.IsNotExist(err) {
        return path
    }
    
    dir := filepath.Dir(path)
    ext := filepath.Ext(path)
    name := strings.TrimSuffix(filepath.Base(path), ext)
    
    for i := 1; ; i++ {
        newPath := filepath.Join(dir, fmt.Sprintf("%s (%d)%s", name, i, ext))
        if _, err := os.Stat(newPath); os.IsNotExist(err) {
            return newPath
        }
    }
}

func (s *Server) handleDownloadToDisk(c *gin.Context) {
    id := c.Param("id")
    var file db.File
    if err := s.DB.First(&file, id).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
        return
    }
    
    var settings db.Settings
    s.DB.First(&settings)
    
    // Fallback if not set
    saveDir := settings.DownloadPath
    if saveDir == "" {
        home, _ := os.UserHomeDir()
        saveDir = filepath.Join(home, "Downloads")
    }
    
    // Ensure dir exists
    if err := os.MkdirAll(saveDir, 0755); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create download directory"})
        return
    }

    dstPath := ensureUniquePath(filepath.Join(saveDir, file.Name))
    
    reader, _, _, err := s.GetFileStream(c.Request.Context(), file.CID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    dst, err := os.Create(dstPath)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create file: " + err.Error()})
        return
    }
    defer dst.Close()
    
    if _, err := io.Copy(dst, reader); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write file: " + err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"status": "saved", "path": dstPath})
}

func (s *Server) handleDownloadShared(c *gin.Context) {
    var req struct {
        CID  string `json:"cid"`
        Name string `json:"name"`
    }
    if err := c.BindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
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

    filename := req.Name
    if filename == "" {
        filename = req.CID + ".bin"
    }
    
    dstPath := ensureUniquePath(filepath.Join(saveDir, filename))
    
    reader, _, _, err := s.GetFileStream(c.Request.Context(), req.CID)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    
    dst, err := os.Create(dstPath)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create file: " + err.Error()})
        return
    }
    defer dst.Close()
    
    if _, err := io.Copy(dst, reader); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write file: " + err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"status": "saved", "path": dstPath})
}

func (s *Server) handleUpload(c *gin.Context, database *gorm.DB) {
	// 1. Parse Multipart Form
	fileHeader, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file uploaded"})
		return
	}

	srcFile, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer srcFile.Close()

	var reader io.Reader = srcFile

	// 2. Add to IPFS
	cid, err := s.Node.AddFile(c.Request.Context(), reader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("IPFS Add failed: %v", err)})
		return
	}

	// 3. Save Metadata to DB
	newFile := db.File{
		CID:            cid,
		Name:           fileHeader.Filename,
		Size:           fileHeader.Size,
		MimeType:       fileHeader.Header.Get("Content-Type"),
		CreatedAt:      time.Now(),
	}

	if err := database.Create(&newFile).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save metadata"})
		return
	}

	c.JSON(http.StatusOK, newFile)
}

func (s *Server) handleListFiles(c *gin.Context, database *gorm.DB) {
	var files []db.File
	// Order by newest first
	if err := database.Order("created_at desc").Find(&files).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch files"})
		return
	}
	c.JSON(http.StatusOK, files)
}

func (s *Server) handleDeleteFile(c *gin.Context, database *gorm.DB) {
	id := c.Param("id")
	var file db.File
	if err := database.First(&file, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	// Unpin from IPFS
	if err := s.Node.Unpin(c.Request.Context(), file.CID); err != nil {
		// Just log error, don't stop DB deletion
		fmt.Printf("Warning: Failed to unpin CID %s: %v\n", file.CID, err)
	}

	if err := database.Delete(&file).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}
