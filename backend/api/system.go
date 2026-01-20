package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/libp2p/go-libp2p/core/peer"
)

func (s *Server) registerSystemRoutes(rg *gin.RouterGroup) {
	system := rg.Group("/system")
	{
		system.GET("/status", s.handleNodeStatus)
		system.GET("/peers", s.handleListPeers)
		system.POST("/connect", s.handleConnectPeer)
		system.POST("/bootstrap", s.handleBootstrap)
		system.POST("/shutdown", s.handleShutdown)
		system.POST("/datadir", s.handleSetDataDir)
	}
}

func (s *Server) handleSetDataDir(c *gin.Context) {
	var req struct {
		NewPath string `json:"new_path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	newPath := filepath.Clean(req.NewPath)
	if err := os.MkdirAll(newPath, 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create directory: " + err.Error()})
		return
	}

	home, _ := os.UserHomeDir()
	defaultBase := filepath.Join(home, ".mochibox")
	pointerPath := filepath.Join(defaultBase, ".pointer")
	
	// Ensure default base exists for the pointer file
	os.MkdirAll(defaultBase, 0755)

	// Determine current data dir
	currentDataDir := defaultBase
	if content, err := os.ReadFile(pointerPath); err == nil {
		p := strings.TrimSpace(string(content))
		if p != "" {
			currentDataDir = p
		}
	}
	
	// Stop services to release file locks
	if s.IpfsManager != nil {
		s.IpfsManager.Stop()
	}
	
	sqlDB, err := s.DB.DB()
	if err == nil {
		sqlDB.Close()
	}

	// 1. Copy mochibox.db
	srcDB := filepath.Join(currentDataDir, "mochibox.db")
	dstDB := filepath.Join(newPath, "mochibox.db")
	// Only copy if source exists
	if _, err := os.Stat(srcDB); err == nil {
		if err := CopyFile(srcDB, dstDB); err != nil {
			fmt.Printf("Warning: Failed to copy DB: %v\n", err)
		}
	}
	
	// 2. Copy IPFS Repo
	srcRepo := filepath.Join(currentDataDir, "ipfs-repo")
	dstRepo := filepath.Join(newPath, "ipfs-repo")
	if _, err := os.Stat(srcRepo); err == nil {
		if err := CopyDir(srcRepo, dstRepo); err != nil {
			fmt.Printf("Warning: Failed to copy IPFS Repo: %v\n", err)
		}
	}
	
	// 3. Write Pointer
	if err := os.WriteFile(pointerPath, []byte(newPath), 0644); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update settings: " + err.Error()})
		return
	}
	
	c.JSON(http.StatusOK, gin.H{"status": "updated", "message": "Data directory updated. Please restart the application."})
	
	// Trigger shutdown
	go func() {
		time.Sleep(1 * time.Second)
		if s.ShutdownChan != nil {
			s.ShutdownChan <- true
		}
	}()
}

func (s *Server) handleShutdown(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "shutting_down"})
	
	go func() {
		// Wait a bit for the response to be sent
		time.Sleep(100 * time.Millisecond)
		if s.ShutdownChan != nil {
			s.ShutdownChan <- true
		}
	}()
}

func (s *Server) handleConnectPeer(c *gin.Context) {
	var req struct {
		Multiaddr string `json:"multiaddr" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse the multiaddr
	addrInfo, err := peer.AddrInfoFromString(req.Multiaddr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid multiaddr format"})
		return
	}

	// Connect with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.Node.IPFS.Swarm().Connect(ctx, *addrInfo); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Connected successfully"})
}

func (s *Server) handleListPeers(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	peers, err := s.Node.IPFS.Swarm().Peers(ctx)
	if err != nil {
		// Return empty list instead of 500
		c.JSON(http.StatusOK, []gin.H{})
		return
	}

	peerList := make([]gin.H, 0)
	for _, p := range peers {
		peerList = append(peerList, gin.H{
			"id":      p.ID().String(),
			"address": p.Address().String(),
		})
	}

	c.JSON(http.StatusOK, peerList)
}

func (s *Server) handleNodeStatus(c *gin.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Determine current data dir (re-read pointer or use cached if available)
	home, _ := os.UserHomeDir()
	defaultBase := filepath.Join(home, ".mochibox")
	pointerPath := filepath.Join(defaultBase, ".pointer")
	currentDataDir := defaultBase
	if content, err := os.ReadFile(pointerPath); err == nil {
		p := strings.TrimSpace(string(content))
		if p != "" {
			currentDataDir = p
		}
	}

	// Get Self ID
	key, err := s.Node.IPFS.Key().Self(ctx)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"online":    false,
			"peer_id":   "",
			"peers":     0,
			"addresses": []string{},
			"data_dir":  currentDataDir,
		})
		return
	}

	// Get Peers Count
	peers, err := s.Node.IPFS.Swarm().Peers(ctx)
	peerCount := 0
	if err == nil {
		peerCount = len(peers)
	}
	
	// Get Addresses
	addrs := make([]string, 0)
	
	listenAddrs, err := s.Node.IPFS.Swarm().ListenAddrs(ctx)
	if err == nil {
		for _, addr := range listenAddrs {
			addrs = append(addrs, addr.String())
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"online":    true,
		"peer_id":   key.ID().String(),
		"peers":     peerCount,
		"addresses": addrs,
		"data_dir":  currentDataDir,
	})
}

func (s *Server) handleBootstrap(c *gin.Context) {
	s.BoostMutex.Lock()
	if s.IsBoosting {
		s.BoostMutex.Unlock()
		c.JSON(http.StatusConflict, gin.H{"status": "running", "message": "Network boost is already running"})
		return
	}
	s.IsBoosting = true
	s.BoostMutex.Unlock()

	defer func() {
		s.BoostMutex.Lock()
		s.IsBoosting = false
		s.BoostMutex.Unlock()
	}()

	// Standard IPFS Bootstrap Nodes + Cloudflare
	bootstrapPeers := []string{
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9CkJg6M6VMcMG_Qx",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		"/ip4/104.131.131.82/tcp/4001/p2p/QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ",
		// Cloudflare
		"/dnsaddr/bootstrap.cloudflare-ipfs.com/ipfs/QmcFf2FH3CEgTNHeMRGhN7HNHU1EXAxoEk6EFuSyXCsvRE",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	// 1. Connect to Bootstrap Peers
	for _, addrStr := range bootstrapPeers {
		addrInfo, err := peer.AddrInfoFromString(addrStr)
		if err == nil {
			wg.Add(1)
			go func(info *peer.AddrInfo) {
				defer wg.Done()
				// We ignore errors here as we just want to try connecting
				_ = s.Node.IPFS.Swarm().Connect(ctx, *info)
			}(addrInfo)
		}
	}

	// Wait for all connection attempts (or timeout)
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All connections attempted
	case <-ctx.Done():
		// Timeout reached
	}

	// 2. Announce Self to DHT (FindPeer Self)
	// This forces a lookup which connects to more peers
	selfKey, err := s.Node.IPFS.Key().Self(ctx)
	if err == nil {
		// We ignore the error here as we just want to trigger the side effect of finding peers
		_, _ = s.Node.IPFS.Routing().FindPeer(ctx, selfKey.ID())
	}

	c.JSON(http.StatusOK, gin.H{"message": "Network boost finished"})
}
