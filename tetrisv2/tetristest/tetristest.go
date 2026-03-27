package tetristest

import (
	"sync/atomic"
	"time"
)

// MockTicker is a mock implementation of the ticker interface that
// allows manual control over the ticks for testing.
type MockTicker struct {
	ch         chan time.Time
	ResetCount atomic.Int32
	StopCount  atomic.Int32
}

func NewMockTicker() *MockTicker          { return &MockTicker{ch: make(chan time.Time)} }
func (m *MockTicker) C() <-chan time.Time { return m.ch }
func (m *MockTicker) Tick()               { m.ch <- time.Now() }
func (m *MockTicker) Stop()               { m.StopCount.Add(1) }
func (m *MockTicker) Reset(time.Duration) { m.ResetCount.Add(1) }
