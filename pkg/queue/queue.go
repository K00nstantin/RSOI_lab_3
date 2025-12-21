package queue

import (
	"sync"
	"time"
)

type RetryRequest struct {
	ID         string
	Method     string
	URL        string
	Headers    map[string]string
	Body       []byte
	RetryAt    time.Time
	RetryCount int
	MaxRetries int
}

type Queue struct {
	items []*RetryRequest
	mu    sync.Mutex
}

func NewQueue() *Queue {
	return &Queue{
		items: make([]*RetryRequest, 0),
	}
}

func (q *Queue) Enqueue(req *RetryRequest) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = append(q.items, req)
}

func (q *Queue) Dequeue() *RetryRequest {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	for i, req := range q.items {
		if req.RetryAt.Before(now) || req.RetryAt.Equal(now) {
			q.items = append(q.items[:i], q.items[i+1:]...)
			return req
		}
	}
	return nil
}

func (q *Queue) Peek() *RetryRequest {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	for _, req := range q.items {
		if req.RetryAt.Before(now) || req.RetryAt.Equal(now) {
			return req
		}
	}
	return nil
}

func (q *Queue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}

func (q *Queue) GetAll() []*RetryRequest {
	q.mu.Lock()
	defer q.mu.Unlock()
	result := make([]*RetryRequest, len(q.items))
	copy(result, q.items)
	return result
}
