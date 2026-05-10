package worker

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"fahd-backend/internal/ai/agent"
	"fahd-backend/internal/config"
	"fahd-backend/internal/queue"
	"fahd-backend/internal/session"
	"fahd-backend/internal/ws"

	"gorm.io/gorm"
)

type Pool struct {
	numWorkers int
	queue      queue.Queue
	hub        *ws.Hub
	agentSvc   *agent.Service
	sessionMgr *session.Manager
}

func NewPool(numWorkers int, q queue.Queue, hub *ws.Hub, db *gorm.DB, cfg config.Config, sessionMgr *session.Manager) *Pool {
	return &Pool{
		numWorkers: numWorkers,
		queue:      q,
		hub:        hub,
		agentSvc:   agent.NewService(db, cfg),
		sessionMgr: sessionMgr,
	}
}

func (p *Pool) Start(ctx context.Context) {
	ch, err := p.queue.Dequeue()
	if err != nil {
		slog.Error("worker pool dequeue failed", "error", err)
		return
	}
	for i := 0; i < p.numWorkers; i++ {
		go p.worker(ctx, i, ch)
	}
	slog.Info("worker pool started", "count", p.numWorkers)
}

func (p *Pool) worker(ctx context.Context, id int, ch <-chan queue.Message) {
	for {
		select {
		case <-ctx.Done():
			slog.Info("worker shutting down", "worker_id", id)
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			p.process(ctx, id, msg)
		}
	}
}

func (p *Pool) process(ctx context.Context, workerID int, msg queue.Message) {
	sessionID := msg.SessionID
	slog.Debug("worker processing message", "worker_id", workerID, "session_id", sessionID)

	p.hub.SendToSession(sessionID, ws.ServerMessage{Type: "ai_typing"})

	conversationID := int64(0)
	if sess := p.sessionMgr.Get(sessionID); sess != nil {
		conversationID = sess.ConversationID
	}

	resp, err := p.agentSvc.HandleMessage(ctx, agent.MessageRequest{
		ConversationID: conversationID,
		Message:        msg.Content,
		Context:        msg.Context,
	})
	if err != nil {
		slog.Error("worker agent error", "worker_id", workerID, "session_id", sessionID, "error", err)
		p.hub.SendToSession(sessionID, ws.ServerMessage{
			Type:  "ai_error",
			Error: "I'm having trouble processing your request. Please try again.",
		})
		return
	}

	if conversationID == 0 && resp.ConversationID > 0 {
		p.sessionMgr.SetConversationID(sessionID, resp.ConversationID)
	}

	p.streamResponse(sessionID, resp.Reply)

	actions := make([]any, len(resp.Actions))
	for i, a := range resp.Actions {
		actions[i] = a
	}

	p.hub.SendToSession(sessionID, ws.ServerMessage{
		Type:    "ai_done",
		Actions: actions,
		Meta:    resp.Meta,
	})
}

func (p *Pool) streamResponse(sessionID, reply string) {
	reply = strings.TrimSpace(reply)
	if reply == "" {
		return
	}

	words := strings.Fields(reply)
	if len(words) == 0 {
		return
	}

	buf := strings.Builder{}
	for _, word := range words {
		if buf.Len() > 0 {
			buf.WriteString(" ")
		}
		buf.WriteString(word)
		p.hub.SendToSession(sessionID, ws.ServerMessage{
			Type:    "ai_chunk",
			Content: buf.String(),
		})
		time.Sleep(15 * time.Millisecond)
	}
}
