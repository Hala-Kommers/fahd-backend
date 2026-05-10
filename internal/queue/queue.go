package queue

type Message struct {
	SessionID string `json:"sessionId"`
	Content   string `json:"content"`
}

type Queue interface {
	Enqueue(msg Message) error
	Dequeue() (<-chan Message, error)
	Stop()
}
