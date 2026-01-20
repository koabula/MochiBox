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
	"time"

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
    var encryptionType, encryptionMeta, filename, savedPassword string
    
    // 1. Check My Files
    var file db.File
    if err := s.DB.Where("cid = ?", cid).First(&file).Error; err == nil {
        encryptionType = file.EncryptionType
        encryptionMeta = file.EncryptionMeta
        filename = file.Name
        savedPassword = file.SavedPassword
    } else {
        // 2. Check Shared History (Fallback if deleted from My Files but still in History)
        var sharedFile db.SharedFile
        if err := s.DB.Where("cid = ?", cid).First(&sharedFile).Error; err == nil {
            encryptionType = sharedFile.EncryptionType
            encryptionMeta = sharedFile.EncryptionMeta
            filename = sharedFile.Name
        } else {
            // 3. Check URL Params (Stateless Fallback)
            metaParam := c.Query("meta")
            typeParam := c.Query("type")
            if metaParam != "" {
                encryptionMeta = metaParam
                if typeParam != "" {
                    encryptionType = typeParam
                } else {
                    // Legacy Fallback
                    if c.Query("password") != "" {
                        encryptionType = "password"
                    } else {
                        encryptionType = "private" 
                    }
                }
            }
        }
    }

    if encryptionType != "" {
        if encryptionType == "password" {
            password := c.Query("password")
            
            // Try saved password if needed
            if password == "" && savedPassword != "" {
                 if s.AccountManager.Wallet != nil {
                     // Decrypt saved password
                     encPass, err := base64.StdEncoding.DecodeString(savedPassword)
                     if err == nil {
                         // Convert Ed25519 keys to Curve25519 for Box
                         curvePub, _ := crypto.Ed25519PublicKeyToCurve25519(s.AccountManager.Wallet.PublicKey)
                         curvePriv, _ := crypto.Ed25519PrivateKeyToCurve25519(s.AccountManager.Wallet.PrivateKey)
                         var pubKey [32]byte
                         var privKey [32]byte
                         copy(pubKey[:], curvePub)
                         copy(privKey[:], curvePriv)
                         
                         decrypted, err := crypto.DecryptBoxAnonymous(encPass, &pubKey, &privKey)
                         if err == nil {
                             password = string(decrypted)
                         }
                     }
                 }
            }
            
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
            
            // Try Seekable Decryption if source supports Seek
            if rs, ok := reader.(io.ReadSeeker); ok {
                decReader, err := crypto.NewSeekableAESCTRDecrypter(rs, key)
                if err != nil {
                    c.String(http.StatusInternalServerError, "Decryption init failed")
                    return
                }
                reader = decReader
                // Adjust size for IV
                if size > 16 {
                    size -= 16
                }
            } else {
                decReader, err := crypto.NewAESCTRDecrypter(reader, key)
                if err != nil {
                    c.String(http.StatusInternalServerError, "Decryption init failed")
                    return
                }
                reader = decReader
                // Adjust size for IV
                if size > 16 {
                    size -= 16
                }
            }
            
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
            
            // Try Seekable Decryption
            if rs, ok := reader.(io.ReadSeeker); ok {
                decReader, err := crypto.NewSeekableAESCTRDecrypter(rs, sessionKey)
                if err != nil {
                    c.String(http.StatusInternalServerError, "Decryption init failed")
                    return
                }
                reader = decReader
                if size > 16 {
                    size -= 16
                }
            } else {
                decReader, err := crypto.NewAESCTRDecrypter(reader, sessionKey)
                if err != nil {
                    c.String(http.StatusInternalServerError, "Decryption init failed")
                    return
                }
                reader = decReader
                if size > 16 {
                    size -= 16
                }
            }
        }
    }

	// 3. Serve Content
	// buffer header to verify mime type if not in DB or generic
	buffer := make([]byte, 512)
	var n int

	// Allow overriding filename via query param (e.g. for folder preview downloads)
	if nameParam := c.Query("filename"); nameParam != "" {
		filename = nameParam
	}
    
    // Sniff content type if needed
	if contentType == "" || contentType == "application/octet-stream" {
		shouldSniff := c.Query("download") != "true"
        // Only sniff if we can't Seek back or if we don't care (not seekable)
        // If it is seekable, we can read and seek back.
        if rs, ok := reader.(io.ReadSeeker); ok && shouldSniff {
             n, _ = rs.Read(buffer)
             if n > 0 {
                 contentType = http.DetectContentType(buffer[:n])
                 rs.Seek(0, io.SeekStart) // Reset
                 n = 0 // Don't write buffer manually
             }
        } else if shouldSniff {
            // Non-seekable, we have to peek
			n, _ = reader.Read(buffer)
            if n > 0 {
                contentType = http.DetectContentType(buffer[:n])
            }
		}
        
		if contentType == "" {
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
    
    // Force .zip for zip streams if not present
    if contentType == "application/zip" && !strings.HasSuffix(strings.ToLower(filename), ".zip") {
        filename += ".zip"
    }
	
	// Set Content-Length ONLY if known (> 0)
	if size > 0 {
		c.Header("Content-Length", fmt.Sprintf("%d", size))
	}
	
	// Disposition
	if c.Query("download") == "true" {
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	} else {
		c.Header("Content-Disposition", "inline")
	}
	
    // Serve
    if rs, ok := reader.(io.ReadSeeker); ok {
        // http.ServeContent handles Range, Content-Length, etc.
        // Note: we already set some headers, ServeContent respects them or overwrites if needed.
        // It handles Range automatically.
        http.ServeContent(c.Writer, c.Request, filename, time.Time{}, rs)
    } else {
        // Write the buffer we peeked (only if we peeked and couldn't seek back)
        if n > 0 {
            if _, err := c.Writer.Write(buffer[:n]); err != nil {
                return
            }
        }
        io.Copy(c.Writer, reader)
    }
}
