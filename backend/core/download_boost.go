package core

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	"github.com/libp2p/go-libp2p/core/peer"
)

type DownloadBooster struct {
	node          *MochiNode
	providerCache sync.Map // string(CID) -> []peer.AddrInfo
	connPool      sync.Map // string(peer.ID) -> int64 (latency in ms)
	mu            sync.RWMutex
}

func NewDownloadBooster(node *MochiNode) *DownloadBooster {
	return &DownloadBooster{
		node: node,
	}
}

// WarmupCID discovers providers for a CID and pre-establishes connections
func (db *DownloadBooster) WarmupCID(ctx context.Context, cid string) error {
	if db.node == nil {
		return fmt.Errorf("node not initialized")
	}

	// Check if already cached
	if cached, ok := db.providerCache.Load(cid); ok {
		providers := cached.([]peer.AddrInfo)
		if len(providers) > 0 {
			log.Printf("Using cached providers for CID %s: %d providers", cid, len(providers))
			return nil
		}
	}

	log.Printf("Starting provider discovery for CID: %s", cid)

	// Find providers
	provChan, err := db.node.FindProviders(ctx, cid)
	if err != nil {
		return fmt.Errorf("failed to find providers: %w", err)
	}

	var providers []peer.AddrInfo
	timeout := time.After(10 * time.Second)
	fastConnCount := 0

	for {
		select {
		case p, ok := <-provChan:
			if !ok {
				goto ConnectPhase
			}
			providers = append(providers, p)
			log.Printf("Found provider %d: %s", len(providers), p.ID.String())

			// Immediately connect to first few providers for fast start
			if fastConnCount < 3 {
				fastConnCount++
				go db.connectWithRetry(ctx, p)
			}

		case <-timeout:
			log.Printf("Provider discovery timeout, found %d providers", len(providers))
			goto ConnectPhase

		case <-ctx.Done():
			return ctx.Err()
		}
	}

ConnectPhase:
	// Cache providers
	db.providerCache.Store(cid, providers)

	if len(providers) == 0 {
		return fmt.Errorf("no providers found for CID")
	}

	// Connect to remaining providers in background
	go db.batchConnect(context.Background(), providers, fastConnCount)

	// Actively prefetch first block to trigger Bitswap session
	go db.prefetchFirstBlock(context.Background(), cid)

	log.Printf("Warmup complete for CID %s: %d providers discovered", cid, len(providers))
	return nil
}

// connectWithRetry attempts to connect to a provider and measures latency
func (db *DownloadBooster) connectWithRetry(ctx context.Context, info peer.AddrInfo) {
	if len(info.Addrs) == 0 {
		log.Printf("No addresses for peer %s", info.ID.String())
		return
	}

	// Build multiaddr string
	addr := fmt.Sprintf("%s/p2p/%s", info.Addrs[0].String(), info.ID.String())

	// Measure connection latency
	start := time.Now()
	err := db.node.Connect(ctx, addr)
	latency := time.Since(start)

	if err != nil {
		log.Printf("Failed to connect to provider %s: %v", info.ID.String(), err)
		return
	}

	// Record connection quality
	db.connPool.Store(info.ID.String(), latency.Milliseconds())
	log.Printf("Connected to provider %s (latency: %dms)", info.ID.String(), latency.Milliseconds())

	// Add to peering for connection protection
	go func() {
		peerCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := db.node.PeeringAdd(peerCtx, addr); err != nil {
			log.Printf("Warning: Failed to add peer to peering list: %v", err)
		}
	}()
}

// batchConnect connects to multiple providers in parallel
func (db *DownloadBooster) batchConnect(ctx context.Context, providers []peer.AddrInfo, skipFirst int) {
	if len(providers) <= skipFirst {
		return
	}

	remaining := providers[skipFirst:]
	var wg sync.WaitGroup

	// Limit concurrent connections
	semaphore := make(chan struct{}, 5)

	for _, p := range remaining {
		wg.Add(1)
		go func(info peer.AddrInfo) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()

			db.connectWithRetry(connectCtx, info)
		}(p)
	}

	wg.Wait()
}

// prefetchFirstBlock actively fetches the first block to trigger Bitswap session
func (db *DownloadBooster) prefetchFirstBlock(ctx context.Context, cid string) {
	prefetchCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	log.Printf("Prefetching first block for CID %s to establish Bitswap session", cid)

	cidPath, err := path.NewPath("/ipfs/" + cid)
	if err != nil {
		log.Printf("Failed to create path for prefetch: %v", err)
		return
	}

	node, err := db.node.IPFS.Unixfs().Get(prefetchCtx, cidPath)
	if err != nil {
		log.Printf("Prefetch failed (acceptable, will retry on actual download): %v", err)
		return
	}

	if f, ok := node.(files.File); ok {
		buf := make([]byte, 256*1024)
		n, _ := f.Read(buf)
		f.Close()
		log.Printf("Successfully prefetched %d bytes for %s, Bitswap session established", n, cid)
	} else if d, ok := node.(files.Directory); ok {
		d.Close()
		log.Printf("Prefetch discovered directory structure for %s", cid)
	}
}

// GetCachedProviders returns cached providers for a CID
func (db *DownloadBooster) GetCachedProviders(cid string) []peer.AddrInfo {
	if cached, ok := db.providerCache.Load(cid); ok {
		return cached.([]peer.AddrInfo)
	}
	return nil
}

// GetConnectionQuality returns connection latency for a peer
func (db *DownloadBooster) GetConnectionQuality(peerID string) int64 {
	if latency, ok := db.connPool.Load(peerID); ok {
		return latency.(int64)
	}
	return -1
}

// ClearCache clears the provider cache
func (db *DownloadBooster) ClearCache() {
	db.providerCache = sync.Map{}
	log.Println("Provider cache cleared")
}
