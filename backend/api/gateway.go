package api

import (
	"fmt"
	"io"
	"net/http"
	"encoding/hex"
	"encoding/base64"
	"mime"
	"path/filepath"
	"strings"

	"mochibox-core/crypto"
	"mochibox-core/db"

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
    
    // Decryption Handling
    // Check if file is encrypted in DB (My Files) OR Shared History OR provided via URL Params (Stateless)
    var encryptionType, encryptionMeta, filename string
    
    // 1. Check My Files
    var file db.File
    if err := s.DB.Where("cid = ?", cid).First(&file).Error; err == nil {
        encryptionType = file.EncryptionType
        encryptionMeta = file.EncryptionMeta
        filename = file.Name
    } else {
        // 2. Check Shared History (Fallback if deleted from My Files but still in History)
        var sharedFile db.SharedFile
        if err := s.DB.Where("cid = ?", cid).First(&sharedFile).Error; err == nil {
            encryptionType = sharedFile.EncryptionType
            encryptionMeta = sharedFile.EncryptionMeta
            filename = sharedFile.Name
        } else {
            // 3. Check URL Params (Stateless Fallback)
            // If user provides 'meta' in query, we trust it.
            // But we also need 'type'. For now, if 'meta' is present, we assume 'password' if len < 100?
            // Or we assume 'password' if not specified? 
            // Ideally URL should have both. But frontend might just send 'meta'.
            // Let's infer:
            // Salt (Hex) is usually 32 chars (16 bytes).
            // Encrypted Session Key (Base64) is longer.
            // Or better, check if 'password' param is present -> assume 'password' type.
            
            metaParam := c.Query("meta")
            if metaParam != "" {
                encryptionMeta = metaParam
                // Infer type or get from param if we add it later
                if c.Query("password") != "" {
                    encryptionType = "password"
                } else {
                    // Could be private? But Private needs local private key anyway.
                    // If Private, we usually don't pass meta in URL?
                    // Actually, for Private, meta is the encrypted session key.
                    // If we support stateless Private sharing, we need to pass that blob too.
                    // Let's assume 'password' if 'password' query param exists.
                    // Else if 'meta' exists, maybe 'private'?
                    encryptionType = "private" 
                }
            }
        }
    }

    if encryptionType != "" {
        if encryptionType == "password" {
            password := c.Query("password")
            if password == "" {
                c.String(http.StatusUnauthorized, "Password required")
                return
            }
            
            salt, err := hex.DecodeString(encryptionMeta)
            if err != nil {
                c.String(http.StatusInternalServerError, "Invalid salt in DB")
                return
            }
            
            key := crypto.DeriveKey(password, salt)
            decReader, err := crypto.NewAESCTRDecrypter(reader, key)
            if err != nil {
                c.String(http.StatusInternalServerError, "Decryption init failed")
                return
            }
            reader = decReader
            
        } else if encryptionType == "private" {
            // Need user private key
            if s.AccountManager.IsLocked() {
                c.String(http.StatusUnauthorized, "Account locked")
                return
            }
            
            // Decrypt Session Key
            encKey, err := base64.StdEncoding.DecodeString(encryptionMeta)
            if err != nil {
                c.String(http.StatusInternalServerError, "Invalid metadata")
                return
            }
            
            sessionKey, err := s.AccountManager.DecryptBox(encKey)
            if err != nil {
                c.String(http.StatusForbidden, "Access denied: " + err.Error())
                return
            }
            
            decReader, err := crypto.NewAESCTRDecrypter(reader, sessionKey)
            if err != nil {
                c.String(http.StatusInternalServerError, "Decryption init failed")
                return
            }
            reader = decReader
        }
    }

	// 3. Serve Content
	// buffer header to verify mime type if not in DB or generic
	buffer := make([]byte, 512)
	var n int

	if contentType == "" || contentType == "application/octet-stream" {
		shouldSniff := c.Query("download") != "true"
		if shouldSniff {
			n, _ = reader.Read(buffer)
		}
		if n > 0 {
			contentType = http.DetectContentType(buffer[:n])
		} else {
			contentType = "application/octet-stream"
		}
	}
	c.Header("Content-Type", contentType)

	if filename == "" {
		filename = cid
	}
	filename = filepath.Base(filename)
	if !strings.Contains(filename, ".") {
		baseType, _, err := mime.ParseMediaType(contentType)
		if err == nil && baseType != "" {
			if exts, err := mime.ExtensionsByType(baseType); err == nil && len(exts) > 0 && exts[0] != "" {
				filename += exts[0]
			}
		}
	}
	
	// Set Content-Length ONLY if known (> 0)
	if size > 0 {
		c.Header("Content-Length", fmt.Sprintf("%d", size))
	}
	
	// Disposition
	if c.Query("download") == "true" {
		c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	} else {
		c.Header("Content-Disposition", "inline")
	}
	
	// Write the buffer we peeked (only if we peeked)
	if n > 0 {
		if _, err := c.Writer.Write(buffer[:n]); err != nil {
			return
		}
	}
	
	// Copy the rest
	io.Copy(c.Writer, reader)
}
