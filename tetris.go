// Package tetris contains the game engine.
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
// Example usage:
// package main
//
// import (
//
//	"context"
//	"fmt"
//
//	"github.com/alwedo/tetris"
//
// )
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
	"math/rand"
	"slices"
	"sync/atomic"
	"time"
)

const animationDelay = 320 * time.Millisecond

type Ticker interface {
	C() <-chan time.Time
	Reset(time.Duration)
	Stop()
}

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
	isClosed    atomic.Bool
}

// Start() starts a new Tetris Game.
func Start(ctx context.Context, opts ...GameOpts) *Game {
	uCh := make(chan GameMessage, 1)
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

	// we send a first read to the channel so
	// there is no delay on viewing the first piece.
	uCh <- GameMessage{Tetris: g.tetris.read()}

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
		defer g.isClosed.Store(true)
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
	if g.isClosed.Load() {
		return
	}
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
		g.remoteLines = i
		return false
	}
}

// Tetris contains the state of the game.
type Tetris struct {
	// Stack is the playfield. 20 rows x 10 columns.
	// Columns are 0 > 9 left to right and represent the X axis
	// Rows are 19 > 0 top to bottom and represent the Y axis
	// An empty string is an empty cell. Otherwise it has the
	// shape of the tetromino.
	Stack [][]Shape

	// Current Tetromino in play.
	Tetromino *Tetromino

	// Next Tetromino to be drawn.
	NextTetromino *Tetromino

	// Current level.
	Level int

	// Number of lines cleared.
	Lines int

	bag *bag
}

type action int

const (
	moveLeft action = iota
	moveRight
	moveDown
	dropDown
	rotateLeft
	rotateRight
)

func newTetris() *Tetris {
	// creates an empty 20x10 stack
	s := make([][]Shape, 20)
	for i := range s {
		s[i] = make([]Shape, 10)
	}

	t := &Tetris{
		Stack: s,
		Level: 1,
		bag:   newBag(),
	}
	t.setTetromino()
	return t
}

// action will perform the requested action and return
// true if the action caused the round to finish.
// Only dropDown and moveDown on collision return true.
func (t *Tetris) action(a action) bool {
	switch a {
	case dropDown:
		t.Tetromino.Y += t.dropDownDelta()
		return true
	case moveDown:
		if t.isCollision(0, -1, t.Tetromino) {
			return true
		}
		t.Tetromino.Y--
	case moveLeft:
		if !t.isCollision(-1, 0, t.Tetromino) {
			t.Tetromino.X--
		}
	case moveRight:
		if !t.isCollision(1, 0, t.Tetromino) {
			t.Tetromino.X++
		}
	case rotateLeft, rotateRight:
		t.rotate(a)
	default:
		// Unlisted actions are ignored
	}
	t.Tetromino.GhostY = t.Tetromino.Y + t.dropDownDelta()

	return false
}

func (t *Tetris) rotate(a action) {
	// https://tetris.wiki/Super_Rotation_System
	if t.Tetromino.Shape == O {
		// the O shape doesn't rotate.
		return
	}

	// we create a test Tetromino with the current XY coordinates
	// and a grid with the same dimensions of the current Tetromino.
	test := &Tetromino{
		Grid: make([][]bool, len(t.Tetromino.Grid)),
		X:    t.Tetromino.X,
		Y:    t.Tetromino.Y,
	}
	for i := range t.Tetromino.Grid {
		test.Grid[i] = make([]bool, len(t.Tetromino.Grid[i]))
	}

	// rotates the grid
	for ix, x := range t.Tetromino.Grid {
		switch a {
		case rotateRight:
			col := len(x) - ix - 1
			for iy, y := range x {
				test.Grid[iy][col] = y
			}
		case rotateLeft:
			for iy, y := range x {
				test.Grid[len(x)-iy-1][ix] = y
			}
		}
	}

	var rCase string
	switch {
	case t.Tetromino.rState.Value == rState0 && a == rotateRight:
		rCase = "0>R"
	case t.Tetromino.rState.Value == rStateR && a == rotateLeft:
		rCase = "R>0"
	case t.Tetromino.rState.Value == rStateR && a == rotateRight:
		rCase = "R>2"
	case t.Tetromino.rState.Value == rState2 && a == rotateLeft:
		rCase = "2>R"
	case t.Tetromino.rState.Value == rState2 && a == rotateRight:
		rCase = "2>L"
	case t.Tetromino.rState.Value == rStateL && a == rotateLeft:
		rCase = "L>2"
	case t.Tetromino.rState.Value == rStateL && a == rotateRight:
		rCase = "L>0"
	case t.Tetromino.rState.Value == rState0 && a == rotateLeft:
		rCase = "0>L"
	}

	var rGroup = "all"
	if t.Tetromino.Shape == I {
		rGroup = "I"
	}

	for _, v := range wallKickMap[rGroup][rCase] {
		if !t.isCollision(v[0], v[1], test) {
			t.Tetromino.Grid = test.Grid
			t.Tetromino.X += v[0]
			t.Tetromino.Y += v[1]
			switch a {
			case rotateRight:
				t.Tetromino.rState = t.Tetromino.rState.Next()
			case rotateLeft:
				t.Tetromino.rState = t.Tetromino.rState.Prev()
			}
			t.Tetromino.GhostY = t.Tetromino.Y + t.dropDownDelta()
			return
		}
	}
}

// isCollision() will receive the desired future X and Y tetromino's position
// and calculate if there is a collision or if it's out of bounds from the stack
func (t *Tetris) isCollision(deltaX, deltaY int, tetromino *Tetromino) bool {
	for iy, y := range tetromino.Grid {
		for ix, x := range y {
			// we check only if the tetromino cell is true as we don't
			// care if the tetromino grid is out of bounds or in collision.
			if x {
				// the position of the tetromino cell against the stack is:
				// current X and Y + cell index offset + desired position offset
				// Y axis decrease to 0 so we need to subtract the index
				yPos := tetromino.Y - iy + deltaY
				xPos := tetromino.X + ix + deltaX

				// check if cell is out of bounds for X, Y and against the stack.
				if yPos < 0 || yPos > 19 || xPos < 0 || xPos > 9 || t.Stack[yPos][xPos] != "" {
					return true
				}
			}
		}
	}
	return false
}

// toStack moves the current tetromino to the stack after
// a collision that prevents it from moving further down.
// It returns a slice of indexes of the lines to be cleared.
func (t *Tetris) toStack() []int {
	// moves the tetromino to the stack
	for ix, x := range t.Tetromino.Grid {
		for iy, y := range x {
			if y {
				t.Stack[t.Tetromino.Y-ix][t.Tetromino.X+iy] = t.Tetromino.Shape
			}
		}
	}

	var lines []int
	for i, x := range t.Stack {
		if !slices.Contains(x, "") {
			lines = append(lines, i)
		}
	}
	return lines
}

// setTetromino uses the bag to draw both current
// and next tetrominos. It returns a bool that would
// indicate game over if NextTetromino has a collision.
func (t *Tetris) setTetromino() bool {
	if t.NextTetromino == nil {
		t.NextTetromino = t.bag.draw()
	}

	// we consider game over when next tetromino spawn's
	// position would have a collision with the stack.
	if t.isCollision(0, 0, t.NextTetromino) {
		return true
	}

	t.Tetromino, t.NextTetromino = t.NextTetromino, t.bag.draw()
	t.Tetromino.GhostY = t.Tetromino.Y + t.dropDownDelta()
	return false
}

// finishRound takes a slice of completed lines indexes and
// performs end-of-round tasks. It returns the response from
// setTetromino, determining if the game is over.
// - Removes completed lines
// - Increases Lines count
// - Calculates new level
// - Executes setTetromino
func (t *Tetris) finishRound(lines []int) bool {
	if len(lines) > 0 {
		// remove complete lines in reverse order to avoid index shift issues.
		for i := len(lines) - 1; i >= 0; i-- {
			t.Stack = slices.Delete(t.Stack, lines[i], lines[i]+1)
			t.Stack = append(t.Stack, make([]Shape, 10))
		}
		t.Lines += len(lines)

		// set the fixed-goal level system
		// https://tetris.wiki/Marathon
		//
		// In the fixed-goal system, each level requires 10 lines to clear.
		// If the player starts at a later level, the number of lines required is the same
		// as if starting at level 1. An example is when the player starts at level 5,
		// the player will have to clear 50 lines to advance to level 6
		var l int
		switch {
		case t.Lines < 10:
			l = 1
		case t.Lines >= 10 && t.Lines < 100:
			l = (t.Lines/10)%10 + 1
		case t.Lines >= 100:
			l = t.Lines/10 + 1
		}
		if l > t.Level {
			t.Level = l
		}
	}

	return t.setTetromino() // evaluates game over
}

func (t *Tetris) dropDownDelta() int {
	// dropDownDelta calculates how much the Tetromino
	// has to move down until the next collision.
	var delta int
	for !t.isCollision(0, delta, t.Tetromino) {
		delta--
	}
	return delta + 1
}

// read() returns a copy of the current Tetris status that's safe to read concurrently.
func (t *Tetris) read() Tetris {
	stack := make([][]Shape, len(t.Stack))
	for i := range t.Stack {
		stack[i] = append([]Shape(nil), t.Stack[i]...)
	}

	return Tetris{
		Stack:         stack,
		Tetromino:     t.Tetromino.copy(),
		NextTetromino: t.NextTetromino.copy(),
		Level:         t.Level,
		Lines:         t.Lines,
	}
}

type bag struct {
	firstDraw bool
	bag       []Shape
}

func newBag() *bag {
	return &bag{firstDraw: true}
}

func (b *bag) draw() *Tetromino {
	// https://tetris.wiki/Random_Generator
	// first piece is always I, J, L, or T
	// new bag is generated after last piece is drawn
	if len(b.bag) == 0 {
		b.bag = []Shape{I, T, J, L, O, S, Z}
	}

	candidates := b.bag
	if b.firstDraw {
		candidates = []Shape{I, T, J, L}
		b.firstDraw = false
	}

	t := candidates[rand.Intn(len(candidates))] //nolint: gosec
	b.bag = slices.DeleteFunc(b.bag, func(tt Shape) bool { return tt == t })
	return shapeMap[t]()
}

var wallKickMap = map[string]map[string][][]int{
	"all": {
		"0>R": [][]int{{0, 0}, {-1, 0}, {-1, 1}, {0, -2}, {-1, -2}},
		"R>0": [][]int{{0, 0}, {1, 0}, {1, -1}, {0, 2}, {1, 2}},
		"R>2": [][]int{{0, 0}, {1, 0}, {1, -1}, {0, 2}, {1, 2}},
		"2>R": [][]int{{0, 0}, {-1, 0}, {-1, 1}, {0, -2}, {-1, -2}},
		"2>L": [][]int{{0, 0}, {1, 0}, {1, 1}, {0, -2}, {1, -2}},
		"L>2": [][]int{{0, 0}, {-1, 0}, {-1, -1}, {0, 2}, {-1, 2}},
		"L>0": [][]int{{0, 0}, {-1, 0}, {-1, -1}, {0, 2}, {-1, 2}},
		"0>L": [][]int{{0, 0}, {1, 0}, {1, 1}, {0, -2}, {1, -2}},
	},
	"I": {
		"0>R": [][]int{{0, 0}, {-2, 0}, {1, 0}, {-2, -1}, {1, 2}},
		"R>0": [][]int{{0, 0}, {2, 0}, {-1, 0}, {2, 1}, {-1, -2}},
		"R>2": [][]int{{0, 0}, {-1, 0}, {2, 0}, {-1, 2}, {2, -1}},
		"2>R": [][]int{{0, 0}, {1, 0}, {-2, 0}, {1, -2}, {-2, 1}},
		"2>L": [][]int{{0, 0}, {2, 0}, {-1, 0}, {2, 1}, {-1, -2}},
		"L>2": [][]int{{0, 0}, {-2, 0}, {1, 0}, {-2, -1}, {1, 2}},
		"L>0": [][]int{{0, 0}, {1, 0}, {-2, 0}, {1, -2}, {-2, 1}},
		"0>L": [][]int{{0, 0}, {-1, 0}, {2, 0}, {-1, 2}, {2, -1}},
	},
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
