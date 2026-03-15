// Package tetris contains the logic of the game.
// Based on:
//   - https://tetris.wiki/Tetris_Guideline
//   - https://tetris.fandom.com/wiki/Tetris_Guideline
//
// Tetris © 1985~2025 Tetris Holding.
// Tetris logos, Tetris theme song and Tetriminos are trademarks of Tetris Holding.
// The Tetris trade dress is owned by Tetris Holding.
// Licensed to The Tetris Company.
// Tetris Game Design by Alexey Pajitnov.
// Tetris Logo Design by Roger Dean.
// All Rights Reserved.
//
// Usage:
// ctx, cancel := context.WithCancel(context.Background())
// defer cancel()
//
// g := tetris.Start(ctx)
//
//	go func() {
//		for {
//			select {
//			case t := <-g.UpdateCh:
//				if t.GameOver {
//					// finish game.
//					// can use cancel() here to signal exiting
//					return
//				}
//				// use t to print the status.
//			case <-ctx.Done():
//				return
//			}
//		}
//	}()
//
// g.Do(tetris.MoveLeft()) // send commands

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
	// This channel is non-blocking. Caller is responible
	// for the timely use of these updates, otherwise
	// the game will drop them.
	UpdateCh <-chan Tetris

	actionCh chan Command
	tetris   *Tetris
}

// Start() starts a new Tetris Game.
// Use a context with cancellation to
// control when to cancel the game.
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

// Do() performs a command of the tetris.Command type.
// This function is safe to call asynchronously.
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
