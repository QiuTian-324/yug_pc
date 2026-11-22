package realtime

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 25 * time.Second
	readLimit  = 32 * 1024
	sendBuffer = 64
)

type Hub struct {
	mu      sync.RWMutex
	clients map[uint]map[*Client]struct{}
}

type Client struct {
	UserID uint
	conn   *websocket.Conn
	hub    *Hub
	send   chan []byte
	done   chan struct{}
	once   sync.Once
}

func NewHub() *Hub {
	return &Hub{clients: make(map[uint]map[*Client]struct{})}
}

func (h *Hub) Register(userID uint, conn *websocket.Conn) *Client {
	client := &Client{
		UserID: userID,
		conn:   conn,
		hub:    h,
		send:   make(chan []byte, sendBuffer),
		done:   make(chan struct{}),
	}

	h.mu.Lock()
	if h.clients[userID] == nil {
		h.clients[userID] = make(map[*Client]struct{})
	}
	h.clients[userID][client] = struct{}{}
	h.mu.Unlock()
	return client
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	if clients, ok := h.clients[client.UserID]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.clients, client.UserID)
		}
	}
	h.mu.Unlock()
	client.close()
}

func (h *Hub) SendToUser(userID uint, except *Client, event any) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	h.mu.RLock()
	clients := make([]*Client, 0, len(h.clients[userID]))
	for client := range h.clients[userID] {
		if client != except {
			clients = append(clients, client)
		}
	}
	h.mu.RUnlock()

	for _, client := range clients {
		client.enqueue(payload)
	}
	return nil
}

func (c *Client) SendJSON(event any) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if !c.enqueue(payload) {
		return websocket.ErrCloseSent
	}
	return nil
}

func (c *Client) ReadPump(handleMessage func([]byte)) {
	defer c.hub.Unregister(c)

	c.conn.SetReadLimit(readLimit)
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return
	}
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		messageType, message, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		if messageType != websocket.TextMessage {
			_ = c.conn.WriteControl(
				websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseUnsupportedData, "text frames only"),
				time.Now().Add(writeWait),
			)
			return
		}
		handleMessage(message)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	defer c.hub.Unregister(c)

	for {
		select {
		case <-c.done:
			return
		case message := <-c.send:
			if err := c.conn.SetWriteDeadline(time.Now().Add(writeWait)); err != nil {
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			if err := c.conn.WriteControl(
				websocket.PingMessage,
				nil,
				time.Now().Add(writeWait),
			); err != nil {
				return
			}
		}
	}
}

func (c *Client) enqueue(payload []byte) bool {
	select {
	case <-c.done:
		return false
	default:
	}
	select {
	case c.send <- payload:
		return true
	default:
		c.hub.Unregister(c)
		return false
	}
}

func (c *Client) close() {
	c.once.Do(func() {
		close(c.done)
		_ = c.conn.Close()
	})
}
