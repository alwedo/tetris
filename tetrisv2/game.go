package tetris

import (
	"context"
	"math"
	"time"
)

const defaultAnimationDelay = 320 * time.Millisecond

type Game struct {
	// UpdateCh will receive a Tetris status every
	// time the status changes by an action.
	UpdateCh <-chan Tetris

	actionCh chan Command
	tetris   *Tetris
}

func Start(ctx context.Context) *Game {
	uCh := make(chan Tetris)
	aCh := make(chan Command)

	g := &Game{
		UpdateCh: uCh,
		actionCh: aCh,
		tetris:   newTetris(),
	}

	ticker := time.NewTicker(setTime(g.tetris))
	ctx, cancel := context.WithCancel(ctx)

	// Ticker goroutine
	go func() {
		for {
			select {
			case <-ticker.C:
				g.actionCh <- MoveDown()
			case <-ctx.Done():
				return
			}
		}
	}()

	// Main game loop
	go func() {
		defer ticker.Stop()
		defer close(uCh)
		defer close(aCh)
		defer cancel()

		for {
			select {
			case cmd := <-aCh:
				isNextRound := cmd(g.tetris)
				if isNextRound {
					ticker.Stop()

					// If we have cleared lines we give the caller time to do an animation.
					if len(g.tetris.LinesClearedIndex) > 0 {
						// sends to Update channel are non-blocking.
						select {
						case uCh <- g.tetris.read():
						default:
						}
						time.Sleep(defaultAnimationDelay)
					}
					g.tetris.finishRound()
				}

				// sends to Update channel are non-blocking.
				select {
				case uCh <- g.tetris.read():
				default:
				}

				if g.tetris.GameOver {
					return
				}

				ticker.Reset(setTime(g.tetris))
			case <-ctx.Done():
				return
			}
		}
	}()

	return g
}

func (g *Game) Do(c Command) {
	g.actionCh <- c
}

func setTime(t *Tetris) time.Duration {
	// setTime() sets the duration for the ticker that will progress the
	// tetromino further down the stack. Based on https://tetris.wiki/Marathon
	//
	// Time = (0.8-((Level-1)*0.007))^(Level-1)
	l := t.Level + int(t.remoteLines) - 1
	seconds := math.Pow(0.8-float64(l)*0.007, float64(l))

	return time.Duration(seconds * float64(time.Second))
}
