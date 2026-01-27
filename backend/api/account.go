package api

import (
	"encoding/base64"
	"encoding/hex"
	"net/http"
	
	"mochibox-core/crypto"
	"mochibox-core/db"

	"github.com/gin-gonic/gin"
)

func (s *Server) registerAccountRoutes(api *gin.RouterGroup) {
	acc := api.Group("/account")
	{
		acc.GET("/status", s.handleAccountStatus)
		acc.POST("/init", s.handleAccountInit)
		acc.POST("/unlock", s.handleAccountUnlock)
		acc.POST("/lock", s.handleAccountLock)
		acc.DELETE("/", s.handleAccountReset)
		acc.POST("/generate-mnemonic", s.handleGenerateMnemonic)
		acc.POST("/export", s.handleAccountExport)
		acc.POST("/change-password", s.handleAccountChangePassword)
		acc.POST("/sign", s.handleAccountSign)
		acc.POST("/verify-signature", s.handleAccountVerifySignature)
	}
}

func (s *Server) handleAccountSign(c *gin.Context) {
	var req struct {
		Message string `json:"message" binding:"required"` // Base64
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	data, err := base64.StdEncoding.DecodeString(req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid base64 message"})
		return
	}

	sig, err := s.AccountManager.Sign(data)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	profile, err := s.AccountManager.GetProfile()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get profile"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"signature":  hex.EncodeToString(sig),
		"public_key": profile.PublicKey, // already hex
	})
}

func (s *Server) handleAccountVerifySignature(c *gin.Context) {
	var req struct {
		Message   string `json:"message" binding:"required"`    // Base64
		Signature string `json:"signature" binding:"required"`  // Hex
		PublicKey string `json:"public_key" binding:"required"` // Hex
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	data, err := base64.StdEncoding.DecodeString(req.Message)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid base64 message"})
		return
	}

	sig, err := hex.DecodeString(req.Signature)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid signature hex"})
		return
	}

	pubKey, err := hex.DecodeString(req.PublicKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid public key hex"})
		return
	}

	valid := s.AccountManager.Verify(data, sig, pubKey)
	c.JSON(http.StatusOK, gin.H{"valid": valid})
}

func (s *Server) handleAccountExport(c *gin.Context) {
    var req struct {
        Password string `json:"password" binding:"required"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    // We need to re-verify password against DB or current session?
    // Actually AccountManager.ExportMnemonic(password) logic would be safer
    mnemonic, err := s.AccountManager.ExportMnemonic(req.Password)
    if err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"mnemonic": mnemonic})
}

func (s *Server) handleAccountChangePassword(c *gin.Context) {
    var req struct {
        OldPassword string `json:"old_password" binding:"required"`
        NewPassword string `json:"new_password" binding:"required"`
    }
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    
    if err := s.AccountManager.ChangePassword(req.OldPassword, req.NewPassword); err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) handleAccountReset(c *gin.Context) {
    if err := s.AccountManager.Reset(); err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reset account: " + err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"status": "reset"})
}

func (s *Server) handleAccountStatus(c *gin.Context) {
	configured := s.AccountManager.IsConfigured()
	locked := s.AccountManager.IsLocked()
	
	var profile *db.Account
	if configured && !locked {
		var err error
		profile, err = s.AccountManager.GetProfile()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load profile"})
			return
		}
	} else if configured {
        // Return public info even if locked?
        // Yes, name and avatar are public
        profile, _ = s.AccountManager.GetProfile()
    }

	c.JSON(http.StatusOK, gin.H{
		"configured": configured,
		"locked":     locked,
		"profile":    profile,
	})
}

func (s *Server) handleGenerateMnemonic(c *gin.Context) {
    wallet, err := crypto.NewWallet()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }
    c.JSON(http.StatusOK, gin.H{"mnemonic": wallet.Mnemonic})
}

func (s *Server) handleAccountInit(c *gin.Context) {
	var req struct {
		Mnemonic string `json:"mnemonic" binding:"required"`
		Password string `json:"password" binding:"required"`
		Name     string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.AccountManager.InitAccount(req.Mnemonic, req.Password, req.Name); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to init account: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) handleAccountUnlock(c *gin.Context) {
	var req struct {
		Password   string `json:"password" binding:"required"`
		RememberMe bool   `json:"remember_me"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := s.AccountManager.Unlock(req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid password"})
		return
	}
    
    // Handle Remember Me
    if req.RememberMe {
        // Obfuscate and save password (or better, save a derived token, but for now we save password for auto-unlock)
        // Wait, storing password allows anyone with access to auth.lock to unlock.
        // Yes, that's the tradeoff discussed.
        if err := crypto.SaveAuthLock([]byte(req.Password), s.AccountManager.DataDir); err != nil {
            // Log warning but don't fail login
        }
    } else {
        // Clear if exists
        crypto.ClearAuthLock(s.AccountManager.DataDir)
    }

	c.JSON(http.StatusOK, gin.H{"status": "unlocked"})
}

func (s *Server) handleAccountLock(c *gin.Context) {
	s.AccountManager.Lock()
    crypto.ClearAuthLock(s.AccountManager.DataDir)
	c.JSON(http.StatusOK, gin.H{"status": "locked"})
}
