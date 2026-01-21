package api

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type WSClient struct {
	conn          *websocket.Conn
	send          chan []byte
	subscriptions map[string]bool
	mu            sync.Mutex
}

type WSHub struct {
	clients    map[*WSClient]bool
	register   chan *WSClient
	unregister chan *WSClient
	broadcast  chan *WSMessage
	mu         sync.RWMutex
}

type WSMessage struct {
	TaskID string      `json:"task_id"`
	Type   string      `json:"type"`
	Data   interface{} `json:"data"`
}

func NewWSHub() *WSHub {
	return &WSHub{
		clients:    make(map[*WSClient]bool),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		broadcast:  make(chan *WSMessage, 256),
	}
}

func (h *WSHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				client.mu.Lock()
				if message.TaskID == "" || client.subscriptions[message.TaskID] {
					select {
					case client.send <- encodeMessage(message):
					default:
						close(client.send)
						delete(h.clients, client)
					}
				}
				client.mu.Unlock()
			}
			h.mu.RUnlock()
		}
	}
}

func (h *WSHub) BroadcastTaskUpdate(taskID string, data interface{}) {
	h.broadcast <- &WSMessage{
		TaskID: taskID,
		Type:   "task_update",
		Data:   data,
	}
}

func encodeMessage(msg *WSMessage) []byte {
	data, _ := json.Marshal(msg)
	return data
}

func (s *Server) handleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	client := &WSClient{
		conn:          conn,
		send:          make(chan []byte, 256),
		subscriptions: make(map[string]bool),
	}

	s.WSHub.register <- client

	go client.writePump()
	go client.readPump(s.WSHub)
}

func (c *WSClient) readPump(hub *WSHub) {
	defer func() {
		hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		var msg struct {
			Action string `json:"action"`
			TaskID string `json:"task_id"`
		}

		err := c.conn.ReadJSON(&msg)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		c.mu.Lock()
		switch msg.Action {
		case "subscribe":
			c.subscriptions[msg.TaskID] = true
		case "unsubscribe":
			delete(c.subscriptions, msg.TaskID)
		}
		c.mu.Unlock()
	}
}

func (c *WSClient) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (s *Server) registerWebSocketRoutes(api *gin.RouterGroup) {
	api.GET("/ws/tasks", s.handleWebSocket)
}
