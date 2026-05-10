package session

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID             string    `json:"id"`
	Token          string    `json:"token"`
	TokenExpiry    int64     `json:"tokenExpiry"`
	CreatedAt      time.Time `json:"createdAt"`
	LastUsedAt     time.Time `json:"lastUsedAt"`
	ConversationID int64     `json:"conversationId"`
	MessageIDs     []int64   `json:"-"`
}

type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	tokens   *TokenService
	ttl      time.Duration
}

func NewManager(secret string, ttl time.Duration) *Manager {
	return &Manager{
		sessions: make(map[string]*Session),
		tokens:   NewTokenService(secret, ttl),
		ttl:      ttl,
	}
}

func (m *Manager) Create() (*Session, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return nil, fmt.Errorf("generate uuid: %w", err)
	}
	sessionID := id.String()
	token, expiry, err := m.tokens.Generate(sessionID)
	if err != nil {
		return nil, err
	}
	sess := &Session{
		ID:          sessionID,
		Token:       token,
		TokenExpiry: expiry,
		CreatedAt:   time.Now(),
		LastUsedAt:  time.Now(),
	}
	m.mu.Lock()
	m.sessions[sessionID] = sess
	m.mu.Unlock()
	return sess, nil
}

func (m *Manager) Authenticate(sessionID, token string) (*Session, error) {
	claims, err := m.tokens.Validate(token)
	if err != nil {
		return nil, fmt.Errorf("token validation failed")
	}
	if claims.SessionID != sessionID {
		return nil, fmt.Errorf("token session mismatch")
	}
	m.mu.RLock()
	sess, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	m.mu.Lock()
	sess.LastUsedAt = time.Now()
	m.mu.Unlock()
	return sess, nil
}

func (m *Manager) Get(sessionID string) *Session {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[sessionID]
}

func (m *Manager) SetConversationID(sessionID string, conversationID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[sessionID]; ok {
		s.ConversationID = conversationID
	}
}

func (m *Manager) AddMessageID(sessionID string, messageID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if s, ok := m.sessions[sessionID]; ok {
		s.MessageIDs = append(s.MessageIDs, messageID)
	}
}

func (m *Manager) Cleanup(maxAge time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now()
	for id, s := range m.sessions {
		if now.Sub(s.LastUsedAt) > maxAge {
			delete(m.sessions, id)
		}
	}
}
