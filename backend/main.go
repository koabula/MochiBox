package main

import (
	"bufio"
	"context"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"mochibox-core/api"
	"mochibox-core/core"
	"mochibox-core/crypto"
	"mochibox-core/db"
)

func main() {
	// 1. Setup Data Directory
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	
	// Check for .pointer file in default location
	defaultBase := filepath.Join(home, ".mochibox")
	pointerPath := filepath.Join(defaultBase, ".pointer")
	
	dataDir := defaultBase
	
	if content, err := os.ReadFile(pointerPath); err == nil {
		customPath := strings.TrimSpace(string(content))
		if customPath != "" {
			dataDir = customPath
			log.Printf("Using custom data directory: %s", dataDir)
		}
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Fatal(err)
	}

	// Setup File Logging
	logFile, err := os.OpenFile(filepath.Join(dataDir, "backend.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(io.MultiWriter(os.Stdout, logFile))
	} else {
		log.Printf("Failed to open log file: %v", err)
	}
	
	log.Println("----------------------------------------")
	log.Println("MochiBox Backend Starting...")
	log.Printf("Data Directory: %s", dataDir)

	// 2. Initialize Database
	database, err := db.InitDB(filepath.Join(dataDir, "mochibox.db"))
	if err != nil {
		log.Fatalf("Failed to initialize db: %v", err)
	}

	var settings db.Settings
	database.First(&settings)

	// Account Manager
	accountMgr := core.NewAccountManager(database, dataDir)
	
	// Try Auto-Unlock
	if lockData, err := crypto.LoadAuthLock(dataDir); err == nil {
		log.Println("Found auto-unlock data, attempting login...")
		if err := accountMgr.Unlock(string(lockData)); err == nil {
			log.Println("Auto-login successful")
		} else {
			log.Printf("Auto-login failed: %v", err)
		}
	}

	// 3. Managed IPFS Node
	var ipfsMgr *core.IpfsManager
	ipfsMgr, err = core.NewIpfsManager(dataDir)
	if err != nil {
		log.Printf("Failed to create IPFS manager: %v", err)
	} else if settings.UseEmbeddedNode {
		if err := ipfsMgr.InitRepo(); err != nil {
			log.Printf("Failed to init repo: %v", err)
		} else {
			go func() {
				log.Println("Starting managed IPFS daemon...")
				if err := ipfsMgr.Start(context.Background()); err != nil {
					log.Printf("IPFS Daemon failed: %v", err)
				}
			}()
		}
	}

	// 4. Initialize IPFS Client Node
	node, err := core.NewNode(dataDir, settings.IpfsApiUrl)
	if err != nil {
		log.Fatalf("Failed to initialize node: %v", err)
	}

	// Monitor Manager and update Node
	if ipfsMgr != nil {
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				// Refresh settings from DB to handle concurrent updates from API
				var currentSettings db.Settings
				if err := database.First(&currentSettings).Error; err != nil {
					continue
				}

				// If manager has an address and it's different from what we thought
				if ipfsMgr.ApiAddr != "" {
					// We only update if it's different from current connected one?
					// Or just if it's different from DB settings?
					// Note: node.UpdateApiUrl will verify connection.
					
					// If settings is empty or different
					if currentSettings.IpfsApiUrl != ipfsMgr.ApiAddr {
						log.Printf("Managed Node ready at %s. Updating client...", ipfsMgr.ApiAddr)
						
						if err := node.UpdateApiUrl(ipfsMgr.ApiAddr); err == nil {
							// Update DB
							currentSettings.IpfsApiUrl = ipfsMgr.ApiAddr
							database.Save(&currentSettings)
						} else {
							log.Printf("Failed to connect to managed node: %v", err)
						}
					}

					// Update Gateway
					gwAddr := ipfsMgr.GetGatewayAddr()
					if gwAddr != "" {
						httpGw := core.MultiaddrToHttp(gwAddr)
						if httpGw != "" && currentSettings.IpfsGatewayUrl != httpGw {
							log.Printf("Managed Gateway ready at %s (%s)", httpGw, gwAddr)
							currentSettings.IpfsGatewayUrl = httpGw
							database.Save(&currentSettings)
						}
					}
				}
			}
		}()
	}

	// 5. Start API Server
	port := os.Getenv("MOCHIBOX_PORT")
	if port == "" {
		port = "3666"
	}

	server := api.NewServer(node, database, ipfsMgr, accountMgr)

	// Watch Stdin for shutdown signal (watchdog)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			if scanner.Text() == "shutdown" {
				if server.ShutdownChan != nil {
					server.ShutdownChan <- true
					return
				}
			}
		}
		// If Stdin closed (parent died), trigger shutdown
		if server.ShutdownChan != nil {
			server.ShutdownChan <- true
		}
	}()
	
	// Handle graceful shutdown
	go func() {
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		
		select {
		case <-c:
			log.Println("Received signal, shutting down...")
		case <-server.ShutdownChan:
			log.Println("Received shutdown request (API/Stdin)...")
		}

		if ipfsMgr != nil {
			ipfsMgr.Stop()
		}
		os.Exit(0)
	}()

	log.Printf("MochiBox Core running on port %s", port)
	if err := server.Run(port); err != nil {
		log.Fatal(err)
	}
}
