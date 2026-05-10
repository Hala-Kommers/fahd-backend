package queue

type Message struct {
	SessionID string         `json:"sessionId"`
	Content   string         `json:"content"`
	Context   map[string]any `json:"context,omitempty"`
}

type Queue interface {
	Enqueue(msg Message) error
	Dequeue() (<-chan Message, error)
	Stop()
}
