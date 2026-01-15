package api

import (
	"context"
	"net/http"
	"os"

	"mochibox-core/db"

	"github.com/gin-gonic/gin"
)

func (s *Server) registerConfigRoutes(api *gin.RouterGroup) {
	api.GET("/config", s.handleGetConfig)
	api.POST("/config", s.handleUpdateConfig)
}

func (s *Server) handleGetConfig(c *gin.Context) {
	var settings db.Settings
	if err := s.DB.First(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch settings"})
		return
	}
    
    // If empty, suggest UserHome/Downloads
    if settings.DownloadPath == "" {
        home, _ := os.UserHomeDir()
        settings.DownloadPath = home + string(os.PathSeparator) + "Downloads"
    }

	// Default IPFS API URL
	if settings.IpfsApiUrl == "" {
		settings.IpfsApiUrl = "http://127.0.0.1:5001"
	}

	c.JSON(http.StatusOK, settings)
}

func (s *Server) handleUpdateConfig(c *gin.Context) {
	var req db.Settings
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	var settings db.Settings
	s.DB.First(&settings)
	settings.DownloadPath = req.DownloadPath
	settings.AskPath = req.AskPath
	settings.IpfsApiUrl = req.IpfsApiUrl
	settings.UseEmbeddedNode = req.UseEmbeddedNode
	
	// If the user clears it, set to default
	if settings.IpfsApiUrl == "" {
		settings.IpfsApiUrl = "http://127.0.0.1:5001"
	}
	
	s.DB.Save(&settings)

	// Handle Embedded Node Lifecycle
	if s.IpfsManager != nil {
		if settings.UseEmbeddedNode {
			// Start if not running
			if err := s.IpfsManager.InitRepo(); err == nil {
				go s.IpfsManager.Start(context.Background())
			}
		} else {
			// Stop
			s.IpfsManager.Stop()
		}
	}
	
	// Update Node connection
	if s.Node != nil {
		s.Node.UpdateApiUrl(settings.IpfsApiUrl)
	}

	c.JSON(http.StatusOK, settings)
}
