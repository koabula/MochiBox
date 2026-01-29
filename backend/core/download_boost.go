package core

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	"github.com/libp2p/go-libp2p/core/peer"
)

type cacheEntry struct {
	providers  []peer.AddrInfo
	cachedAt   time.Time
	isNegative bool // true if no providers were found
}

type DownloadBooster struct {
	node          *MochiNode
	providerCache sync.Map // string(CID) -> *cacheEntry
	connPool      sync.Map // string(peer.ID) -> int64 (latency in ms)
	mu            sync.RWMutex
}

const (
	negativeCacheTTL         = 5 * time.Second // Cache "no providers" for 5s
	positiveCacheTTL         = 5 * time.Minute  // Cache valid providers for 5min
	minProvidersForEarlyExit = 2                // Exit early after finding this many
	maxConnectedForEarlyExit = 1                // Exit early after connecting to this many
)

func NewDownloadBooster(node *MochiNode) *DownloadBooster {
	return &DownloadBooster{
		node: node,
	}
}

// WarmupCID discovers providers for a CID and pre-establishes connections
// Features:
// - Early exit: returns as soon as enough providers are found and connected
// - Negative cache: caches "no providers" for 30s to avoid repeated discovery
// - Positive cache: reuses previously found providers
func (db *DownloadBooster) WarmupCID(ctx context.Context, cid string) error {
	if db.node == nil {
		return fmt.Errorf("node not initialized")
	}

	// Check cache first
	if cached, ok := db.providerCache.Load(cid); ok {
		entry := cached.(*cacheEntry)
		age := time.Since(entry.cachedAt)

		if entry.isNegative {
			// Negative cache: check if expired
			if age < negativeCacheTTL {
				log.Printf("Negative cache hit for CID %s (age: %v), skipping discovery", cid, age)
				return fmt.Errorf("no providers found (cached)")
			}
			// Expired, continue with discovery
		} else if len(entry.providers) > 0 {
			// Positive cache: check if still valid
			if age < positiveCacheTTL {
				log.Printf("Using cached providers for CID %s: %d providers (age: %v)", cid, len(entry.providers), age)
				return nil
			}
		}
	}

	log.Printf("Starting provider discovery for CID: %s", cid)

	// Find providers
	provChan, err := db.node.FindProviders(ctx, cid)
	if err != nil {
		return fmt.Errorf("failed to find providers: %w", err)
	}

	var providers []peer.AddrInfo
	var connectedCount int32
	timeout := time.After(10 * time.Second)
	earlyExit := make(chan struct{})

	// Track connection success for early exit
	onConnected := func() {
		if atomic.AddInt32(&connectedCount, 1) >= maxConnectedForEarlyExit {
			select {
			case earlyExit <- struct{}{}:
			default:
			}
		}
	}

	for {
		select {
		case p, ok := <-provChan:
			if !ok {
				goto ConnectPhase
			}
			providers = append(providers, p)
			log.Printf("Found provider %d: %s", len(providers), p.ID.String())

			// Connect immediately and track success
			go func(info peer.AddrInfo) {
				if db.connectProvider(ctx, info) {
					onConnected()
				}
			}(p)

			// Early exit check: enough providers found
			if len(providers) >= minProvidersForEarlyExit {
				log.Printf("Found %d providers, checking for early exit", len(providers))
				// Give a short window for connection to complete
				select {
				case <-earlyExit:
					log.Printf("Early exit: connected to provider, proceeding with download")
					goto ConnectPhase
				case <-time.After(500 * time.Millisecond):
					// Continue collecting more providers
				}
			}

		case <-earlyExit:
			log.Printf("Early exit triggered: connected to provider")
			goto ConnectPhase

		case <-timeout:
			log.Printf("Provider discovery timeout, found %d providers", len(providers))
			goto ConnectPhase

		case <-ctx.Done():
			return ctx.Err()
		}
	}

ConnectPhase:
	// Cache the result
	entry := &cacheEntry{
		providers:  providers,
		cachedAt:   time.Now(),
		isNegative: len(providers) == 0,
	}
	db.providerCache.Store(cid, entry)

	if len(providers) == 0 {
		return fmt.Errorf("no providers found for CID")
	}

	// Start prefetch and wait with short timeout (500ms)
	// If prefetch takes longer, it continues in background while download starts
	prefetchDone := db.PrefetchFirstBlock(context.Background(), cid)
	db.WaitForPrefetch(prefetchDone, 500*time.Millisecond)

	log.Printf("Warmup complete for CID %s: %d providers discovered", cid, len(providers))
	return nil
}

// connectProvider attempts to connect to a provider and returns success status
func (db *DownloadBooster) connectProvider(ctx context.Context, info peer.AddrInfo) bool {
	if len(info.Addrs) == 0 {
		return false
	}

	// Build multiaddr string
	addr := fmt.Sprintf("%s/p2p/%s", info.Addrs[0].String(), info.ID.String())

	// Use short timeout for connection
	connCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	start := time.Now()
	err := db.node.Connect(connCtx, addr)
	latency := time.Since(start)

	if err != nil {
		log.Printf("Failed to connect to provider %s: %v", info.ID.String(), err)
		return false
	}

	// Record connection quality
	db.connPool.Store(info.ID.String(), latency.Milliseconds())
	log.Printf("Connected to provider %s (latency: %dms)", info.ID.String(), latency.Milliseconds())

	// Add to peering in background
	go func() {
		peerCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		db.node.PeeringAdd(peerCtx, addr)
	}()

	return true
}

// PrefetchFirstBlock actively fetches the first block to trigger Bitswap session.
// It waits up to shortTimeout for quick completion, then continues async if needed.
// Returns a channel that closes when prefetch completes (success or failure).
func (db *DownloadBooster) PrefetchFirstBlock(ctx context.Context, cid string) <-chan struct{} {
	done := make(chan struct{})

	go func() {
		defer close(done)

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
	}()

	return done
}

// WaitForPrefetch waits for prefetch with a short timeout.
// If prefetch doesn't complete within shortTimeout, it returns immediately
// and the prefetch continues in the background.
func (db *DownloadBooster) WaitForPrefetch(prefetchDone <-chan struct{}, shortTimeout time.Duration) bool {
	select {
	case <-prefetchDone:
		return true // Prefetch completed within timeout
	case <-time.After(shortTimeout):
		log.Printf("Prefetch not completed within %v, continuing with download", shortTimeout)
		return false // Timeout, prefetch continues in background
	}
}

// GetCachedProviders returns cached providers for a CID
func (db *DownloadBooster) GetCachedProviders(cid string) []peer.AddrInfo {
	if cached, ok := db.providerCache.Load(cid); ok {
		entry := cached.(*cacheEntry)
		if !entry.isNegative && time.Since(entry.cachedAt) < positiveCacheTTL {
			return entry.providers
		}
	}
	return nil
}

// HasCachedProviders checks if valid providers are cached for a CID
func (db *DownloadBooster) HasCachedProviders(cid string) bool {
	if cached, ok := db.providerCache.Load(cid); ok {
		entry := cached.(*cacheEntry)
		if entry.isNegative {
			return false
		}
		if len(entry.providers) > 0 && time.Since(entry.cachedAt) < positiveCacheTTL {
			return true
		}
	}
	return false
}

// GetConnectionQuality returns connection latency for a peer
func (db *DownloadBooster) GetConnectionQuality(peerID string) int64 {
	if latency, ok := db.connPool.Load(peerID); ok {
		return latency.(int64)
	}
	return -1
}

// ClearCache clears the entire provider cache
func (db *DownloadBooster) ClearCache() {
	db.providerCache = sync.Map{}
	log.Println("Provider cache cleared")
}

// ClearCacheForCID removes cached providers for a specific CID
func (db *DownloadBooster) ClearCacheForCID(cid string) {
	db.providerCache.Delete(cid)
	log.Printf("Provider cache cleared for CID: %s", cid)
}
