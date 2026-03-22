package tetris

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"
)

func TestStart(t *testing.T) {
	t.Run("ticker should call MoveDown(), decreasing the tetromino's Y position by 1 and resetting the ticker", func(t *testing.T) {
		mockTicker := NewMockTicker()
		game := Start(context.Background(), WithCustomTicker(mockTicker))
		wantTetrominoPos := game.tetris.Tetromino.Y - 1

		var wg sync.WaitGroup
		wg.Go(func() {
			msg := <-game.UpdateCh
			if msg.Tetromino.Y != wantTetrominoPos {
				t.Errorf("wanted tetromino Y Position to be %d, got %d", wantTetrominoPos, msg.Tetromino.Y)
			}
			if mockTicker.resetCount.Load() != 1 {
				t.Errorf("wanted ticker resetCount to be 1, got %d", mockTicker.resetCount.Load())
			}
		})

		mockTicker.Tick()
		wg.Wait()
	})

	t.Run("action that triggers a new round stops and resets the ticker", func(t *testing.T) {
		mockTicker := NewMockTicker()
		game := Start(context.Background(), WithCustomTicker(mockTicker))

		var wg sync.WaitGroup
		wg.Go(func() {
			<-game.UpdateCh
			if mockTicker.resetCount.Load() != 1 {
				t.Errorf("wanted ticker resetCount to be 1, got %d", mockTicker.resetCount.Load())
			}
			if mockTicker.stopCount.Load() != 1 {
				t.Errorf("wanted ticker stopCount to be 1, got %d", mockTicker.stopCount.Load())
			}
		})

		game.Do(DropDown())
		wg.Wait()
	})

	t.Run("new round action with cleared lines trigger animation delay", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		game := Start(ctx,
			WithCustomTicker(NewMockTicker()),
			WithCustomStack(map[int][]Shape{
				0: {I, I, I, "", "", "", "", I, I, I},
			}),
			WithCustomSape(I),
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
}
