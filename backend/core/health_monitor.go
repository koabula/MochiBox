package core

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// HealthMonitor provides periodic maintenance and on-demand repair for IPFS connections.
// It helps prevent Bitswap session state issues that accumulate over long runtime.
type HealthMonitor struct {
	node    *MochiNode
	ipfsMgr *IpfsManager
	booster *DownloadBooster

	mu              sync.Mutex
	lastMaintenance time.Time
	failureCount    map[string]int // CID -> consecutive failure count
	running         bool
	stopCh          chan struct{}
}

const (
	maintenanceInterval   = 30 * time.Minute
	failureThreshold      = 2 // Trigger repair after this many consecutive failures
	connectionTestTimeout = 5 * time.Second
)

func NewHealthMonitor(node *MochiNode, ipfsMgr *IpfsManager, booster *DownloadBooster) *HealthMonitor {
	return &HealthMonitor{
		node:         node,
		ipfsMgr:      ipfsMgr,
		booster:      booster,
		failureCount: make(map[string]int),
	}
}

// Start begins the periodic maintenance routine
func (hm *HealthMonitor) Start() {
	hm.mu.Lock()
	if hm.running {
		hm.mu.Unlock()
		return
	}
	hm.running = true
	hm.stopCh = make(chan struct{})
	hm.mu.Unlock()

	go hm.maintenanceLoop()
	log.Println("Health monitor started")
}

// Stop halts the periodic maintenance
func (hm *HealthMonitor) Stop() {
	hm.mu.Lock()
	if !hm.running {
		hm.mu.Unlock()
		return
	}
	hm.running = false
	close(hm.stopCh)
	hm.mu.Unlock()
	log.Println("Health monitor stopped")
}

func (hm *HealthMonitor) maintenanceLoop() {
	ticker := time.NewTicker(maintenanceInterval)
	defer ticker.Stop()

	for {
		select {
		case <-hm.stopCh:
			return
		case <-ticker.C:
			hm.runMaintenance()
		}
	}
}

// runMaintenance performs periodic cleanup tasks
func (hm *HealthMonitor) runMaintenance() {
	hm.mu.Lock()
	hm.lastMaintenance = time.Now()
	hm.mu.Unlock()

	log.Println("Running periodic IPFS maintenance")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// 1. Clean up stale swarm connections
	hm.pruneStaleConnections(ctx)

	// 2. Clear provider cache to force fresh discovery
	if hm.booster != nil {
		hm.booster.ClearCache()
	}

	// 3. Reset failure counters
	hm.mu.Lock()
	hm.failureCount = make(map[string]int)
	hm.mu.Unlock()

	log.Println("Periodic maintenance completed")
}

// pruneStaleConnections removes inactive swarm connections
func (hm *HealthMonitor) pruneStaleConnections(ctx context.Context) {
	if hm.node == nil || hm.node.IPFS == nil {
		return
	}

	peers, err := hm.node.IPFS.Swarm().Peers(ctx)
	if err != nil {
		log.Printf("Failed to get swarm peers: %v", err)
		return
	}

	// Identify peers with high latency or no recent activity
	var toDisconnect []string
	for _, p := range peers {
		latency, err := p.Latency()
		// Disconnect peers with latency > 30 seconds (likely stale) or latency error
		if err != nil || latency > 30*time.Second {
			addr := fmt.Sprintf("/p2p/%s", p.ID().String())
			toDisconnect = append(toDisconnect, addr)
		}
	}

	if len(toDisconnect) > 0 {
		log.Printf("Pruning %d stale connections", len(toDisconnect))
		for _, addr := range toDisconnect {
			hm.disconnectPeer(ctx, addr)
		}
	}
}

// disconnectPeer disconnects a peer using CLI
func (hm *HealthMonitor) disconnectPeer(ctx context.Context, addr string) {
	if hm.ipfsMgr == nil || hm.ipfsMgr.BinPath == "" {
		return
	}

	cmd := exec.CommandContext(ctx, hm.ipfsMgr.BinPath, "swarm", "disconnect", addr)
	cmd.Env = append(cmd.Env, "IPFS_PATH="+hm.ipfsMgr.DataDir)
	if err := cmd.Run(); err != nil {
		log.Printf("Failed to disconnect %s: %v", addr, err)
	}
}

// OnDownloadTimeout should be called when a download times out or fails.
// It tracks failures and triggers repair when threshold is reached.
func (hm *HealthMonitor) OnDownloadTimeout(cid string) {
	hm.mu.Lock()
	hm.failureCount[cid]++
	count := hm.failureCount[cid]
	hm.mu.Unlock()

	log.Printf("Download timeout for CID %s (failure count: %d)", cid, count)

	if count >= failureThreshold {
		go hm.repairForCID(cid)
	}
}

// OnDownloadSuccess resets the failure counter for a CID
func (hm *HealthMonitor) OnDownloadSuccess(cid string) {
	hm.mu.Lock()
	delete(hm.failureCount, cid)
	hm.mu.Unlock()
}

// repairForCID performs targeted repair for a specific CID
func (hm *HealthMonitor) repairForCID(cid string) {
	log.Printf("Initiating repair for CID %s", cid)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Clear cached providers for this CID
	if hm.booster != nil {
		hm.booster.ClearCacheForCID(cid)
	}

	// 2. Get and validate current provider connections
	providers := hm.getProvidersForCID(ctx, cid)
	if len(providers) == 0 {
		log.Printf("No cached providers for CID %s, will rediscover on next attempt", cid)
		return
	}

	// 3. Disconnect potentially stale providers
	for _, peerID := range providers {
		if !hm.validateConnection(ctx, peerID) {
			addr := fmt.Sprintf("/p2p/%s", peerID)
			hm.disconnectPeer(ctx, addr)
			log.Printf("Disconnected stale provider %s for CID %s", peerID, cid)
		}
	}

	// 4. Reset failure counter
	hm.mu.Lock()
	delete(hm.failureCount, cid)
	hm.mu.Unlock()

	log.Printf("Repair completed for CID %s", cid)
}

// getProvidersForCID returns cached provider peer IDs for a CID
func (hm *HealthMonitor) getProvidersForCID(ctx context.Context, cid string) []string {
	if hm.booster == nil {
		return nil
	}

	providers := hm.booster.GetCachedProviders(cid)
	var peerIDs []string
	for _, p := range providers {
		peerIDs = append(peerIDs, p.ID.String())
	}
	return peerIDs
}

// validateConnection checks if a peer connection is actually usable
func (hm *HealthMonitor) validateConnection(ctx context.Context, peerID string) bool {
	if hm.node == nil || hm.node.IPFS == nil {
		return false
	}

	// Check if peer is in swarm
	peers, err := hm.node.IPFS.Swarm().Peers(ctx)
	if err != nil {
		return false
	}

	for _, p := range peers {
		if p.ID().String() == peerID {
			// Check latency - if too high, consider it stale
			latency, err := p.Latency()
			if err == nil && latency > 0 && latency < 10*time.Second {
				return true
			}
			// Latency unknown or too high - try ping
			return hm.pingPeer(ctx, peerID)
		}
	}

	return false
}

// pingPeer attempts to ping a peer to verify connectivity
func (hm *HealthMonitor) pingPeer(ctx context.Context, peerID string) bool {
	if hm.ipfsMgr == nil || hm.ipfsMgr.BinPath == "" {
		return false
	}

	pingCtx, cancel := context.WithTimeout(ctx, connectionTestTimeout)
	defer cancel()

	cmd := exec.CommandContext(pingCtx, hm.ipfsMgr.BinPath, "ping", "-n", "1", peerID)
	cmd.Env = append(cmd.Env, "IPFS_PATH="+hm.ipfsMgr.DataDir)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}

	// Check if ping succeeded (output contains "time=")
	return strings.Contains(string(output), "time=")
}

// ForceRefresh triggers immediate maintenance (can be called manually)
func (hm *HealthMonitor) ForceRefresh() {
	go hm.runMaintenance()
}

// GetStatus returns current health monitor status
func (hm *HealthMonitor) GetStatus() map[string]interface{} {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	failureCounts := make(map[string]int)
	for k, v := range hm.failureCount {
		failureCounts[k] = v
	}

	return map[string]interface{}{
		"running":         hm.running,
		"lastMaintenance": hm.lastMaintenance,
		"trackedFailures": len(failureCounts),
		"failureCounts":   failureCounts,
	}
}
