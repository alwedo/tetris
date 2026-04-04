package tetris

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/alwedo/tetris/tetris/tetristest"
)

func TestStart(t *testing.T) {
	t.Run("ticker should call MoveDown(), decreasing the tetromino Y position by 1 and resetting the ticker", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			mockTicker := tetristest.NewMockTicker()
			game := Start(t.Context(), WithCustomTicker(mockTicker))
			wantTetrominoPos := game.tetris.Tetromino.Y - 1

			<-game.GameMessageCh // skip first message to render stack
			var wg sync.WaitGroup
			wg.Go(func() {
				msg := <-game.GameMessageCh
				if msg.Tetris.Tetromino.Y != wantTetrominoPos {
					t.Errorf("wanted tetromino Y Position to be %d, got %d", wantTetrominoPos, msg.Tetris.Tetromino.Y)
				}
				if mockTicker.ResetCount.Load() != 1 {
					t.Errorf("wanted ticker resetCount to be 1, got %d", mockTicker.ResetCount.Load())
				}
				if mockTicker.LastResetDuration.Load() != int64(game.setTime()) {
					t.Errorf("wanted reset duration to be %d, got %d", int64(game.setTime()), mockTicker.LastResetDuration.Load())
				}
			})

			time.AfterFunc(time.Millisecond, func() { mockTicker.Tick() })
			wg.Wait()
		})
	})

	t.Run("action that triggers a new round stops and resets the ticker", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			mockTicker := tetristest.NewMockTicker()
			game := Start(t.Context(), WithCustomTicker(mockTicker))

			<-game.GameMessageCh // skip first message to render stack
			var wg sync.WaitGroup
			wg.Go(func() {
				<-game.GameMessageCh
				if mockTicker.ResetCount.Load() != 1 {
					t.Errorf("wanted ticker resetCount to be 1, got %d", mockTicker.ResetCount.Load())
				}
				if mockTicker.StopCount.Load() != 1 {
					t.Errorf("wanted ticker stopCount to be 1, got %d", mockTicker.StopCount.Load())
				}
			})

			time.AfterFunc(time.Millisecond, func() { game.Do(DropDown()) })
			wg.Wait()
		})
	})

	t.Run("new round action with cleared lines will delay ticker for animation", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			mockTicker := tetristest.NewMockTicker()
			game := Start(ctx,
				WithCustomTicker(mockTicker),
				WithCustomStack(map[int][]Shape{
					0: {I, I, I, "", "", "", "", I, I, I},
				}),
				WithCustomShape(I),
			)

			<-game.GameMessageCh // skip first message to render stack
			var wg sync.WaitGroup
			wg.Go(func() {
				for {
					msg, ok := <-game.GameMessageCh
					if !ok {
						return
					}
					wantClearedLinesIndex := []int{0}
					if !reflect.DeepEqual(wantClearedLinesIndex, msg.ClearedLines) {
						t.Errorf("wanted cleared lines index %v, got %v", wantClearedLinesIndex, msg.ClearedLines)
					}
					if mockTicker.LastResetDuration.Load() != animationDelay.Nanoseconds() {
						t.Errorf("wanted ticker reset duration to be %d, got %d", animationDelay.Nanoseconds(), mockTicker.LastResetDuration.Load())
					}
				}
			})

			game.Do(DropDown())
			time.Sleep(time.Millisecond) // sleep until the next tick
			cancel()                     // quit the game
			wg.Wait()
		})
	})

	t.Run("channel close on game over", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			game := Start(t.Context())
			var wg = sync.WaitGroup{}
			var wantGameOver bool
			wg.Go(func() {
				for {
					_, ok := <-game.GameMessageCh
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
