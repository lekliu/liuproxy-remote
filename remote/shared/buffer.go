package shared

import (
	"bytes"
	"sync"
)

type ThreadSafeBuffer struct {
	b  bytes.Buffer
	mu sync.Mutex
}

func NewThreadSafeBuffer() *ThreadSafeBuffer { return &ThreadSafeBuffer{} }
func (b *ThreadSafeBuffer) Read(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Read(p)
}
func (b *ThreadSafeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Write(p)
}
func (b *ThreadSafeBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.b.Len()
}
