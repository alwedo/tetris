package tetris

import (
	"math/rand"
	"slices"
)

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

	// After finishing a round this will display the indexes
	// of the lines cleared in the stack. Callers can use
	// this to render cleared lines animations.
	// When there are lines cleared Tetris will wait to
	// start the new round by an amount determined by the
	// animation time.
	LinesClearedIndex []int

	gameOver    bool
	bag         *bag
	remoteLines int
}

type rotateAction string

const (
	rotateRight rotateAction = "rotatecw"  // Rotates the Tetromino clockwise.
	rotateLeft  rotateAction = "rotateccw" // Rotates the Tetromino counter-clockwise.
)

// Command are functions that change the state of the game.
// They return a bool that indicates if the round is over.
type Command func(*Tetris) bool

// DropDown moves the tetromino down the stack until it finds
// a collision. This action immediately triggers a new round.
func DropDown() Command {
	return func(t *Tetris) bool {
		t.Tetromino.Y += t.dropDownDelta()
		return true
	}
}

// MoveDown moves the tetromino one step down. If the action can
// not be taken due to a collision, it will trigger a new round.
func MoveDown() Command {
	return func(t *Tetris) bool {
		if t.isCollision(0, -1, t.Tetromino) {
			return true
		}
		t.Tetromino.Y--
		return false
	}
}

// MoveLeft will move the tetromino one step to the left.
// This action has no effect if there is a collision.
func MoveLeft() Command {
	return func(t *Tetris) bool {
		if !t.isCollision(-1, 0, t.Tetromino) {
			t.Tetromino.X--
			t.Tetromino.GhostY = t.Tetromino.Y + t.dropDownDelta()
		}
		return false
	}
}

// MoveRight will move the tetromino one step to the right.
// This action has no effect if there is a collision.
func MoveRight() Command {
	return func(t *Tetris) bool {
		if !t.isCollision(1, 0, t.Tetromino) {
			t.Tetromino.X++
			t.Tetromino.GhostY = t.Tetromino.Y + t.dropDownDelta()
		}
		return false
	}
}

// RotateLeft will rotate the tetromino counter clockwise.
// This action has no effect if there is a collision.
func RotateLeft() Command {
	return func(t *Tetris) bool {
		t.rotate(rotateLeft)
		return false
	}
}

// RotateRight will rotate the tetromino clockwise.
// This action has no effect if there is a collision.
func RotateRight() Command {
	return func(t *Tetris) bool {
		t.rotate(rotateRight)
		return false
	}
}

// AddRemoteLines will increase the number of remote lines by i.
func AddRemoteLines(i int) Command {
	return func(t *Tetris) bool {
		t.remoteLines += i
		return false
	}
}

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

func (t *Tetris) rotate(a rotateAction) {
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

func (t *Tetris) isCollision(deltaX, deltaY int, tetromino *Tetromino) bool {
	// isCollision() will receive the desired future X and Y tetromino's position
	// and calculate if there is a collision or if it's out of bounds from the stack
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

func (t *Tetris) toStack() {
	// moves the tetromino to the stack
	for ix, x := range t.Tetromino.Grid {
		for iy, y := range x {
			if y {
				t.Stack[t.Tetromino.Y-ix][t.Tetromino.X+iy] = t.Tetromino.Shape
			}
		}
	}

	// Add the indexes of cleared lines. This will determine downstream
	// steps like lines to be cleared, increase in Lines and Level and
	// consumer animation time.
	for i, x := range t.Stack {
		if !slices.Contains(x, "") {
			t.LinesClearedIndex = append(t.LinesClearedIndex, i)
		}
	}
}

func (t *Tetris) setTetromino() {
	if t.NextTetromino == nil {
		t.NextTetromino = t.bag.draw()
	}

	// we consider game over when next tetromino spawn's
	// position would have a collision with the stack.
	if t.isCollision(0, 0, t.NextTetromino) {
		t.gameOver = true
		return
	}

	t.Tetromino, t.NextTetromino = t.NextTetromino, t.bag.draw()
	t.Tetromino.GhostY = t.Tetromino.Y + t.dropDownDelta()
}

func (t *Tetris) finishRound() {
	// - Removes completed lines
	// - Increases Lines count
	// - Empties the LinesClearedIndex
	// - Calculates new level
	// - Rotates Tetromino and NexTetromino

	if len(t.LinesClearedIndex) > 0 {
		// remove complete lines in reverse order to avoid index shift issues.
		for i := len(t.LinesClearedIndex) - 1; i >= 0; i-- {
			t.Stack = append(t.Stack[:t.LinesClearedIndex[i]], t.Stack[t.LinesClearedIndex[i]+1:]...)
			t.Stack = append(t.Stack, make([]Shape, 10))
		}
		t.Lines += len(t.LinesClearedIndex)
		t.LinesClearedIndex = nil

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

	t.setTetromino() // evaluates game over
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

	cleared := append([]int(nil), t.LinesClearedIndex...)
	return Tetris{
		Stack:             stack,
		Tetromino:         t.Tetromino.copy(),
		NextTetromino:     t.NextTetromino.copy(),
		Level:             t.Level,
		Lines:             t.Lines,
		LinesClearedIndex: cleared,
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
