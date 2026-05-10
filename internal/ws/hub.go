package ws

import (
	"log/slog"
	"sync"
)

type Hub struct {
	mu       sync.RWMutex
	clients  map[string]*Client
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]*Client),
	}
}

func (h *Hub) Register(sessionID string, client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if existing, ok := h.clients[sessionID]; ok {
		close(existing.send)
	}
	h.clients[sessionID] = client
	slog.Debug("ws client registered", "session_id", sessionID)
}

func (h *Hub) Unregister(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	delete(h.clients, sessionID)
	slog.Debug("ws client unregistered", "session_id", sessionID)
}

func (h *Hub) SendToSession(sessionID string, msg ServerMessage) {
	h.mu.RLock()
	client, ok := h.clients[sessionID]
	h.mu.RUnlock()
	if !ok {
		return
	}
	client.Send(msg)
}

func (h *Hub) BroadcastToSession(sessionID string, msg ServerMessage) {
	h.SendToSession(sessionID, msg)
}
