package core

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type IpfsManager struct {
	DataDir string
	BinPath string
	Cmd     *exec.Cmd
	mu      sync.Mutex
	ApiAddr string
}

func NewIpfsManager(dataDir string) (*IpfsManager, error) {
	// Find IPFS binary
	// Development: ../electron/resources/bin/ipfs.exe
	// Production: ./ipfs.exe (in the same dir as mochibox-core) or ../bin/ipfs.exe

	binName := "ipfs"
	if runtime.GOOS == "windows" {
		binName = "ipfs.exe"
	}

	// Try multiple locations
	possiblePaths := []string{
		filepath.Join(filepath.Dir(os.Args[0]), binName),
		filepath.Join(filepath.Dir(os.Args[0]), "../bin", binName),   // Electron production structure
		filepath.Join("..", "electron", "resources", "bin", binName), // Dev
	}

	binPath := ""

	// 1. Check env var first (from Electron)
	if envPath := os.Getenv("MOCHIBOX_IPFS_BIN"); envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			binPath = envPath
			log.Printf("Using IPFS binary from env: %s\n", binPath)
		} else {
			log.Printf("IPFS binary from env not found: %s\n", envPath)
		}
	}

	if binPath == "" {
		for _, p := range possiblePaths {
			abs, _ := filepath.Abs(p)
			if _, err := os.Stat(abs); err == nil {
				binPath = abs
				break
			}
		}
	}

	if binPath == "" {
		// Fallback to PATH
		path, err := exec.LookPath(binName)
		if err == nil {
			binPath = path
		}
	}

	repoPath := filepath.Join(dataDir, "ipfs-repo")

	return &IpfsManager{
		DataDir: repoPath,
		BinPath: binPath,
	}, nil
}

func (m *IpfsManager) InitRepo() error {
	if m.BinPath == "" {
		return fmt.Errorf("ipfs binary not found")
	}

	// Check if repo exists (config file)
	configPath := filepath.Join(m.DataDir, "config")
	if _, err := os.Stat(configPath); err == nil {
		return nil // Already initialized
	}

	// ipfs init (default profile supports local discovery)
	// We avoid "server" profile for desktop use to enable MDNS/Local Discovery by default.
	cmd := exec.Command(m.BinPath, "init")
	cmd.Env = append(os.Environ(), "IPFS_PATH="+m.DataDir)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ipfs init failed: %v, output: %s", err, string(output))
	}

	return m.ConfigRepo()
}

func (m *IpfsManager) ConfigRepo() error {
	// Set custom config to avoid conflicts
	// We use port 0 for API and Gateway to let OS assign random ports
	mdnsEnabled := true
	if v := strings.TrimSpace(os.Getenv("MOCHIBOX_MDNS")); v != "" {
		if v == "0" || strings.EqualFold(v, "false") {
			mdnsEnabled = false
		}
	}

	const configVersion = "2026-01-27-2"
	markerPath := filepath.Join(m.DataDir, "mochibox.ipfs.config.version")
	if strings.TrimSpace(os.Getenv("MOCHIBOX_FORCE_CONFIG")) == "" {
		if b, err := os.ReadFile(markerPath); err == nil {
			if strings.TrimSpace(string(b)) == configVersion {
				return nil
			}
		}
	}

	criticalConfigs := [][]string{
		{"Addresses.API", `"/ip4/127.0.0.1/tcp/0"`},
		{"Addresses.Gateway", `"/ip4/127.0.0.1/tcp/0"`},
		// Enable CORS for frontend
		{"API.HTTPHeaders.Access-Control-Allow-Origin", `["http://localhost:5173", "app://*"]`},
		{"API.HTTPHeaders.Access-Control-Allow-Methods", `["PUT", "POST", "GET"]`},
		{"Discovery.MDNS.Enabled", strconv.FormatBool(mdnsEnabled)},
		{"Pubsub.Router", `"gossipsub"`},
	}

	optionalConfigs := [][]string{
		// Optimization: DHT & NAT
		{"Routing.Type", `"dht"`}, // Force DHT (Server mode if possible, or auto)
		{"Swarm.EnableAutoNATService", "true"},
		{"Routing.AcceleratedDHTClient", "true"}, // Fast DHT (Kubo >= 0.21)

		// Optimization: Connection Manager
		// Higher defaults for stable p2p connections (both downloading and sharing)
		// LowWater: minimum connections to maintain
		// HighWater: trigger cleanup when exceeded
		// GracePeriod: new connections immune to cleanup for this duration
		{"Swarm.ConnMgr.LowWater", "400"},
		{"Swarm.ConnMgr.HighWater", "1000"},
		{"Swarm.ConnMgr.GracePeriod", `"60s"`},

		// Optimization: Relay Client & Transports
		{"Swarm.RelayClient.Enabled", "true"},
		{"Swarm.Transports.Network.Relay", "true"},

		// Optimization: QUIC & WebTransport (Faster, better roaming)
		{"Swarm.Transports.Network.QUIC", "true"},
		{"Swarm.Transports.Network.WebTransport", "true"},

		// Feature: Filestore (No Copy)
		{"Experimental.FilestoreEnabled", "true"},

		// === Phase 1 Optimization: Enhanced IPFS Configuration ===

		// Block storage optimization with bloom filter for faster lookups
		{"Datastore.BloomFilterSize", "1048576"}, // 1MB bloom filter

		// Resource manager to prevent resource exhaustion
		{"Swarm.ResourceMgr.Enabled", "true"},
		{"Swarm.ResourceMgr.MaxMemory", `"1GB"`},
		{"Swarm.ResourceMgr.MaxFileDescriptors", "4096"},

		// Reprovider optimization (for file sharers)
		{"Reprovider.Interval", `"12h"`},   // Reduce frequency to save resources
		{"Reprovider.Strategy", `"roots"`}, // Only announce root CIDs, not every block

		// Bitswap optimization for better throughput
		{"Swarm.Transports.Network.Bitswap.MaxOutstandingBytesPerPeer", `"5MB"`},
		{"Swarm.Transports.Network.Bitswap.TargetMessageSize", `"2MB"`},
	}

	for _, cfg := range criticalConfigs {
		cmd := exec.Command(m.BinPath, "config", "--json", cfg[0], cfg[1])
		cmd.Env = append(os.Environ(), "IPFS_PATH="+m.DataDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("config %s failed: %v, out: %s", cfg[0], err, string(out))
		}
	}

	for _, cfg := range optionalConfigs {
		cmd := exec.Command(m.BinPath, "config", "--json", cfg[0], cfg[1])
		cmd.Env = append(os.Environ(), "IPFS_PATH="+m.DataDir)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Printf("Warning: Failed to apply optional config %s: %v, out: %s", cfg[0], err, string(out))
		}
	}

	_ = os.WriteFile(markerPath, []byte(configVersion), 0644)
	return nil
}

// AddPeering adds a peer to the peering subsystem via CLI
// This ensures the peer is protected from connection manager trimming and reconnects are attempted.
func (m *IpfsManager) AddPeering(ctx context.Context, multiaddr string) error {
	if m.BinPath == "" {
		return fmt.Errorf("ipfs binary not found")
	}

	// ipfs swarm peering add <multiaddr>
	cmd := exec.CommandContext(ctx, m.BinPath, "swarm", "peering", "add", multiaddr)
	cmd.Env = append(os.Environ(), "IPFS_PATH="+m.DataDir)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Ignore if already present or specific harmless errors if needed
		// But usually we want to know
		return fmt.Errorf("peering add failed: %v, out: %s", err, string(output))
	}

	log.Printf("Successfully added peer to peering list: %s", multiaddr)
	return nil
}

func (m *IpfsManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.Cmd != nil && m.Cmd.Process != nil {
		return nil // Already running
	}

	if m.BinPath == "" {
		return fmt.Errorf("ipfs binary not found")
	}

	// If repo.lock exists, try to gracefully shutdown any lingering daemon
	lockPath := filepath.Join(m.DataDir, "repo.lock")
	if _, err := os.Stat(lockPath); err == nil {
		// Try API shutdown first
		if m.ApiAddr != "" {
			_ = tryShutdownAPI(m.ApiAddr)
			time.Sleep(1 * time.Second)
		}
		// Try CLI shutdown as fallback
		_ = tryShutdownCLI(m.BinPath, m.DataDir)
		time.Sleep(1 * time.Second)
	}

	// Remove api file if exists to avoid reading stale address
	os.Remove(filepath.Join(m.DataDir, "api"))
	os.Remove(filepath.Join(m.DataDir, "gateway"))

	mdnsEnabled := true
	if v := strings.TrimSpace(os.Getenv("MOCHIBOX_MDNS")); v != "" {
		if v == "0" || strings.EqualFold(v, "false") {
			mdnsEnabled = false
		}
	}

	// Force apply optimization config every start to ensure updates
	if err := m.ConfigRepo(); err != nil {
		log.Printf("Warning: Failed to apply optimization config: %v", err)
	}

	healCmd := exec.Command(m.BinPath, "config", "Discovery.MDNS.Enabled", strconv.FormatBool(mdnsEnabled), "--json")
	healCmd.Env = append(os.Environ(), "IPFS_PATH="+m.DataDir)
	if err := healCmd.Run(); err != nil {
		log.Printf("Warning: Failed to ensure MDNS config: %v", err)
	}

	cmd := exec.CommandContext(ctx, m.BinPath, "daemon", "--enable-gc", "--enable-pubsub-experiment")
	cmd.Env = append(os.Environ(), "IPFS_PATH="+m.DataDir)

	// Capture stdout/stderr for logging
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		log.Printf("Failed to start IPFS daemon: %v", err)
		return err
	}
	m.Cmd = cmd

	go io.Copy(os.Stdout, stdout)
	go io.Copy(os.Stderr, stderr)

	// Wait for API file to appear
	// This confirms the daemon is ready and tells us the port
	apiFile := filepath.Join(m.DataDir, "api")

	timeout := time.After(60 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			m.Stop()
			return fmt.Errorf("timeout waiting for ipfs daemon to start")
		case <-ticker.C:
			content, err := os.ReadFile(apiFile)
			if err == nil && len(content) > 0 {
				m.ApiAddr = strings.TrimSpace(string(content))

				// Health check: wait for API to be responsive
				// Convert multiaddr to http
				checkUrl := MultiaddrToHttp(m.ApiAddr) + "/api/v0/id"

				// Try a few times to ensure it's ready
				for i := 0; i < 20; i++ {
					// Use Post as IPFS RPC uses POST
					resp, err := http.Post(checkUrl, "application/json", nil)
					if err == nil {
						resp.Body.Close()
						if resp.StatusCode == 200 {
							fmt.Printf("Managed IPFS Node started and ready at %s\n", m.ApiAddr)
							return nil
						}
					}
					time.Sleep(200 * time.Millisecond)
				}

				fmt.Printf("Managed IPFS Node started at %s (API check timed out)\n", m.ApiAddr)
				return nil
			}
			// Check if process died
			if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
				return fmt.Errorf("ipfs daemon exited unexpectedly")
			}
		}
	}
}

func (m *IpfsManager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Prefer graceful shutdown via API if possible
	if m.ApiAddr != "" {
		if err := tryShutdownAPI(m.ApiAddr); err == nil {
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Fallback to CLI shutdown
	_ = tryShutdownCLI(m.BinPath, m.DataDir)

	// If we still have a running process, force kill
	if m.Cmd != nil && m.Cmd.Process != nil {
		if runtime.GOOS == "windows" {
			// Kill entire process tree
			_ = exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(m.Cmd.Process.Pid)).Run()
		} else {
			m.Cmd.Process.Signal(os.Interrupt)
			time.Sleep(500 * time.Millisecond)
			m.Cmd.Process.Kill()
		}
		m.Cmd = nil
	}
	return nil
}

// GetGatewayAddr reads the gateway file similarly to api file
func (m *IpfsManager) GetGatewayAddr() string {
	// IPFS doesn't always write a 'gateway' file like 'api' file in older versions,
	// but newer Kubo does.
	// If not, we might need to parse config.
	// Let's try reading the file first.
	content, err := os.ReadFile(filepath.Join(m.DataDir, "gateway"))
	if err == nil {
		return strings.TrimSpace(string(content))
	}

	// Fallback: Read config
	// But since we use port 0, config just says port 0 until running?
	// Actually 'gateway' file is the reliable way for ephemeral ports.
	return ""
}

// MultiaddrToHttp converts /ip4/127.0.0.1/tcp/8080 to http://127.0.0.1:8080
func MultiaddrToHttp(maStr string) string {
	parts := strings.Split(maStr, "/")
	// /ip4/127.0.0.1/tcp/8080 -> ["", "ip4", "127.0.0.1", "tcp", "8080"]
	if len(parts) >= 5 && parts[1] == "ip4" && parts[3] == "tcp" {
		return fmt.Sprintf("http://%s:%s", parts[2], parts[4])
	}
	return ""
}

// tryShutdownAPI calls /api/v0/shutdown on the managed API address
func tryShutdownAPI(apiAddr string) error {
	url := MultiaddrToHttp(apiAddr)
	if url == "" {
		return fmt.Errorf("invalid api addr")
	}
	_, err := http.Post(url+"/api/v0/shutdown", "application/json", nil)
	if err != nil {
		log.Printf("API shutdown failed: %v", err)
		return err
	}
	return nil
}

// tryShutdownCLI executes `ipfs shutdown` with IPFS_PATH to gracefully stop daemon
func tryShutdownCLI(binPath, dataDir string) error {
	if binPath == "" {
		return fmt.Errorf("no ipfs bin")
	}
	cmd := exec.Command(binPath, "shutdown")
	cmd.Env = append(os.Environ(), "IPFS_PATH="+dataDir)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("CLI shutdown failed: %v, out: %s", err, string(out))
		return err
	}
	return nil
}
