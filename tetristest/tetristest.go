package tetristest

import (
	"sync/atomic"
	"time"
)

// MockTicker is a mock implementation of the ticker interface that
// allows manual control over the ticks for testing.
type MockTicker struct {
	ch                chan time.Time
	StopCount         atomic.Int32
	ResetCount        atomic.Int32
	LastResetDuration atomic.Int64
}

func NewMockTicker() *MockTicker          { return &MockTicker{ch: make(chan time.Time)} }
func (m *MockTicker) C() <-chan time.Time { return m.ch }
func (m *MockTicker) Tick()               { m.ch <- time.Now() }
func (m *MockTicker) Stop()               { m.StopCount.Add(1) }
func (m *MockTicker) Reset(t time.Duration) {
	m.ResetCount.Add(1)
	m.LastResetDuration.Store(int64(t))
}
