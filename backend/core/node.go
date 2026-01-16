package core

import (
	"context"
	"fmt"
	"io"
	"strings"

	// files "github.com/ipfs/go-ipfs-files"
	// kuborpc "github.com/ipfs/kubo/client/rpc"
	// iface "github.com/ipfs/kubo/core/coreiface"
	// "github.com/ipfs/interface-go-ipfs-core/path"
	// "github.com/multiformats/go-multiaddr"
	
	// We need consistent versions.
	// kuborpc returns boxo/coreiface
	// boxo/coreiface uses boxo/files and boxo/path
	
	kuborpc "github.com/ipfs/kubo/client/rpc"
	iface "github.com/ipfs/kubo/core/coreiface"
	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	"github.com/multiformats/go-multiaddr"
)

type MochiNode struct {
	// External IPFS API
	IPFS iface.CoreAPI
}

// NewNode initializes a connection to an external IPFS node
func NewNode(dataDir string, apiAddr string) (*MochiNode, error) {
	var ipfsApi iface.CoreAPI
	var err error

	if apiAddr != "" {
		// Use provided address
		if err := (&MochiNode{}).UpdateApiUrl(apiAddr); err == nil {
			// Create a temporary node just to reuse the UpdateApiUrl logic? 
			// No, UpdateApiUrl sets n.IPFS.
			// Let's do it manually or helper.
			ma, err := multiaddr.NewMultiaddr(apiAddr)
			if err == nil {
				ipfsApi, err = kuborpc.NewApi(ma)
			}
		}
		if ipfsApi == nil {
			// Fallback or error
			fmt.Printf("Failed to connect to provided API addr %s, falling back to auto-detection\n", apiAddr)
		}
	}
	
	if ipfsApi == nil {
		// Create IPFS HTTP Client (Auto Detect)
		ipfsApi, err = kuborpc.NewLocalApi()
		if err != nil {
			// Fallback to default URL if detection fails
			ma, _ := multiaddr.NewMultiaddr("/ip4/127.0.0.1/tcp/5001")
			ipfsApi, err = kuborpc.NewApi(ma)
			if err != nil {
				// Don't error out yet, just return nil IPFS, allow Start() to retry or Manager to fix it
				fmt.Println("Warning: Could not connect to default IPFS node.")
			}
		}
	}

	return &MochiNode{
		IPFS: ipfsApi,
	}, nil
}

// Start is now just a check
func (n *MochiNode) Start(ctx context.Context) error {
	if n.IPFS == nil {
		return fmt.Errorf("IPFS client not initialized")
	}
	fmt.Println("Connecting to external IPFS Node...")
	
	// Check connection by getting ID
	info, err := n.IPFS.Key().Self(ctx)
	if err != nil {
		return fmt.Errorf("failed to contact IPFS node: %w", err)
	}

	fmt.Printf("Connected to IPFS Node. PeerID: %s\n", info.ID())
	return nil
}

func (n *MochiNode) Stop() error {
	// Nothing to stop for client
	return nil
}

func (n *MochiNode) AddFile(ctx context.Context, reader io.Reader) (string, error) {
	// Create a file node
	node := files.NewReaderFile(reader)
	
	// Add to IPFS
	p, err := n.IPFS.Unixfs().Add(ctx, node)
	if err != nil {
		return "", fmt.Errorf("failed to add file to IPFS: %w", err)
	}
	
	return p.RootCid().String(), nil
}

// Unpin removes a pin for the given CID
func (n *MochiNode) Unpin(ctx context.Context, cidStr string) error {
	cidPath, err := path.NewPath("/ipfs/" + cidStr)
	if err != nil {
		return fmt.Errorf("invalid CID: %w", err)
	}

	// Rm returns a channel of changes, but we just wait for it to finish or error
	err = n.IPFS.Pin().Rm(ctx, cidPath)
	if err != nil {
		return fmt.Errorf("failed to unpin CID: %w", err)
	}

	return nil
}

// Pin recursively pins the given CID
func (n *MochiNode) Pin(ctx context.Context, cidStr string) error {
	cidPath, err := path.NewPath("/ipfs/" + cidStr)
	if err != nil {
		return fmt.Errorf("invalid CID: %w", err)
	}

	// Add pin (recursive by default)
	err = n.IPFS.Pin().Add(ctx, cidPath)
	if err != nil {
		return fmt.Errorf("failed to pin CID: %w", err)
	}

	return nil
}

func (n *MochiNode) ListPins(ctx context.Context) ([]string, error) {
	// List all pins
	pins := make(chan iface.Pin)
	errCh := make(chan error, 1)

	go func() {
		// Prevent panic if channel is already closed by Ls
		defer func() {
			recover()
		}()
		defer close(pins)

		if err := n.IPFS.Pin().Ls(ctx, pins); err != nil {
			errCh <- err
		}
		close(errCh)
	}()

	var cidList []string
	for p := range pins {
		// Filter for recursive and direct pins (user explicitly pinned)
		if p.Type() == "recursive" || p.Type() == "direct" {
			cidList = append(cidList, p.Path().RootCid().String())
		}
	}

	if err := <-errCh; err != nil {
		return nil, fmt.Errorf("failed to list pins: %w", err)
	}

	return cidList, nil
}

func (n *MochiNode) GetFile(ctx context.Context, cidStr string) (io.Reader, error) {
	// Boxo path handling
	cidPath, err := path.NewPath("/ipfs/" + cidStr)
	if err != nil {
		return nil, err
	}
	
	node, err := n.IPFS.Unixfs().Get(ctx, cidPath)
	if err != nil {
		return nil, err
	}
	
	if f, ok := node.(files.File); ok {
		return f, nil
	}
	
	return nil, fmt.Errorf("node is not a file")
}

func (n *MochiNode) GetFileSize(ctx context.Context, cidStr string) (int64, error) {
	// Boxo path handling
	cidPath, err := path.NewPath("/ipfs/" + cidStr)
	if err != nil {
		return 0, err
	}
	
	node, err := n.IPFS.Unixfs().Get(ctx, cidPath)
	if err != nil {
		return 0, err
	}
	
	if f, ok := node.(files.File); ok {
		return f.Size()
	}
	
	return 0, fmt.Errorf("node is not a file")
}

func (n *MochiNode) ListBlocks(ctx context.Context) ([]string, error) {
	// This function is less relevant for client mode, 
	// or we can use Pins/MFS to list files.
	// For now, return empty to satisfy interface if any
	return []string{}, nil
}

func (n *MochiNode) UpdateApiUrl(urlStr string) error {
	var ma multiaddr.Multiaddr
	var err error

	if strings.HasPrefix(urlStr, "/") {
		// Assume multiaddr (e.g. /ip4/127.0.0.1/tcp/5001)
		ma, err = multiaddr.NewMultiaddr(urlStr)
	} else {
		// Parse URL (e.g. http://127.0.0.1:5001) to multiaddr
		// We need to convert http url to multiaddr
		// Simple assumption: /ip4/IP/tcp/PORT
		
		// Strip http://
		urlStr = strings.TrimPrefix(urlStr, "http://")
		urlStr = strings.TrimPrefix(urlStr, "https://")
		
		parts := strings.Split(urlStr, ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid url format")
		}
		
		ip := parts[0]
		port := parts[1]
		
		maStr := fmt.Sprintf("/ip4/%s/tcp/%s", ip, port)
		ma, err = multiaddr.NewMultiaddr(maStr)
	}

	if err != nil {
		return err
	}
	
	newApi, err := kuborpc.NewApi(ma)
	if err != nil {
		return err
	}
	
	n.IPFS = newApi
	fmt.Printf("Updated IPFS API Connection to %s\n", ma.String())
	return nil
}
