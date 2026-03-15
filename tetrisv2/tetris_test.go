package tetris

import (
	"fmt"
	"reflect"
	"testing"
)

func newTestTetris(shape Shape) *Tetris {
	t := newTetris()
	t.Tetromino = shapeMap[shape]()
	t.NexTetromino = shapeMap[shape]()
	t.Tetromino.GhostY = t.Tetromino.Y + t.dropDownDelta()
	return t
}

func TestStack(t *testing.T) {
	t.Run("New tetris starts with empty stack", func(t *testing.T) {
		tetris := newTestTetris(J)
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
			tetris := newTestTetris(J)
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
		command      Command
		updateStack  func(g *Tetris)
		wantGrid     [][]bool
		wantLocation []int // x, y
	}{
		{
			name:         "Move left unblocked",
			command:      MoveLeft(),
			wantLocation: []int{19, 2},
		},
		{
			name:    "Move left blocked",
			command: MoveLeft(),
			updateStack: func(g *Tetris) {
				g.Stack[18][2] = J
			},
			wantLocation: []int{19, 3},
		},
		{
			name:         "Move right unblocked",
			command:      MoveRight(),
			wantLocation: []int{19, 4},
		},
		{
			name:    "Move right blocked",
			command: MoveRight(),
			updateStack: func(g *Tetris) {
				g.Stack[18][6] = J
			},
			wantLocation: []int{19, 3},
		},
		{
			name:         "Move down unblocked",
			command:      MoveDown(),
			wantLocation: []int{18, 3},
		},
		{
			name:    "Move down blocked",
			command: MoveDown(),
			updateStack: func(g *Tetris) {
				g.Stack[17][3] = J
			},
			wantLocation: []int{19, 3},
		},
		{
			name:         "Drop moves down until blocked",
			command:      DropDown(),
			wantLocation: []int{1, 3},
		},
		{
			name:         "Rotate right when unblocked",
			command:      RotateRight(),
			wantLocation: []int{19, 3},
			wantGrid: [][]bool{
				{false, true, true},
				{false, true, false},
				{false, true, false},
			},
		},
		{
			name:         "Rotate left when unblocked",
			command:      RotateLeft(),
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
			tetris := newTestTetris(J)
			if tt.updateStack != nil {
				tt.updateStack(tetris)
			}
			tt.command(tetris)
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
		command      Command
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
			command:    RotateRight(),
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
			command:    RotateRight(),
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
			command:    RotateRight(),
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
			command:    RotateRight(),
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
			shape:   I,
			command: RotateLeft(),
			setR: func(t *Tetris) {
				// for this case we put the tetromino against the left wall
				t.Tetromino.X = -2
				RotateRight()(t)
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
			shape:   I,
			command: RotateLeft(),
			setR: func(t *Tetris) {
				// for this case we put the tetromino against the right wall
				t.Tetromino.X = 7
				RotateRight()(t)
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
			command:    RotateLeft(),
			blockStack: [][]int{{9, 3}, {9, 6}},
			setR:       func(t *Tetris) { RotateRight()(t) },
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
			command:    RotateLeft(),
			blockStack: [][]int{{9, 3}, {9, 6}, {10, 6}},
			setR:       func(t *Tetris) { RotateRight()(t) },
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
			shape:   I,
			command: RotateRight(),
			setR: func(t *Tetris) {
				// for this case we put the tetromino against the right wall
				t.Tetromino.X = 7
				RotateRight()(t)
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
			shape:   I,
			command: RotateRight(),
			setR: func(t *Tetris) {
				// for this case we put the tetromino against the left wall
				t.Tetromino.X = -2
				RotateRight()(t)
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
			command:    RotateRight(),
			blockStack: [][]int{{8, 3}, {8, 6}},
			setR:       func(t *Tetris) { RotateRight()(t) },
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
			command:    RotateRight(),
			blockStack: [][]int{{8, 3}, {8, 6}, {10, 3}},
			setR:       func(t *Tetris) { RotateRight()(t) },
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
			command:    RotateLeft(),
			blockStack: [][]int{{9, 5}},
			setR: func(t *Tetris) {
				RotateRight()(t)
				RotateRight()(t)
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
			command:    RotateLeft(),
			blockStack: [][]int{{9, 5}, {9, 6}},
			setR: func(t *Tetris) {
				RotateRight()(t)
				RotateRight()(t)
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
			command:    RotateLeft(),
			blockStack: [][]int{{9, 5}, {9, 6}, {7, 3}},
			setR: func(t *Tetris) {
				RotateRight()(t)
				RotateRight()(t)
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
			command:    RotateLeft(),
			blockStack: [][]int{{9, 5}, {9, 6}, {7, 3}, {7, 6}},
			setR: func(t *Tetris) {
				RotateRight()(t)
				RotateRight()(t)
			},
			wantX: 1,
			wantY: 11,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tetris := newTestTetris(tt.shape)
			tetris.Tetromino.Y = 10
			if tt.setR != nil {
				tt.setR(tetris)
			}
			if tt.blockStack != nil {
				for _, v := range tt.blockStack {
					tetris.Stack[v[0]][v[1]] = J
				}
			}
			tt.command(tetris)
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
	tetris := newTestTetris(J)
	tetris.toStack()

	wantStack := [][]int{{19, 3}, {18, 3}, {18, 4}, {18, 5}}
	for _, ws := range wantStack {
		if tetris.Stack[ws[0]][ws[1]] != J {
			t.Errorf("wanted stack %v to be J, got %v", ws, tetris.Stack[ws[0]][ws[1]])
		}
	}
	if len(tetris.LinesClearedIndex) != 0 {
		t.Errorf("wanted empty LinesClearedIndex, got %d", len(tetris.LinesClearedIndex))
	}

	t.Run("toStack will fill LinesClearedIndex if any", func(t *testing.T) {
		tetris := newTetris()
		for i := range tetris.Stack[0] {
			tetris.Stack[0][i] = I
		}
		tetris.toStack()
		if len(tetris.LinesClearedIndex) != 1 {
			t.Errorf("expected 1 line to be cleared, got %d", len(tetris.LinesClearedIndex))
		}
	})
}

func TestFinishRound(t *testing.T) {
	t.Run("it rotates the tetrominoes", func(t *testing.T) {
		tetris := newTetris()
		wantTetrominoShape := tetris.NexTetromino.Shape
		tetris.finishRound()
		if tetris.Tetromino.Shape != wantTetrominoShape {
			t.Errorf("wanted current tetromino to be %s, got %s", wantTetrominoShape, tetris.Tetromino.Shape)
		}
	})

	t.Run("it removes completed lines from the stack", func(t *testing.T) {
		tetris := newTetris()
		index := 1
		tetris.LinesClearedIndex = []int{index}

		// set a complete line to be cleared
		for i := range tetris.Stack[index] {
			tetris.Stack[index][i] = I
		}

		tetris.finishRound()
		for i := range tetris.Stack[index] {
			if tetris.Stack[index][i] != "" {
				t.Errorf("wanted Stack[0][%d] to be empty, got %s", i, tetris.Stack[index][i])
			}
		}
	})

	t.Run("increases the number of lines and cleares LinesClearedIndex", func(t *testing.T) {
		tetris := newTetris()
		tetris.LinesClearedIndex = []int{1, 2}
		tetris.finishRound()
		if tetris.Lines != 2 {
			t.Errorf("wanted 2 lines cleared, got %d", tetris.Lines)
		}
		if tetris.LinesClearedIndex != nil {
			t.Errorf("wanted LinesClearedIndex to be nil, got %v", tetris.LinesClearedIndex)
		}
	})

	t.Run("it calculates new level", func(t *testing.T) {
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
				tetris.LinesClearedIndex = []int{1} // set level only happens when there are lines to be cleared.
				tetris.Lines = tt.lines - 1         // we remove one line to offset for the LinesClearedIndex above.
				tetris.finishRound()
				if tetris.Level != tt.wantLevel {
					t.Errorf("wanted level %d, got %d", tt.wantLevel, tetris.Level)
				}
			})
		}

		t.Run("set level is not overriden until lines > level", func(t *testing.T) {
			tetris := newTetris()
			tetris.LinesClearedIndex = []int{1}
			tetris.Level = 5
			tetris.finishRound()
			if tetris.Level != 5 {
				t.Errorf("wanted level 5, got %d", tetris.Level)
			}
			tetris.LinesClearedIndex = []int{1}
			tetris.Lines = 49
			tetris.finishRound()
			if tetris.Level != 6 {
				t.Errorf("wanted level 6, got %d", tetris.Level)
			}
		})
	})
}

func TestRead(t *testing.T) {
	tetris := newTestTetris(J)
	MoveDown()(tetris)
	if reflect.DeepEqual(tetris, tetris.read()) {
		t.Errorf("tetris and tetris.read() content should be equal. wanted %v, got %v", tetris, tetris.read())
	}
	got := tetris.read()
	if tetris.Tetromino == got.Tetromino {
		t.Errorf("tetrominos' pointers should be different. wanted %p, got %p", tetris.Tetromino, got.Tetromino)
	}
	if tetris.NexTetromino == got.NexTetromino {
		t.Errorf("next tetrominos' pointers should be different. wanted %p, got %p", tetris.NexTetromino, got.NexTetromino)
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
		DropDown()(tetris)
		tetris.toStack()
		wantShape := tetris.NexTetromino.Shape
		tetris.setTetromino()
		if tetris.Tetromino.Shape != wantShape {
			t.Errorf("wanted current tetromino to have shape %v, got %v", wantShape, tetris.Tetromino.Shape)
		}
		if tetris.GameOver {
			t.Errorf("wanted GameOver to be false, got %v", tetris.GameOver)
		}
	})
	t.Run("turns GameOver true if next tetromino has collision with stack", func(t *testing.T) {
		tetris := newTetris()
		tetris.toStack()
		tetris.setTetromino()
		if !tetris.GameOver {
			t.Errorf("wanted GameOver to be true, got %v", tetris.GameOver)
		}
	})
}
