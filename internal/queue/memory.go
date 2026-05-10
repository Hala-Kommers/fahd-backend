package queue

import "fmt"

type MemoryQueue struct {
	ch      chan Message
	stopped bool
}

func NewMemoryQueue(bufferSize int) *MemoryQueue {
	return &MemoryQueue{
		ch: make(chan Message, bufferSize),
	}
}

func (q *MemoryQueue) Enqueue(msg Message) error {
	if q.stopped {
		return fmt.Errorf("queue is stopped")
	}
	select {
	case q.ch <- msg:
		return nil
	default:
		return fmt.Errorf("queue full")
	}
}

func (q *MemoryQueue) Dequeue() (<-chan Message, error) {
	if q.stopped {
		return nil, fmt.Errorf("queue is stopped")
	}
	return q.ch, nil
}

func (q *MemoryQueue) Stop() {
	q.stopped = true
	close(q.ch)
}
