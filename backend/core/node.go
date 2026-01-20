package core

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"sort"
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
	"github.com/ipfs/kubo/core/coreiface/options"
	"github.com/ipfs/boxo/files"
	"github.com/ipfs/boxo/path"
	"github.com/multiformats/go-multiaddr"
    "github.com/libp2p/go-libp2p/core/peer"
	"os"
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

func (n *MochiNode) AddFileNoCopy(ctx context.Context, filePath string) (string, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to stat file: %w", err)
	}

	// Create a serial file node (references path)
	node, err := files.NewSerialFile(filePath, false, stat)
	if err != nil {
		return "", fmt.Errorf("failed to create serial file: %w", err)
	}

	// Add to IPFS with Nocopy option
	p, err := n.IPFS.Unixfs().Add(ctx, node, options.Unixfs.Nocopy(true))
	if err != nil {
		return "", fmt.Errorf("failed to add file (nocopy) to IPFS: %w", err)
	}

	return p.RootCid().String(), nil
}

type FileEntry struct {
	Path   string
	Reader io.Reader
}

func (n *MochiNode) AddDirectory(ctx context.Context, entries []FileEntry) (string, error) {
	if len(entries) == 0 {
		return "", fmt.Errorf("no files in directory")
	}

	rootNode, err := buildTree(entries)
	if err != nil {
		return "", err
	}

	p, err := n.IPFS.Unixfs().Add(ctx, rootNode)
	if err != nil {
		return "", fmt.Errorf("failed to add directory: %w", err)
	}

	return p.RootCid().String(), nil
}

func buildTree(entries []FileEntry) (files.Node, error) {
	subs := make(map[string][]FileEntry)
	filesInDir := make(map[string]io.Reader)

	for _, e := range entries {
		parts := strings.SplitN(e.Path, "/", 2)
		if len(parts) == 1 {
			filesInDir[parts[0]] = e.Reader
		} else {
			dirName := parts[0]
			restPath := parts[1]
			subs[dirName] = append(subs[dirName], FileEntry{Path: restPath, Reader: e.Reader})
		}
	}

	var dirEntries []files.DirEntry

	for name, reader := range filesInDir {
		dirEntries = append(dirEntries, files.FileEntry(name, files.NewReaderFile(reader)))
	}

	for name, subEntries := range subs {
		subNode, err := buildTree(subEntries)
		if err != nil {
			return nil, err
		}
		dirEntries = append(dirEntries, files.FileEntry(name, subNode))
	}

	sort.Slice(dirEntries, func(i, j int) bool {
		return dirEntries[i].Name() < dirEntries[j].Name()
	})

	return files.NewSliceDirectory(dirEntries), nil
}

type DirItem struct {
	Name string `json:"name"`
	CID  string `json:"cid"`
	Size int64  `json:"size"`
	Type string `json:"type"` // "file" or "dir"
}

func (n *MochiNode) ListDirectory(ctx context.Context, cidStr string) ([]DirItem, error) {
	cidPath, err := path.NewPath("/ipfs/" + cidStr)
	if err != nil {
		return nil, err
	}
	
	node, err := n.IPFS.Unixfs().Get(ctx, cidPath)
	if err != nil {
		return nil, err
	}
	
	dir, ok := node.(files.Directory)
	if !ok {
		return nil, fmt.Errorf("node is not a directory")
	}

	var items []DirItem
	it := dir.Entries()
	for it.Next() {
		name := it.Name()
		subNode := it.Node()
		
		item := DirItem{
			Name: name,
			Type: "file",
		}
		
		if f, ok := subNode.(files.File); ok {
			item.Size, _ = f.Size()
		} else if _, ok := subNode.(files.Directory); ok {
			item.Type = "dir"
		}
		
		// Resolve CID for sub-item
		// Construct path: /ipfs/<RootCID>/<Name>
		subPathStr := "/ipfs/" + cidStr + "/" + name
		subPath, err := path.NewPath(subPathStr)
		if err == nil {
			// Use ResolvePath to get the CID
			// Note: This adds overhead but is necessary for navigation
			// n.IPFS is iface.CoreAPI
			// It seems in this version it returns 3 values? (resolved, remainder, error)? or similar?
			// Let's try to ignore the second return value.
			resolved, _, err := n.IPFS.ResolvePath(ctx, subPath)
			if err == nil {
				// resolved is likely path.ImmutablePath or similar which might use RootCid() or similar?
				// Error says: type "github.com/ipfs/boxo/path".ImmutablePath has no field or method Cid
				// It probably has RootCid()
				item.CID = resolved.RootCid().String()
			} else {
				fmt.Printf("Warning: Failed to resolve subpath %s: %v\n", subPathStr, err)
			}
		}
		
		items = append(items, item)
	}
	
	return items, nil
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
	
	if d, ok := node.(files.Directory); ok {
		// It's a directory, return a ZIP stream
		return zipDirectory(ctx, d)
	}

	return nil, fmt.Errorf("node is not a file or directory")
}

func zipDirectory(ctx context.Context, dir files.Directory) (io.Reader, error) {
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()
		
		zw := zip.NewWriter(pw)
		defer zw.Close()

		// Helper to walk the directory
		var walk func(d files.Directory, prefix string) error
		walk = func(d files.Directory, prefix string) error {
			it := d.Entries()
			for it.Next() {
				name := it.Name()
				node := it.Node()
				
				currPath := name
				if prefix != "" {
					currPath = prefix + "/" + name
				}

				if f, ok := node.(files.File); ok {
					w, err := zw.Create(currPath)
					if err != nil {
						return err
					}
					if _, err := io.Copy(w, f); err != nil {
						return err
					}
					f.Close()
				} else if subDir, ok := node.(files.Directory); ok {
					if err := walk(subDir, currPath); err != nil {
						return err
					}
					subDir.Close()
				}
			}
			return it.Err()
		}

		if err := walk(dir, ""); err != nil {
			pw.CloseWithError(err)
		}
	}()

	return pr, nil
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

func (n *MochiNode) FindProviders(ctx context.Context, cidStr string) (<-chan peer.AddrInfo, error) {
    // If cidStr doesn't start with /ipfs/, path.NewPath might handle it or not depending on version.
    // Safest is to ensure /ipfs/ prefix or just rely on library.
    p := cidStr
    if !strings.HasPrefix(p, "/ipfs/") {
        p = "/ipfs/" + p
    }
    
    cidPath, err := path.NewPath(p)
    if err != nil {
        return nil, err
    }
    
    return n.IPFS.Routing().FindProviders(ctx, cidPath)
}

func (n *MochiNode) Connect(ctx context.Context, maStr string) error {
    maStr = strings.TrimSpace(maStr)
    if maStr == "" {
        return fmt.Errorf("empty multiaddr")
    }

    info, err := peer.AddrInfoFromString(maStr)
    if err != nil {
        ma, maErr := multiaddr.NewMultiaddr(maStr)
        if maErr != nil {
            return err
        }
        info, err = peer.AddrInfoFromP2pAddr(ma)
        if err != nil {
            return err
        }
    }

    return n.IPFS.Swarm().Connect(ctx, *info)
}

// PeeringAdd adds a peer to the peering subsystem
func (n *MochiNode) PeeringAdd(ctx context.Context, addr string) error {
	// Note: go-ipfs-core-iface doesn't expose Peering service directly yet in some versions.
	// But we can fallback to config modification or just Swarm Connect for now.
	// Actually, for permanent peering, we should edit the config.
	// But since we are using CoreAPI, let's see if we can just "Connect" and rely on Connection Manager tags if exposed.
	// Since we can't easily edit config via CoreAPI without restarting or using specific Config API (which n.IPFS has).
	
	// Let's try to add to config via API
	// n.IPFS.Config().Set(ctx, "Peering.Peers", ...) is complex json manipulation.
	
	// For now, we will just ensure we Connect (which resets the idle timer).
	// Implementing full Peering via config requires parsing the current config, adding to list, and saving.
	return n.Connect(ctx, addr)
}
