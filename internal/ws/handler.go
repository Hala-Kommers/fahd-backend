package ws

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"fahd-backend/internal/session"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ChatHandler struct {
	hub           *Hub
	sessionMgr    *session.Manager
	msgHandler    func(sessionID string, content string)
	rateLimiter   func(sessionID string) bool
}

func NewChatHandler(hub *Hub, sessionMgr *session.Manager, msgHandler func(string, string)) *ChatHandler {
	return &ChatHandler{
		hub:        hub,
		sessionMgr: sessionMgr,
		msgHandler: msgHandler,
		rateLimiter: func(sessionID string) bool {
			return true
		},
	}
}

func (h *ChatHandler) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("ws upgrade failed", "error", err)
		return
	}

	client := NewClient(h.hub, conn)
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	go client.WritePump(ctx)

	client.ReadPump(ctx, func(msg ClientMessage) {
		switch msg.Type {
		case "init":
			sess, err := h.sessionMgr.Create()
			if err != nil {
				slog.Error("ws session create failed", "error", err)
				client.Send(ServerMessage{Type: "error", Error: "failed to create session"})
				return
			}
			client.SetSessionID(sess.ID)
			h.hub.Register(sess.ID, client)
			client.Send(ServerMessage{
				Type:      "session_created",
				SessionID: sess.ID,
				Token:     sess.Token,
				ExpiresAt: sess.TokenExpiry,
			})
			slog.Info("ws session created", "session_id", sess.ID)

		case "auth":
			sid := strings.TrimSpace(msg.SessionID)
			token := strings.TrimSpace(msg.Token)
			if sid == "" || token == "" {
				client.Send(ServerMessage{Type: "auth_error", Error: "session_id and token are required"})
				return
			}
			sess, err := h.sessionMgr.Authenticate(sid, token)
			if err != nil {
				client.Send(ServerMessage{Type: "auth_error", Error: err.Error()})
				return
			}
			client.SetSessionID(sess.ID)
			h.hub.Register(sess.ID, client)
			client.Send(ServerMessage{Type: "auth_ok", SessionID: sess.ID})
			slog.Info("ws session restored", "session_id", sess.ID)

		case "message":
			content := strings.TrimSpace(msg.Content)
			if content == "" {
				client.Send(ServerMessage{Type: "error", Error: "message content is required"})
				return
			}
			sid := client.SessionID()
			if sid == "" {
				client.Send(ServerMessage{Type: "error", Error: "session not initialized. send init first"})
				return
			}
			if h.rateLimiter != nil && !h.rateLimiter(sid) {
				client.Send(ServerMessage{Type: "error", Error: "rate limit exceeded"})
				return
			}
			client.Send(ServerMessage{Type: "message_received"})
			if h.msgHandler != nil {
				go h.msgHandler(sid, content)
			}

		case "ping":
			client.Send(ServerMessage{Type: "pong"})

		default:
			client.Send(ServerMessage{Type: "error", Error: "unknown message type: " + msg.Type})
		}
	})

	slog.Debug("ws connection closed", "session_id", client.SessionID())
	time.Sleep(100 * time.Millisecond)
}
