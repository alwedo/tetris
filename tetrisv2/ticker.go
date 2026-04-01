package tetris

import (
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
