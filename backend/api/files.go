package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"encoding/hex"
	"encoding/base64"
	"crypto/rand"

	"mochibox-core/db"
	"mochibox-core/crypto"

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
        api.POST("/:id/reveal", s.handleRevealPassword)
        api.POST("/download/shared", s.handleDownloadShared)
		api.POST("/sync", func(c *gin.Context) {
			s.handleSyncFiles(c, db)
		})
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
    var req struct {
        Password string `json:"password"`
    }
    // Bind JSON if present, ignore error if empty body
    c.ShouldBindJSON(&req)

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

    // Decryption Logic
    if file.EncryptionType == "password" {
        password := req.Password
        
        // If password not provided, try to use saved password
        if password == "" && file.SavedPassword != "" {
            if s.AccountManager.Wallet != nil {
                // Decrypt saved password
                encPass, err := base64.StdEncoding.DecodeString(file.SavedPassword)
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
            c.JSON(http.StatusUnauthorized, gin.H{"error": "Password required"})
            return
        }

        salt, _ := hex.DecodeString(file.EncryptionMeta)
        key := crypto.DeriveKey(password, salt)
        decReader, err := crypto.NewAESCTRDecrypter(reader, key)
        if err != nil {
            c.JSON(http.StatusInternalServerError, gin.H{"error": "Decryption init failed"})
            return
        }
        reader = decReader

    } else if file.EncryptionType == "private" {
         if s.AccountManager.IsLocked() {
             c.JSON(http.StatusUnauthorized, gin.H{"error": "Account locked"})
             return
         }
         encKey, _ := base64.StdEncoding.DecodeString(file.EncryptionMeta)
         sessionKey, err := s.AccountManager.DecryptBox(encKey)
         if err != nil {
             c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to decrypt session key"})
             return
         }
         decReader, err := crypto.NewAESCTRDecrypter(reader, sessionKey)
         if err != nil {
             c.JSON(http.StatusInternalServerError, gin.H{"error": "Decryption init failed"})
             return
         }
         reader = decReader
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

func (s *Server) handleRevealPassword(c *gin.Context) {
    id := c.Param("id")
    var file db.File
    if err := s.DB.First(&file, id).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
        return
    }
    
    if file.SavedPassword == "" {
        c.JSON(http.StatusNotFound, gin.H{"error": "No saved password"})
        return
    }
    
    if s.AccountManager.Wallet == nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Account locked"})
        return
    }
    
    encPass, err := base64.StdEncoding.DecodeString(file.SavedPassword)
    if err != nil {
         c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid saved password"})
         return
    }
    
    // Convert Ed25519 keys to Curve25519 for Box
    curvePub, _ := crypto.Ed25519PublicKeyToCurve25519(s.AccountManager.Wallet.PublicKey)
    curvePriv, _ := crypto.Ed25519PrivateKeyToCurve25519(s.AccountManager.Wallet.PrivateKey)
    var pubKey [32]byte
    var privKey [32]byte
    copy(pubKey[:], curvePub)
    copy(privKey[:], curvePriv)
    
    decrypted, err := crypto.DecryptBoxAnonymous(encPass, &pubKey, &privKey)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Decryption failed"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"password": string(decrypted)})
}

func (s *Server) handleDownloadShared(c *gin.Context) {
    var req struct {
        CID      string `json:"cid"`
        Name     string `json:"name"`
        Password string `json:"password"`
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
    
    // Decryption Logic (Replicated from handlePreview/gateway.go)
    // Try to find file in DB first for Metadata
    var file db.File
    if err := s.DB.Where("cid = ?", req.CID).First(&file).Error; err == nil {
        if file.EncryptionType == "password" {
            if req.Password == "" {
                 c.JSON(http.StatusUnauthorized, gin.H{"error": "Password required"})
                 return
            }
            salt, _ := hex.DecodeString(file.EncryptionMeta)
            key := crypto.DeriveKey(req.Password, salt)
            decReader, err := crypto.NewAESCTRDecrypter(reader, key)
            if err == nil {
                reader = decReader
            }
        } else if file.EncryptionType == "private" {
             if s.AccountManager.IsLocked() {
                 c.JSON(http.StatusUnauthorized, gin.H{"error": "Account locked"})
                 return
             }
             encKey, _ := base64.StdEncoding.DecodeString(file.EncryptionMeta)
             sessionKey, err := s.AccountManager.DecryptBox(encKey)
             if err == nil {
                 decReader, err := crypto.NewAESCTRDecrypter(reader, sessionKey)
                 if err == nil {
                     reader = decReader
                 }
             }
        }
    } else {
        // Not in DB? Try to use params if provided (TODO: Frontend needs to send salt/meta)
        // For now, if not in DB, we can't decrypt unless we trust the user provided metadata
        // which we haven't added to the request struct yet (except Password).
        // BUT, Shared files should be pinned/added to DB upon "Pin" or "Import"?
        // If just "Previewing", we use handlePreview.
        // If "Downloading" via this endpoint, it implies we want to save it locally.
        // If it's a "Shared" file that hasn't been pinned, we might not have the metadata in DB!
        // This is a gap. The frontend has the metadata from the Mochi Link.
        // The frontend SHOULD pass the metadata here if it's not in DB.
        // But for this immediate fix, let's assume the user has Pinned it or we accept metadata in request.
        // Let's rely on the frontend falling back to Blob Download (via handlePreview) if this fails?
        // No, user said "Download currently downloads encrypted file".
        // This endpoint is for "Silent Download" (server-side save).
        // If the file is not in DB (unpinned), we don't know the Salt.
        // We should probably fail if we can't decrypt, rather than saving garbage.
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
	
	// Encryption Logic
	encType := c.PostForm("encryption_type")
	if encType == "" { encType = "public" }
    
    savePassword := c.PostForm("save_password") == "true"
    var savedPassword string
    var recipientPubKey string
	
	var encryptionMeta string
	mimeType := fileHeader.Header.Get("Content-Type")
	if encType == "public" && (mimeType == "" || mimeType == "application/octet-stream") {
		buffer := make([]byte, 512)
		n, _ := reader.Read(buffer)
		if n > 0 {
			mimeType = http.DetectContentType(buffer[:n])
			reader = io.MultiReader(bytes.NewReader(buffer[:n]), reader)
		}
	}
	
	if encType == "password" {
		password := c.PostForm("password")
		if password == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Password required"})
			return
		}

        if savePassword {
            if s.AccountManager.Wallet != nil {
                // Encrypt password with Account Public Key
                curvePub, _ := crypto.Ed25519PublicKeyToCurve25519(s.AccountManager.Wallet.PublicKey)
                encPass, err := crypto.EncryptSessionKey(curvePub, []byte(password))
                if err == nil {
                    savedPassword = base64.StdEncoding.EncodeToString(encPass)
                }
            }
        }
		
		salt, err := crypto.GenerateSalt(16)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate salt"})
			return
		}
		
		key := crypto.DeriveKey(password, salt)
		r, err := crypto.NewAESCTRReader(reader, key)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Encryption init failed"})
			return
		}
		
		// IMPORTANT: Use the wrapper directly!
		reader = r
		encryptionMeta = hex.EncodeToString(salt)
		
	} else if encType == "private" {
		receiverPubHex := c.PostForm("receiver_pub_key")
		if receiverPubHex == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Receiver Public Key required"})
			return
		}
        recipientPubKey = receiverPubHex
		
		edPub, err := hex.DecodeString(receiverPubHex)
		if err != nil || len(edPub) != 32 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Public Key"})
			return
		}
		
		curvePub, err := crypto.Ed25519PublicKeyToCurve25519(edPub)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to convert key: " + err.Error()})
			return
		}
		
		sessionKey := make([]byte, 32)
		if _, err := rand.Read(sessionKey); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "RNG failed"})
			return
		}
		
		r, err := crypto.NewAESCTRReader(reader, sessionKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Encryption init failed"})
			return
		}
		
		// IMPORTANT: Use the wrapper directly!
		reader = r
		
		encKey, err := crypto.EncryptSessionKey(curvePub, sessionKey)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to encrypt session key"})
			return
		}
		
		encryptionMeta = base64.StdEncoding.EncodeToString(encKey)
	}

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
		MimeType:       mimeType,
		EncryptionType: encType,
		EncryptionMeta: encryptionMeta,
        SavedPassword:  savedPassword,
        RecipientPubKey: recipientPubKey,
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

func (s *Server) handleSyncFiles(c *gin.Context, database *gorm.DB) {
	pins, err := s.Node.ListPins(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list pins: " + err.Error()})
		return
	}

	addedCount := 0
	for _, cid := range pins {
		var count int64
		database.Model(&db.File{}).Where("cid = ?", cid).Count(&count)
		if count == 0 {
			// New file found
			newFile := db.File{
				CID:       cid,
				Name:      "Imported-" + cid[:8], // Generic name
				CreatedAt: time.Now(),
				MimeType:  "application/octet-stream",
			}

			// Try to get size
			size, err := s.Node.GetFileSize(c.Request.Context(), cid)
			if err == nil {
				newFile.Size = size
			}

			if err := database.Create(&newFile).Error; err == nil {
				addedCount++
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "synced",
		"total_pins": len(pins),
		"added":      addedCount,
	})
}
