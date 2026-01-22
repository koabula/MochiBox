package core

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

type ConnectionManager struct {
	ipfsMgr         *IpfsManager
	mu              sync.Mutex
	activeDownloads map[string]time.Time
	originalConfig  map[string]string
	boosted         bool
}

func NewConnectionManager(ipfsMgr *IpfsManager) *ConnectionManager {
	return &ConnectionManager{
		ipfsMgr:         ipfsMgr,
		activeDownloads: make(map[string]time.Time),
		originalConfig:  make(map[string]string),
		boosted:         false,
	}
}

// BoostForDownload temporarily increases connection limits during download
func (cm *ConnectionManager) BoostForDownload(cid string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Mark download as active
	cm.activeDownloads[cid] = time.Now()

	// Only boost once for concurrent downloads
	if cm.boosted {
		log.Printf("Already boosted, tracking CID: %s", cid)
		return nil
	}

	log.Printf("Boosting connection limits for download: %s", cid)

	// Save original config
	if err := cm.saveOriginalConfig(); err != nil {
		log.Printf("Warning: Failed to save original config: %v", err)
	}

	// Apply boost configuration
	if err := cm.applyBoostConfig(); err != nil {
		return fmt.Errorf("failed to apply boost config: %w", err)
	}

	cm.boosted = true

	// Schedule automatic restore after 5 minutes
	go func() {
		time.Sleep(5 * time.Minute)
		cm.autoRestore()
	}()

	return nil
}

// RestoreDefaults restores connection limits after download completes
func (cm *ConnectionManager) RestoreDefaults(cid string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Remove from active downloads
	delete(cm.activeDownloads, cid)

	// Only restore if no active downloads remain
	if len(cm.activeDownloads) > 0 {
		log.Printf("Still have %d active downloads, keeping boost", len(cm.activeDownloads))
		return nil
	}

	return cm.restore()
}

// autoRestore restores config automatically after timeout
func (cm *ConnectionManager) autoRestore() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Check if downloads are still active
	now := time.Now()
	for cid, startTime := range cm.activeDownloads {
		if now.Sub(startTime) > 5*time.Minute {
			delete(cm.activeDownloads, cid)
		}
	}

	if len(cm.activeDownloads) > 0 {
		log.Printf("Auto-restore deferred: %d downloads still active", len(cm.activeDownloads))
		return
	}

	if err := cm.restore(); err != nil {
		log.Printf("Warning: Auto-restore failed: %v", err)
	}
}

// restore applies the original configuration
func (cm *ConnectionManager) restore() error {
	if !cm.boosted {
		return nil
	}

	log.Println("Restoring original connection limits")

	configs := []struct {
		key   string
		value string
	}{
		{"Swarm.ConnMgr.HighWater", "600"},
		{"Swarm.ConnMgr.LowWater", "100"},
		{"Swarm.ConnMgr.GracePeriod", "\"20s\""},
	}

	for _, cfg := range configs {
		// Restore from saved config if available
		if original, ok := cm.originalConfig[cfg.key]; ok {
			cfg.value = original
		}

		if err := cm.updateIPFSConfigJSON(cfg.key, cfg.value); err != nil {
			log.Printf("Warning: Failed to restore %s: %v", cfg.key, err)
		}
	}

	cm.boosted = false
	log.Println("Connection limits restored")
	return nil
}

// saveOriginalConfig saves current configuration before boosting
func (cm *ConnectionManager) saveOriginalConfig() error {
	if cm.ipfsMgr == nil || cm.ipfsMgr.BinPath == "" {
		return fmt.Errorf("ipfs manager not available")
	}

	configs := []string{
		"Swarm.ConnMgr.HighWater",
		"Swarm.ConnMgr.LowWater",
		"Swarm.ConnMgr.GracePeriod",
	}

	for _, key := range configs {
		cmd := exec.Command(cm.ipfsMgr.BinPath, "config", key)
		cmd.Env = append(cmd.Env, "IPFS_PATH="+cm.ipfsMgr.DataDir)

		output, err := cmd.Output()
		if err == nil {
			cm.originalConfig[key] = string(output)
		}
	}

	return nil
}

// applyBoostConfig applies higher connection limits
func (cm *ConnectionManager) applyBoostConfig() error {
	configs := []struct {
		key   string
		value string
	}{
		{"Swarm.ConnMgr.HighWater", "2000"},
		{"Swarm.ConnMgr.LowWater", "1500"},
		{"Swarm.ConnMgr.GracePeriod", "\"120s\""},
	}

	for _, cfg := range configs {
		if err := cm.updateIPFSConfigJSON(cfg.key, cfg.value); err != nil {
			return err
		}
	}

	return nil
}

// updateIPFSConfigJSON updates IPFS configuration via CLI with --json flag
func (cm *ConnectionManager) updateIPFSConfigJSON(key, value string) error {
	if cm.ipfsMgr == nil || cm.ipfsMgr.BinPath == "" {
		return fmt.Errorf("ipfs manager not available")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, cm.ipfsMgr.BinPath, "config", "--json", key, value)
	cmd.Env = append(cmd.Env, "IPFS_PATH="+cm.ipfsMgr.DataDir)

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to update %s: %v, output: %s", key, err, string(output))
	}

	log.Printf("Updated IPFS config: %s = %s", key, value)
	return nil
}

// ProtectPeers adds peers to the protected list (via peering)
func (cm *ConnectionManager) ProtectPeers(ctx context.Context, node *MochiNode, peers []peer.ID) error {
	if node == nil {
		return fmt.Errorf("node not initialized")
	}

	var wg sync.WaitGroup
	errors := make(chan error, len(peers))

	for _, peerID := range peers {
		wg.Add(1)
		go func(pid peer.ID) {
			defer wg.Done()

			// Get peer connections
			peerConns, err := node.IPFS.Swarm().Peers(ctx)
			if err != nil {
				errors <- fmt.Errorf("failed to get peers: %w", err)
				return
			}

			for _, conn := range peerConns {
				if conn.ID() == pid {
					// Get address from connection
					addr := conn.Address()
					fullAddr := fmt.Sprintf("%s/p2p/%s", addr.String(), pid.String())

					// Add to peering
					if err := node.PeeringAdd(ctx, fullAddr); err != nil {
						errors <- fmt.Errorf("failed to protect peer %s: %w", pid.String(), err)
						return
					}
					log.Printf("Protected peer: %s", pid.String())
					return
				}
			}

			errors <- fmt.Errorf("peer not found: %s", pid.String())
		}(peerID)
	}

	wg.Wait()
	close(errors)

	// Collect errors
	var lastErr error
	for err := range errors {
		if err != nil {
			lastErr = err
			log.Printf("Warning: %v", err)
		}
	}

	return lastErr
}

// GetActiveDownloads returns the number of active downloads
func (cm *ConnectionManager) GetActiveDownloads() int {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return len(cm.activeDownloads)
}

// IsBoosted returns whether connection limits are currently boosted
func (cm *ConnectionManager) IsBoosted() bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.boosted
}
