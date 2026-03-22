package tetris

import (
	"sync/atomic"
	"time"
)

type Ticker interface {
	C() <-chan time.Time
	Reset(time.Duration)
	Stop()
}

// timeTicker is a time.Ticker wrapped in the Ticker interface.
type timeTicker struct {
	ticker *time.Ticker
}

func newTimeTicker(t time.Duration) *timeTicker {
	return &timeTicker{ticker: time.NewTicker(t)}
}

func (t *timeTicker) C() <-chan time.Time   { return t.ticker.C }
func (t *timeTicker) Stop()                 { t.ticker.Stop() }
func (t *timeTicker) Reset(d time.Duration) { t.ticker.Reset(d) }

// MockTicker is a mock implementation of the ticker interface.
type MockTicker struct {
	ch         chan time.Time
	resetCount atomic.Int32
	stopCount  atomic.Int32
}

func NewMockTicker() *MockTicker          { return &MockTicker{ch: make(chan time.Time)} }
func (m *MockTicker) C() <-chan time.Time { return m.ch }
func (m *MockTicker) Tick()               { m.ch <- time.Now() }
func (m *MockTicker) Stop()               { m.stopCount.Add(1) }
func (m *MockTicker) Reset(time.Duration) { m.resetCount.Add(1) }
