package tetris

import (
	"fmt"
	"reflect"
	"testing"
)

func TestStack(t *testing.T) {
	t.Run("New tetris starts with empty stack", func(t *testing.T) {
		tetris := NewTestTetris(J)
		for _, c := range tetris.Stack {
			for _, r := range c {
				if r != "" {
					t.Errorf("Expected cell to be an empty string, got %v", r)
				}
			}
		}
	})
}

func TestIsCollision(t *testing.T) {
	// 		0 1 2 3 4 5 6 7 8 9			0 1 2
	// 19	X X X O X X X X X X		0	O X X
	// 18	X X X O O O X X X X		1	O O O
	// 17	X X X X X C X X X X		2	X X X
	tests := []struct {
		name           string
		deltaX, deltaY int
		wantCollision  bool
	}{
		{
			name: "no collision",
		},
		{
			name:          "stack collision",
			deltaY:        -1,
			wantCollision: true,
		},
		{
			name:          "left bond collision",
			deltaX:        -4,
			wantCollision: true,
		},
		{
			name:          "right bond collision",
			deltaX:        5,
			wantCollision: true,
		},
		{
			name:          "bottom bond collision",
			deltaY:        -19,
			wantCollision: true,
		},
		{
			name: "upper bond collision",
			// when drafting an I and rotating it immediately, it
			// should put the tetromino out of the upper bond.
			// the collision should allow for a wall-kick.
			deltaY:        1,
			wantCollision: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tetris := NewTestTetris(J)
			tetris.Stack[17][5] = "C"

			c := tetris.isCollision(tt.deltaX, tt.deltaY, tetris.Tetromino)
			if c && !tt.wantCollision {
				t.Errorf("Expected no collision")
			}
			if !c && tt.wantCollision {
				t.Errorf("Expected collision")
			}
		})
	}
}

func TestMoveActions(t *testing.T) {
	// Initial state of the test:
	//
	// 	.	Spawn Location		.	Shape
	// .	0 1 2 3 4 5 6 7 8 9		.	0 1 2
	// 19	X X X O X X X X X X		0	O X X
	// 18	X X X O O O X X X X		1	O O O
	// 17	X X X X X X X X X X		2	X X X
	tests := []struct {
		name         string
		action       Action
		updateStack  func(g *Tetris)
		wantGrid     [][]bool
		wantLocation []int // x, y
	}{
		{
			name:         "Move left unblocked",
			action:       MoveLeft,
			wantLocation: []int{19, 2},
		},
		{
			name:   "Move left blocked",
			action: MoveLeft,
			updateStack: func(g *Tetris) {
				g.Stack[18][2] = J
			},
			wantLocation: []int{19, 3},
		},
		{
			name:         "Move right unblocked",
			action:       MoveRight,
			wantLocation: []int{19, 4},
		},
		{
			name:   "Move right blocked",
			action: MoveRight,
			updateStack: func(g *Tetris) {
				g.Stack[18][6] = J
			},
			wantLocation: []int{19, 3},
		},
		{
			name:         "Move down unblocked",
			action:       MoveDown,
			wantLocation: []int{18, 3},
		},
		{
			name:   "Move down blocked",
			action: MoveDown,
			updateStack: func(g *Tetris) {
				g.Stack[17][3] = J
			},
			wantLocation: []int{19, 3},
		},
		{
			name:         "Drop moves down until blocked",
			action:       DropDown,
			wantLocation: []int{1, 3},
		},
		{
			name:         "Rotate right when unblocked",
			action:       RotateRight,
			wantLocation: []int{19, 3},
			wantGrid: [][]bool{
				{false, true, true},
				{false, true, false},
				{false, true, false},
			},
		},
		{
			name:         "Rotate left when unblocked",
			action:       RotateLeft,
			wantLocation: []int{19, 3},
			wantGrid: [][]bool{
				{false, true, false},
				{false, true, false},
				{true, true, false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tetris := NewTestTetris(J)
			if tt.updateStack != nil {
				tt.updateStack(tetris)
			}
			tetris.action(tt.action)
			if tetris.Tetromino.Y != tt.wantLocation[0] {
				t.Errorf("wanted tetromino's Y to be %d, got %d", tt.wantLocation[0], tetris.Tetromino.Y)
			}
			if tetris.Tetromino.X != tt.wantLocation[1] {
				t.Errorf("wanted tetromino's X to be %d, got %d", tt.wantLocation[1], tetris.Tetromino.X)
			}
			if tt.wantGrid != nil {
				if !reflect.DeepEqual(tetris.Tetromino.Grid, tt.wantGrid) {
					t.Errorf("wanted %v, got %v", tt.wantGrid, tetris.Tetromino.Grid)
				}
			}
		})
	}
}

func TestWallKick(t *testing.T) {
	// for this test we set the tetromino in the middle of the stack to
	// allow for setting up multiple blocks in order to test all the cases.
	// we don't test for case 0 (0,0) since that donesn't wall kick.
	tests := []struct {
		name         string
		shape        Shape
		action       Action
		blockStack   [][]int
		setR         func(g *Tetris)
		wantX, wantY int
	}{
		{
			name: "I tetromino, case 0>R, test 2 (-2,0)",
			// .	0 1 2 3 4 5 6 7 8 9
			// 10	. . . . . X . . . .
			// 9	. . . O O O O . . .
			shape:      I,
			action:     RotateRight,
			blockStack: [][]int{{10, 5}},
			wantX:      1,
			wantY:      10,
		},
		{
			name: "I tetromino, case 0>R, test 3 (1, 0)",
			// .	0 1 2 3 4 5 6 7 8 9
			// 10	. . . X . X . . . .
			// 9	. . . O O O O . . .
			shape:      I,
			action:     RotateRight,
			blockStack: [][]int{{10, 5}, {10, 3}},
			wantX:      4,
			wantY:      10,
		},
		{
			name: "I tetromino, case 0>R, test 4 (-2, -1)",
			// .	0 1 2 3 4 5 6 7 8 9
			// 10	. . . X . X . . . .
			// 9	. . . O O O O . . .
			// 8	. . . . . . X . . .
			shape:      I,
			action:     RotateRight,
			blockStack: [][]int{{8, 6}, {10, 5}, {10, 3}},
			wantX:      1,
			wantY:      9,
		},
		{
			name: "I tetromino, case 0>R, test 5 (1, 2)",
			// .	0 1 2 3 4 5 6 7 8 9
			// 10	. . . X . X . . . .
			// 9	. . . O O O O . . .
			// 8	. . . X . . X . . .
			shape:      I,
			action:     RotateRight,
			blockStack: [][]int{{8, 3}, {8, 6}, {10, 5}, {10, 3}},
			wantX:      4,
			wantY:      12,
		},
		{
			name: "I tetromino, case R>0, test 2 (2, 0)",
			// .	0 1 2 3 4 5 6 7 8 9
			// 10	O . . . . . . . . .
			// 9	O . . . . . . . . .
			// 8	O . . . . . . . . .
			// 7    O . . . . . . . . .
			shape:  I,
			action: RotateLeft,
			setR: func(g *Tetris) {
				// for this case we put the tetromino against the left wall
				g.Tetromino.X = -2
				g.rotate(RotateRight)
			},
			wantX: 0,
			wantY: 10,
		},
		{
			name: "I tetromino, case R>0, test 3 (-1, 0)",
			// .	0 1 2 3 4 5 6 7 8 9
			// 10	. . . . . . . . . O
			// 9	. . . . . . . . . O
			// 8	. . . . . . . . . O
			// 7    . . . . . . . . . O
			shape:  I,
			action: RotateLeft,
			setR: func(g *Tetris) {
				// for this case we put the tetromino against the right wall
				g.Tetromino.X = 7
				g.rotate(RotateRight)
			},
			wantX: 6,
			wantY: 10,
		},
		{
			name: "I tetromino, case R>0, test 4 (2, 1)",
			// .	0 1 2 3 4 5 6 7 8 9
			// 10	. . . . . O . . . .
			// 9	. . . X . O X . . .
			// 8	. . . . . O . . . .
			// 7    . . . . . O . . . .
			shape:      I,
			action:     RotateLeft,
			blockStack: [][]int{{9, 3}, {9, 6}},
			setR:       func(g *Tetris) { g.rotate(RotateRight) },
			wantX:      5,
			wantY:      11,
		},
		{
			name: "I tetromino, case R>0, test 5 (-1, -2)",
			// .	0 1 2 3 4 5 6 7 8 9
			// 10	. . . . . O X . . .
			// 9	. . . X . O X . . .
			// 8	. . . . . O . . . .
			// 7    . . . . . O . . . .
			shape:      I,
			action:     RotateLeft,
			blockStack: [][]int{{9, 3}, {9, 6}, {10, 6}},
			setR:       func(g *Tetris) { g.rotate(RotateRight) },
			wantX:      2,
			wantY:      8,
		},
		{
			name: "I tetromino, case R>2, test 2 (-1, 0)",
			// .	0 1 2 3 4 5 6 7 8 9
			// 10	. . . . . . . . . O
			// 9	. . . . . . . . . O
			// 8	. . . . . . . . . O
			// 7    . . . . . . . . . O
			shape:  I,
			action: RotateRight,
			setR: func(g *Tetris) {
				// for this case we put the tetromino against the right wall
				g.Tetromino.X = 7
				g.rotate(RotateRight)
			},
			wantX: 6,
			wantY: 10,
		},
		{
			name: "I tetromino, case R>2, test 3 (2, 0)",
			// .	0 1 2 3 4 5 6 7 8 9
			// 10	O . . . . . . . . .
			// 9	O . . . . . . . . .
			// 8	O . . . . . . . . .
			// 7    O . . . . . . . . .
			shape:  I,
			action: RotateRight,
			setR: func(g *Tetris) {
				// for this case we put the tetromino against the left wall
				g.Tetromino.X = -2
				g.rotate(RotateRight)
			},
			wantX: 0,
			wantY: 10,
		},
		{
			name: "I tetromino, case R>2, test 4 (-1, 2)",
			// .	0 1 2 3 4 5 6 7 8 9
			// 10	. . . . . O . . . .
			// 9	. . . . . O . . . .
			// 8	. . . X . O X . . .
			// 7    . . . . . O . . . .
			shape:      I,
			action:     RotateRight,
			blockStack: [][]int{{8, 3}, {8, 6}},
			setR:       func(g *Tetris) { g.rotate(RotateRight) },
			wantX:      2,
			wantY:      12,
		},
		{
			name: "I tetromino, case R>2, test 5 (2, -1)",
			// .	0 1 2 3 4 5 6 7 8 9
			// 10	. . . X . O . . . .
			// 9	. . . . . O . . . .
			// 8	. . . X . O X . . .
			// 7    . . . . . O . . . .
			shape:      I,
			action:     RotateRight,
			blockStack: [][]int{{8, 3}, {8, 6}, {10, 3}},
			setR:       func(g *Tetris) { g.rotate(RotateRight) },
			wantX:      5,
			wantY:      9,
		},
		{
			name: "I tetromino, case 2>R, test 2 (1, 0)",
			// .	0 1 2 3 4 5 6 7 8 9
			// 10	. . . . . . . . . .
			// 9	. . . . . X . . . .
			// 8	. . . O O O O . . .
			// 7    . . . . . . . . . .
			shape:      I,
			action:     RotateLeft,
			blockStack: [][]int{{9, 5}},
			setR: func(g *Tetris) {
				g.rotate(RotateRight)
				g.rotate(RotateRight)
			},
			wantX: 4,
			wantY: 10,
		},
		{
			name: "I tetromino, case 2>R, test 3 (-2, 0)",
			// .	0 1 2 3 4 5 6 7 8 9
			// 10	. . . . . . . . . .
			// 9	. . . . . X X . . .
			// 8	. . . O O O O . . .
			// 7    . . . . . . . . . .
			shape:      I,
			action:     RotateLeft,
			blockStack: [][]int{{9, 5}, {9, 6}},
			setR: func(g *Tetris) {
				g.rotate(RotateRight)
				g.rotate(RotateRight)
			},
			wantX: 1,
			wantY: 10,
		},
		{
			name: "I tetromino, case 2>R, test 4 (1, -2)",
			// .	0 1 2 3 4 5 6 7 8 9
			// 10	. . . . . . . . . .
			// 9	. . . . . X X . . .
			// 8	. . . O O O O . . .
			// 7    . . . X . . . . . .
			shape:      I,
			action:     RotateLeft,
			blockStack: [][]int{{9, 5}, {9, 6}, {7, 3}},
			setR: func(g *Tetris) {
				g.rotate(RotateRight)
				g.rotate(RotateRight)
			},
			wantX: 4,
			wantY: 8,
		},
		{
			name: "I tetromino, case 2>R, test 5 (-2, 1)",
			// .	0 1 2 3 4 5 6 7 8 9
			// 10	. . . . . . . . . .
			// 9	. . . . . X X . . .
			// 8	. . . O O O O . . .
			// 7    . . . X . . X . . .
			shape:      I,
			action:     RotateLeft,
			blockStack: [][]int{{9, 5}, {9, 6}, {7, 3}, {7, 6}},
			setR: func(g *Tetris) {
				g.rotate(RotateRight)
				g.rotate(RotateRight)
			},
			wantX: 1,
			wantY: 11,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tetris := NewTestTetris(tt.shape)
			tetris.Tetromino.Y = 10
			if tt.setR != nil {
				tt.setR(tetris)
			}
			if tt.blockStack != nil {
				for _, v := range tt.blockStack {
					tetris.Stack[v[0]][v[1]] = J
				}
			}
			tetris.rotate(tt.action)
			if tt.wantX != tetris.Tetromino.X {
				t.Errorf("wanted X to be %d, got %d", tt.wantX, tetris.Tetromino.X)
			}
			if tt.wantY != tetris.Tetromino.Y {
				t.Errorf("wanted Y to be %d, got %d", tt.wantY, tetris.Tetromino.Y)
			}
		})
	}
}

func TestToStack(t *testing.T) {
	tetris := NewTestTetris(J)
	tetris.toStack()
	wantStack := emptyStack()
	wantStack[19][3] = J
	wantStack[18][3] = J
	wantStack[18][4] = J
	wantStack[18][5] = J

	if !reflect.DeepEqual(tetris.Stack, wantStack) {
		t.Errorf("wanted %v, got %v", wantStack, tetris.Stack)
	}
	if tetris.Tetromino != nil {
		t.Errorf("wanted Tetromino to be nil, got %v", tetris.Tetromino)
	}
}

func TestRead(t *testing.T) {
	tetris := NewTestTetris(J)
	tetris.action(MoveDown)
	if reflect.DeepEqual(tetris, tetris.read()) {
		t.Errorf("tetris and tetris.read() content should be equal. wanted %v, got %v", tetris, tetris.read())
	}
	if tetris == tetris.read() {
		t.Errorf("tetris and tetris.read() pointers should be different. wanted %p, got %p", tetris, tetris.read())
	}
}

func TestRandomBag(t *testing.T) {
	t.Run("bag should contain 7 elements. after drawing it should contain one less", func(t *testing.T) {
		t.Parallel()
		bag := newBag()
		bag.draw()
		if len(bag.bag) != 6 {
			t.Errorf("wanted bag to have 6 pieces, got %d", len(bag.bag))
		}
	})

	t.Run("first draw should always be I, J, L or T", func(t *testing.T) {
		t.Parallel()
		for range 10 {
			go func() {
				bag := newBag()
				tetromino := bag.draw()
				if tetromino.Shape == O || tetromino.Shape == Z || tetromino.Shape == S {
					t.Errorf("wanted I, J, L, or T, got %v", tetromino.Shape)
				}
			}()
		}
	})

	t.Run("after drawing 7 tetrominos the bag should empty. next draw whould replenish it", func(t *testing.T) {
		t.Parallel()
		bag := newBag()
		for range 7 {
			bag.draw()
		}
		if len(bag.bag) != 0 {
			t.Errorf("wanted bag to be empty, got %d pieces", len(bag.bag))
		}
		bag.draw()
		if len(bag.bag) != 6 {
			t.Errorf("wanted bag to have 6 pieces, got %d", len(bag.bag))
		}
	})
}

func TestSetLevel(t *testing.T) {
	tests := []struct {
		lines, wantLevel int
	}{
		{1, 1},
		{9, 1},
		{10, 2},
		{12, 2},
		{20, 3},
		{94, 10},
		{100, 11},
		{209, 21},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("for %d lines should have level %d", tt.lines, tt.wantLevel), func(t *testing.T) {
			tetris := newTetris()
			tetris.LinesClear = tt.lines
			tetris.setLevel()
			if tetris.Level != tt.wantLevel {
				t.Errorf("wanted level %d, got %d", tt.wantLevel, tetris.Level)
			}
		})
	}

	t.Run("set level is not overriden until lines > level", func(t *testing.T) {
		tetris := newTetris()
		tetris.Level = 5
		tetris.LinesClear = 1
		tetris.setLevel()
		if tetris.Level != 5 {
			t.Errorf("wanted level 5, got %d", tetris.Level)
		}
		tetris.LinesClear = 50
		tetris.setLevel()
		if tetris.Level != 6 {
			t.Errorf("wanted level 6, got %d", tetris.Level)
		}
	})
}

func TestSetTetromino(t *testing.T) {
	t.Run("first time it populates current and next tetromino", func(t *testing.T) {
		tetris := newTetris()
		tetris.setTetromino()
		if tetris.Tetromino == nil || tetris.NexTetromino == nil {
			t.Errorf("want Tetromino and NextTetromino to not be nil, got: %v, %v", tetris.Tetromino, tetris.NexTetromino)
		}
	})
	t.Run("after tetromino has been transferred to the stack, moves next tetromino to current", func(t *testing.T) {
		tetris := newTetris()
		tetris.setTetromino()
		tetris.action(MoveDown)
		tetris.toStack()
		wantShape := tetris.NexTetromino.Shape
		tetris.setTetromino()
		if tetris.Tetromino.Shape != wantShape {
			t.Errorf("wanted current tetromino to have shape %v, got %v", wantShape, tetris.Tetromino.Shape)
		}
	})
}

func TestIsGameOver(t *testing.T) {
	tetris := NewTestTetris(J)
	if tetris.isGameOver() {
		t.Error("expected isGameOver() to be false")
	}
	if tetris.GameOver {
		t.Error("expected GameOver to be false")
	}
	tetris.Stack[19][3] = J
	if !tetris.isGameOver() {
		t.Error("expected isGameOver() to be true")
	}
	if !tetris.GameOver {
		t.Error("expected GameOver to be true")
	}
}
