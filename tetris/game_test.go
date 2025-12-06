package tetris_test

import (
	"testing"
	"time"

	"github.com/alwedo/tetris/tetris"
)

func TestGetUpdate(t *testing.T) {
	te := tetris.NewTestTetris(tetris.J)
	game, ticker := tetris.NewTestGame(te)
	doneCh := make(chan struct{})
	wantY := te.Tetromino.Y

	go func() {
		for {
			select {
			case u := <-game.GetUpdate():
				if u.Tetromino.Y != wantY {
					t.Errorf("Expected tetromino Y to be %d, got %d", wantY, u.Tetromino.Y)
				}
				wantY--
			case <-time.After(1 * time.Second):
				t.Error("Timed out waiting for update signal")
				close(doneCh)
			case <-doneCh:
				return
			}
		}
	}()
	game.Start()
	time.Sleep(10 * time.Millisecond)
	ticker.Tick()
	time.Sleep(10 * time.Millisecond)
	doneCh <- struct{}{}
}

func TestStartStop(t *testing.T) {
	te := tetris.NewTestTetris(tetris.J)
	game, ticker := tetris.NewTestGame(te)
	go func() {
		for range game.GetUpdate() { // nolint:revive
		}
	}()
	game.Start()
	time.Sleep(50 * time.Millisecond)
	if !ticker.IsReset() {
		t.Errorf("Expected ticker to be reset")
	}
	game.Stop()
	if !ticker.IsStop() {
		t.Errorf("Expected ticker to be stopped")
	}
	if !te.GameOver {
		t.Errorf("Expected game to be over")
	}
}
