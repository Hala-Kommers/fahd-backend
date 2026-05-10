package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 10000
	sendBufSize    = 100
)

type Client struct {
	hub       *Hub
	conn      *websocket.Conn
	sessionID string
	send      chan ServerMessage
	once      sync.Once
}

func NewClient(hub *Hub, conn *websocket.Conn) *Client {
	return &Client{
		hub:  hub,
		conn: conn,
		send: make(chan ServerMessage, sendBufSize),
	}
}

func (c *Client) SetSessionID(sessionID string) {
	c.sessionID = sessionID
}

func (c *Client) SessionID() string {
	return c.sessionID
}

func (c *Client) Send(msg ServerMessage) {
	select {
	case c.send <- msg:
	default:
		slog.Warn("ws client send buffer full, dropping message", "session_id", c.sessionID)
	}
}

func (c *Client) ReadPump(ctx context.Context, handler func(ClientMessage)) {
	defer func() {
		c.conn.Close()
		if c.sessionID != "" {
			c.hub.Unregister(c.sessionID)
		}
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		select {
		case <-ctx.Done():
			return
		default:
			_, raw, err := c.conn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
					slog.Debug("ws read error", "session_id", c.sessionID, "error", err)
				}
				return
			}
			var msg ClientMessage
			if err := json.Unmarshal(raw, &msg); err != nil {
				slog.Warn("ws invalid message", "session_id", c.sessionID, "error", err)
				c.Send(ServerMessage{Type: "error", Error: "invalid message format"})
				continue
			}
			handler(msg)
		}
	}
}

func (c *Client) WritePump(ctx context.Context) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			data, err := json.Marshal(msg)
			if err != nil {
				slog.Error("ws marshal error", "session_id", c.sessionID, "error", err)
				continue
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				slog.Debug("ws write error", "session_id", c.sessionID, "error", err)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
