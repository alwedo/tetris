package tetris

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/alwedo/tetris/tetrisv2/tetristest"
)

func TestStart(t *testing.T) {
	t.Run("ticker should call MoveDown(), decreasing the tetromino's Y position by 1 and resetting the ticker", func(t *testing.T) {
		mockTicker := tetristest.NewMockTicker()
		game := Start(context.Background(), WithCustomTicker(mockTicker))
		wantTetrominoPos := game.tetris.Tetromino.Y - 1

		var wg sync.WaitGroup
		wg.Go(func() {
			msg := <-game.UpdateCh
			if msg.Tetromino.Y != wantTetrominoPos {
				t.Errorf("wanted tetromino Y Position to be %d, got %d", wantTetrominoPos, msg.Tetromino.Y)
			}
			if mockTicker.ResetCount.Load() != 1 {
				t.Errorf("wanted ticker resetCount to be 1, got %d", mockTicker.ResetCount.Load())
			}
		})

		mockTicker.Tick()
		wg.Wait()
	})

	t.Run("action that triggers a new round stops and resets the ticker", func(t *testing.T) {
		mockTicker := tetristest.NewMockTicker()
		game := Start(context.Background(), WithCustomTicker(mockTicker))

		var wg sync.WaitGroup
		wg.Go(func() {
			<-game.UpdateCh
			if mockTicker.ResetCount.Load() != 1 {
				t.Errorf("wanted ticker resetCount to be 1, got %d", mockTicker.ResetCount.Load())
			}
			if mockTicker.StopCount.Load() != 1 {
				t.Errorf("wanted ticker stopCount to be 1, got %d", mockTicker.StopCount.Load())
			}
		})

		game.Do(DropDown())
		wg.Wait()
	})

	t.Run("new round action with cleared lines trigger animation delay", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		game := Start(ctx,
			WithCustomTicker(tetristest.NewMockTicker()),
			WithCustomStack(map[int][]Shape{
				0: {I, I, I, "", "", "", "", I, I, I},
			}),
			WithCustomShape(I),
		)
		var gotUpdates []time.Time
		var gotClearedLinesIndexes []int
		var gotClearedLines int

		var wg sync.WaitGroup
		wg.Go(func() {
			for {
				msg, ok := <-game.UpdateCh
				if !ok {
					return
				}
				if len(msg.LinesClearedIndex) > 0 {
					gotClearedLinesIndexes = msg.LinesClearedIndex
				}
				gotClearedLines = msg.Lines
				gotUpdates = append(gotUpdates, time.Now())
			}
		})

		game.Do(DropDown())
		cancel()
		wg.Wait()

		wantClearedLinesIndex := []int{0}
		if !reflect.DeepEqual(wantClearedLinesIndex, gotClearedLinesIndexes) {
			t.Errorf("wanted cleared lines index %v, got %v", wantClearedLinesIndex, gotClearedLinesIndexes)
		}
		if gotClearedLines != 1 {
			t.Errorf("wanted 1 cleared line, got %d", gotClearedLines)
		}
		if len(gotUpdates) < 2 {
			t.Fatalf("expected at least 2 updates, got %v", gotUpdates)
		}

		gotDelay := gotUpdates[1].Sub(gotUpdates[0])
		if gotDelay <= animationDelay-2*time.Millisecond || gotDelay >= animationDelay+2*time.Millisecond {
			t.Errorf("wanted duration between updates to be %d ±2ms, got %d", animationDelay.Milliseconds(), gotDelay.Milliseconds())
		}
	})

	t.Run("channel close on game over", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			game := Start(t.Context())
			var wg = sync.WaitGroup{}
			var wantGameOver bool
			wg.Go(func() {
				for {
					_, ok := <-game.UpdateCh
					if !ok {
						wantGameOver = true
						return
					}
				}
			})
			wg.Wait()
			if !wantGameOver {
				t.Errorf("wanted gameOver to be true but is %v", wantGameOver)
			}
		})
	})
}
