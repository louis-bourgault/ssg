package dev

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait = 10 * time.Second
	pongWait  = 60 * time.Second
	pingEvery = 25 * time.Second
)

// ReloadMessage is the JSON protocol shared with the browser client.
type ReloadMessage struct {
	Type    string `json:"type"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message,omitempty"`
}

// ReloadHub fans build events out to every connected browser without allowing
// a slow connection to block the builder or other clients.
type ReloadHub struct {
	mu      sync.Mutex
	clients map[*reloadClient]struct{}
	closed  bool
	wg      sync.WaitGroup
}

type reloadClient struct {
	hub  *ReloadHub
	conn *websocket.Conn
	send chan []byte
}

func NewReloadHub() *ReloadHub {
	return &ReloadHub{clients: make(map[*reloadClient]struct{})}
}

func (h *ReloadHub) ServeHTTP(w http.ResponseWriter, r *http.Request, currentError string) {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin:     sameOrigin,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := &reloadClient{hub: h, conn: conn, send: make(chan []byte, 16)}
	ready, _ := json.Marshal(ReloadMessage{Type: "ready"})
	client.send <- ready
	if currentError != "" {
		message, _ := json.Marshal(ReloadMessage{Type: "error", Message: currentError})
		client.send <- message
	}

	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		_ = conn.Close()
		return
	}
	h.clients[client] = struct{}{}
	h.wg.Add(2)
	h.mu.Unlock()
	go client.readPump()
	go client.writePump()
}

func (h *ReloadHub) Broadcast(message ReloadMessage) {
	encoded, err := json.Marshal(message)
	if err != nil {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	for client := range h.clients {
		select {
		case client.send <- encoded:
		default:
			delete(h.clients, client)
			close(client.send)
			_ = client.conn.Close()
		}
	}
}

func (h *ReloadHub) remove(client *reloadClient) {
	h.mu.Lock()
	if _, exists := h.clients[client]; exists {
		delete(h.clients, client)
		close(client.send)
		_ = client.conn.Close()
	}
	h.mu.Unlock()
}

func (h *ReloadHub) Close() {
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	h.closed = true
	for client := range h.clients {
		delete(h.clients, client)
		close(client.send)
		_ = client.conn.Close()
	}
	h.mu.Unlock()
	h.wg.Wait()
}

func (c *reloadClient) readPump() {
	defer c.hub.wg.Done()
	defer c.hub.remove(c)
	c.conn.SetReadLimit(1024)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})
	for {
		if _, _, err := c.conn.NextReader(); err != nil {
			return
		}
	}
}

func (c *reloadClient) writePump() {
	defer c.hub.wg.Done()
	defer c.hub.remove(c)
	ticker := time.NewTicker(pingEvery)
	defer ticker.Stop()
	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	return strings.EqualFold(parsed.Host, r.Host)
}
