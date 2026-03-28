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
//
//	func main() {
//		ctx, cancel := context.WithCancel(context.Background())
//		defer cancel()
//		t := tetris.Start(ctx)
//
//		// asynchronously send commands to the game
//		go func() {
//			for {
//				select {
//				case msg, ok := <-t.UpdateCh:
//					if !ok { // game over
//						return
//					}
//					// use tetris status
//					fmt.Println(msg)
//				case <-ctx.Done():
//					// use cancel func to end the game if needed
//					return
//				}
//			}
//		}()
//
//		t.Do(tetris.MoveRight()) // or any other action
//	}
package tetris

import (
	"context"
	"math"
	"time"
)

const animationDelay = 320 * time.Millisecond

type GameOpts func(*Game)

// WithCustomTicker provides a custom ticker that
// replaces the default time.Ticker. Used for testing.
func WithCustomTicker(t Ticker) GameOpts {
	return func(g *Game) {
		g.ticker.Stop()
		g.ticker = t
	}
}

// WithCustomStack modifies the stack given the provided index
// and row configuration. Used for testing.
func WithCustomStack(update map[int][]Shape) GameOpts {
	return func(g *Game) {
		for k, v := range update {
			g.tetris.Stack[k] = v
		}
	}
}

// WithCustomShape will set the current Tetromino to the
// provided shape. Used for testing.
func WithCustomShape(s Shape) GameOpts {
	return func(g *Game) {
		g.tetris.Tetromino = shapeMap[s]()
	}
}

// Game interfaces between the caller and the Tetris state by managing
// automatic down ticks, state transformation and  game stages.
type Game struct {
	// UpdateCh will receive a Tetris status every
	// time the status changes by an action.
	//
	// The game will be over when the channel is closed.
	//
	// This channel is non-blocking. Caller is responsible
	// for the timely use of these updates, otherwise
	// the game will drop them.
	UpdateCh <-chan Tetris

	actionCh chan Command
	tetris   *Tetris
	ticker   Ticker
}

// Start() starts a new Tetris Game.
func Start(ctx context.Context, opts ...GameOpts) *Game {
	uCh := make(chan Tetris)
	aCh := make(chan Command)

	g := &Game{
		UpdateCh: uCh,
		actionCh: aCh,
		tetris:   newTetris(),
	}
	g.ticker = newTimeTicker(g.setTime())
	for _, o := range opts {
		o(g)
	}

	// Ticker goroutine
	go func() {
		for {
			select {
			case <-g.ticker.C():
				// Ticker always reset itself
				g.ticker.Reset(g.setTime())
				select {
				case g.actionCh <- MoveDown():
				default:
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Main game loop
	go func() {
		defer g.ticker.Stop()
		defer close(uCh)
		defer close(aCh)

		for {
			select {
			case cmd := <-aCh:
				isNextRound := cmd(g.tetris)
				if isNextRound {
					g.ticker.Stop()
					g.tetris.toStack()

					// If we have cleared lines we give the caller time to do an animation.
					if len(g.tetris.LinesClearedIndex) > 0 {
						// sends to Update channel are non-blocking.
						select {
						case uCh <- g.tetris.read():
						default:
						}
						g.ticker.Reset(animationDelay)
					} else {
						g.ticker.Reset(g.setTime())
					}

					g.tetris.finishRound()
				}

				// sends to Update channel are non-blocking.
				select {
				case uCh <- g.tetris.read():
				default:
				}

				if g.tetris.gameOver {
					return
				}

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
	select {
	case g.actionCh <- c:
	default:
	}
}

func (g *Game) setTime() time.Duration {
	// setTime() sets the duration for the ticker that will progress the
	// tetromino further down the stack. Based on https://tetris.wiki/Marathon
	//
	// Time = (0.8-((Level-1)*0.007))^(Level-1)
	// We cap l to 100 to avoid overflowing.
	l := min(g.tetris.Level+g.tetris.remoteLines-1, 100)
	seconds := math.Pow(0.8-float64(l)*0.007, float64(l))

	return time.Duration(seconds * float64(time.Second))
}
