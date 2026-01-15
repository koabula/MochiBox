package api

import (
	"context"
	"net/http"
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
		system.POST("/shutdown", s.handleShutdown)
	}
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

	// Get Self ID
	key, err := s.Node.IPFS.Key().Self(ctx)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"online":    false,
			"peer_id":   "",
			"peers":     0,
			"addresses": []string{},
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
	})
}
