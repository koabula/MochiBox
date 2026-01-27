package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
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

	addrs := make([]string, 0)
	shareAddrs := make([]string, 0)
	shareSeen := make(map[string]bool)
	peerID := key.ID().String()
	tcpPort := "4001"

	listenAddrs, err := s.Node.IPFS.Swarm().ListenAddrs(ctx)
	if err == nil {
		for _, addr := range listenAddrs {
			sAddr := addr.String()
			addrs = append(addrs, sAddr)

			if p, err := addr.ValueForProtocol(multiaddr.P_TCP); err == nil && strings.TrimSpace(p) != "" {
				tcpPort = p
			}

			if strings.Contains(sAddr, "/ip4/0.0.0.0/") || strings.Contains(sAddr, "/ip6/::/") {
				continue
			}
			if strings.Contains(sAddr, "/127.0.0.1/") || strings.Contains(sAddr, "/::1/") {
				continue
			}

			dial := sAddr
			if !strings.Contains(dial, "/p2p/") {
				dial = dial + "/p2p/" + peerID
			}
			if !shareSeen[dial] {
				shareSeen[dial] = true
				shareAddrs = append(shareAddrs, dial)
			}
		}
	}

	if ifaces, err := net.Interfaces(); err == nil {
		for _, iface := range ifaces {
			if (iface.Flags&net.FlagUp) == 0 || (iface.Flags&net.FlagLoopback) != 0 {
				continue
			}
			iAddrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, a := range iAddrs {
				var ip net.IP
				switch v := a.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				default:
					continue
				}
				if ip == nil || ip.IsLoopback() || ip.IsMulticast() || ip.IsUnspecified() {
					continue
				}
				if ip.IsLinkLocalUnicast() {
					continue
				}

				var base string
				if ip4 := ip.To4(); ip4 != nil {
					base = fmt.Sprintf("/ip4/%s/tcp/%s", ip4.String(), tcpPort)
				} else {
					base = fmt.Sprintf("/ip6/%s/tcp/%s", ip.String(), tcpPort)
				}
				dial := base + "/p2p/" + peerID
				if !shareSeen[dial] {
					shareSeen[dial] = true
					shareAddrs = append(shareAddrs, dial)
				}
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"online":          true,
		"peer_id":         peerID,
		"peers":           peerCount,
		"addresses":       addrs,
		"share_addresses": shareAddrs,
		"data_dir":        currentDataDir,
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
	var mu sync.Mutex
	attempted := 0
	connected := 0

	// 1. Connect to Bootstrap Peers
	for _, addrStr := range bootstrapPeers {
		addrInfo, err := peer.AddrInfoFromString(addrStr)
		if err == nil {
			wg.Add(1)
			go func(info *peer.AddrInfo) {
				defer wg.Done()
				mu.Lock()
				attempted++
				mu.Unlock()

				pCtx, pCancel := context.WithTimeout(ctx, 5*time.Second)
				defer pCancel()

				if err := s.Node.IPFS.Swarm().Connect(pCtx, *info); err == nil {
					mu.Lock()
					connected++
					mu.Unlock()
				}
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

	c.JSON(http.StatusOK, gin.H{
		"message":   "Network boost finished",
		"attempted": attempted,
		"connected": connected,
	})
}
