package tetris

import (
	"sync"
	"time"
)

// MockTicker is a mock implementation of the ticker interface.
type MockTicker struct {
	ch          chan time.Time
	stop, reset bool
	mu          sync.Mutex
}

func newMockTicker() *MockTicker          { return &MockTicker{ch: make(chan time.Time)} }
func (m *MockTicker) C() <-chan time.Time { return m.ch }
func (m *MockTicker) Stop()               { m.stop = true }
func (m *MockTicker) Tick()               { m.ch <- time.Now() }
func (m *MockTicker) Reset(time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reset = true
}
func (m *MockTicker) IsReset() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reset
}
func (m *MockTicker) IsStop() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stop
}

// NewTestGame creates a game with a specific TestTetris and returns a game and a manual ticker.
func NewTestGame(t *Tetris) (*Game, *MockTicker) {
	ticker := newMockTicker()
	return &Game{
		updateCh: make(chan *Tetris),
		actionCh: make(chan Action),
		tetris:   t,
		ticker:   ticker,
	}, ticker
}

// NewTestTetris creates a new Tetris struct with a test tetromino.
func NewTestTetris(shape Shape) *Tetris {
	t := newTetris()
	t.Tetromino = shapeMap[shape]()
	t.NexTetromino = shapeMap[shape]()
	t.Tetromino.GhostY = t.Tetromino.Y + t.dropDownDelta()
	return t
}
