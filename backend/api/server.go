package api

import (
	"mochibox-core/core"
	"sync"
	// "mochibox-core/db" // Used in handle functions but maybe not directly in NewServer signature if we pass *gorm.DB

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Server struct {
	Node        *core.MochiNode
	Router      *gin.Engine
	DB          *gorm.DB
	IpfsManager *core.IpfsManager
	AccountManager *core.AccountManager
	ShutdownChan chan bool
	
	// Network Boost State
	BoostMutex   sync.Mutex
	IsBoosting   bool
}

func NewServer(node *core.MochiNode, database *gorm.DB, ipfsMgr *core.IpfsManager, accMgr *core.AccountManager) *Server {
	r := gin.Default()
	// Trust only local proxies to avoid security warning
	_ = r.SetTrustedProxies([]string{"127.0.0.1"})
	
	// CORS for Electron
	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	s := &Server{
		Node:        node,
		Router:      r,
		DB:          database,
		IpfsManager: ipfsMgr,
		AccountManager: accMgr,
		ShutdownChan: make(chan bool),
	}
	s.RegisterRoutes()
	return s
}

func (s *Server) RegisterRoutes() {
	s.Router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "MochiBox Core"})
	})

	api := s.Router.Group("/api")
	{
		// api.GET("/files", s.handleListFiles) // Moved to registerFileRoutes
        s.registerGatewayRoutes(api)
		s.registerConfigRoutes(api)
		s.registerSystemRoutes(api)
		s.registerSharedRoutes(api)
		s.registerAccountRoutes(api)
	}
    
    s.registerFileRoutes(s.DB)
}

func (s *Server) Run(port string) error {
	return s.Router.Run(":" + port)
}
