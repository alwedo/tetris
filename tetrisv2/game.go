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
	"sync/atomic"
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

// GameMessage is the message sent to the caller after
// every update. Contains the current Tetris status and
// a slice of the cleared lines's indexes.
type GameMessage struct {
	Tetris       Tetris
	ClearedLines []int
}

// Game interfaces between the caller and the Tetris state by managing
// automatic down ticks, state transformation and  game stages.
type Game struct {
	// GameMessageCh will receive a GameMessage every
	// time the status changes by an action.
	//
	// The game will be over when the channel is closed.
	//
	// This channel is non-blocking. Caller is responsible
	// for the timely use of these updates, otherwise
	// the game will drop them.
	GameMessageCh <-chan GameMessage

	actionCh    chan Command
	tetris      *Tetris
	ticker      Ticker
	remoteLines int
	isAnimating atomic.Bool
}

// Start() starts a new Tetris Game.
func Start(ctx context.Context, opts ...GameOpts) *Game {
	uCh := make(chan GameMessage)
	aCh := make(chan Command)

	g := &Game{
		GameMessageCh: uCh,
		actionCh:      aCh,
		tetris:        newTetris(),
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

		var isGameOver bool
		for !isGameOver {
			select {
			case cmd := <-aCh:
				isNextRound := cmd(g)
				if isNextRound {
					g.ticker.Stop()
					linesCleared := g.tetris.toStack()

					// If we have cleared lines we give the caller time to do an animation.
					if len(linesCleared) > 0 {
						g.isAnimating.Store(true)
						select {
						case uCh <- GameMessage{
							Tetris:       g.tetris.read(),
							ClearedLines: linesCleared,
						}:
						default:
						}
						g.ticker.Reset(animationDelay)
						time.AfterFunc(animationDelay, func() {
							g.isAnimating.Store(false)
						})
					} else {
						g.ticker.Reset(g.setTime())
					}

					isGameOver = g.tetris.finishRound(linesCleared)
				}

				// sends to Update channel are non-blocking.
				select {
				case uCh <- GameMessage{Tetris: g.tetris.read()}:
				default:
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
	l := min(g.tetris.Level+g.remoteLines-1, 100)
	seconds := math.Pow(0.8-float64(l)*0.007, float64(l))

	return time.Duration(seconds * float64(time.Second))
}

// Command are functions that change the state of the game.
// They return a bool that indicates if the round is over.
type Command func(*Game) bool

// DropDown moves the tetromino down the stack until it finds
// a collision. This action immediately triggers a new round.
func DropDown() Command {
	return func(g *Game) bool {
		if g.isAnimating.Load() {
			return false
		}
		return g.tetris.action(dropDown)
	}
}

// MoveDown moves the tetromino one step down. If the action can
// not be taken due to a collision, it will trigger a new round.
func MoveDown() Command {
	return func(g *Game) bool {
		if g.isAnimating.Load() {
			return false
		}
		return g.tetris.action(moveDown)
	}
}

// MoveLeft will move the tetromino one step to the left.
// This action has no effect if there is a collision.
func MoveLeft() Command {
	return func(g *Game) bool {
		if g.isAnimating.Load() {
			return false
		}
		return g.tetris.action(moveLeft)
	}
}

// MoveRight will move the tetromino one step to the right.
// This action has no effect if there is a collision.
func MoveRight() Command {
	return func(g *Game) bool {
		if g.isAnimating.Load() {
			return false
		}
		return g.tetris.action(moveRight)
	}
}

// RotateLeft will rotate the tetromino counter clockwise.
// This action has no effect if there is a collision.
func RotateLeft() Command {
	return func(g *Game) bool {
		if g.isAnimating.Load() {
			return false
		}
		return g.tetris.action(rotateLeft)
	}
}

// RotateRight will rotate the tetromino clockwise.
// This action has no effect if there is a collision.
func RotateRight() Command {
	return func(g *Game) bool {
		if g.isAnimating.Load() {
			return false
		}
		return g.tetris.action(rotateRight)
	}
}

// AddRemoteLines will increase the number of remote lines by i.
func AddRemoteLines(i int) Command {
	return func(g *Game) bool {
		g.remoteLines += i
		return false
	}
}
