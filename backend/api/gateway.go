package api

import (
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
)

func (s *Server) registerGatewayRoutes(g *gin.RouterGroup) {
	g.GET("/preview/:cid", s.handlePreview)
}

func (s *Server) handlePreview(c *gin.Context) {
	cid := c.Param("cid")

    reader, contentType, size, err := s.GetFileStream(c.Request.Context(), cid)
    
    if err != nil {
        c.String(http.StatusNotFound, err.Error())
        return
    }

    // 3. Serve Content
	// buffer header to verify mime type if not in DB or generic
    // We also use this buffer to start the response
	buffer := make([]byte, 512)
	n, _ := reader.Read(buffer) // Note: This consumes stream!

    if contentType == "" || contentType == "application/octet-stream" {
        contentType = http.DetectContentType(buffer[:n])
    }
    c.Header("Content-Type", contentType)
    
    // Set Content-Length if known
    if size > 0 {
        c.Header("Content-Length", fmt.Sprintf("%d", size))
    }
    
    // Disposition
    if c.Query("download") == "true" {
        c.Header("Content-Disposition", "attachment") // Browser will pick filename from URL or default
    } else {
        c.Header("Content-Disposition", "inline")
    }
    
    // Write the buffer we peeked
    if n > 0 {
        if _, err := c.Writer.Write(buffer[:n]); err != nil {
            return
        }
    }
    
    // Copy the rest
    io.Copy(c.Writer, reader)
}
