package client

import (
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/alwedo/tetris/pb"
	"github.com/alwedo/tetris/tetris"
	approvals "github.com/approvals/go-approval-tests"
	"google.golang.org/protobuf/proto"
)

func TestRender(t *testing.T) {
	tests := []struct {
		name string
		do   func(*render)
	}{
		{
			name: "single player with no data renders game frame",
			do:   func(r *render) { r.singlePlayer(nil) },
		},
		{
			name: "single player with data renders game",
			do:   func(r *render) { r.singlePlayer(tetris.NewTestTetris(tetris.T)) },
		},
		{
			name: "multiplayer with data renders game",
			do: func(r *render) {
				tts := tetris.NewTestTetris(tetris.T)
				r.multiPlayer(&mpData{
					remote: pb.GameMessage_builder{
						Stack:      stack2Proto(tts),
						LinesClear: proto.Int32(int32(tts.LinesClear)), //nolint:gosec
						Name:       proto.String("remote"),
					}.Build(),
					local: tts,
				})
			},
		},
		{
			name: "default lobby message",
			do:   func(r *render) { r.lobby(defaultLobby()) },
		},
		{
			name: "game over lobby message",
			do:   func(r *render) { r.lobby(gameOver()) },
		},
		{
			name: "you won lobby message",
			do:   func(r *render) { r.lobby(youWon()) },
		},
		{
			name: "waiting opponent lobby message",
			do:   func(r *render) { r.lobby(waitingOpponent()) },
		},
		{
			name: "waiting opponent error message",
			do:   func(r *render) { r.lobby(waitingOpponentError()) },
		},
		{
			name: "opponent left the game message",
			do:   func(r *render) { r.lobby(opponentLeft()) },
		},
		{
			name: "error lobby message",
			do:   func(r *render) { r.lobby(errorMessage()) },
		},
	}
	tmpl := loadTemplate()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			w := &strings.Builder{}
			r := &render{
				writer:       w,
				logger:       slog.Default(),
				template:     tmpl,
				templateData: &templateData{Name: "local"},
			}
			tt.do(r)
			approvals.VerifyString(t, w.String())
		})
	}
}

func TestLocalStack(t *testing.T) {
	td := &templateData{
		Local: tetris.NewTestTetris(tetris.J),
	}
	want := [20][10]string{}
	for y := range want {
		for x := range want[y] {
			want[y][x] = "  "
		}
	}
	blueCell := "\x1b[7m\x1b[34m[]\x1b[0m"
	want[0][3] = blueCell
	want[1][3] = blueCell
	want[1][4] = blueCell
	want[1][5] = blueCell
	want[19][3] = "[]"
	want[18][3] = "[]"
	want[19][4] = "[]"
	want[19][5] = "[]"
	got := localStack(td)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}

	t.Run("localStack with nil tetris returns emtpy spaces", func(t *testing.T) {
		want := [20][10]string{}
		for y := range 20 {
			for x := range 10 {
				want[y][x] = "  "
			}
		}
		got := localStack(nil)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("want %v, got %v", want, got)
		}
	})
}

func TestRemoteStack(t *testing.T) {
	td := &templateData{
		Remote: pb.GameMessage_builder{
			Stack: stack2Proto(tetris.NewTestTetris(tetris.J)),
		}.Build(),
	}
	want := [20][10]string{}
	for y := range want {
		for x := range want[y] {
			want[y][x] = "  "
		}
	}
	blueCell := "\x1b[7m\x1b[34m[]\x1b[0m"
	want[0][3] = blueCell
	want[1][3] = blueCell
	want[1][4] = blueCell
	want[1][5] = blueCell
	got := remoteStack(td)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}

	t.Run("remoteStack with nil tetris returns emtpy spaces", func(t *testing.T) {
		want := [20][10]string{}
		for y := range 20 {
			for x := range 10 {
				want[y][x] = "  "
			}
		}
		got := remoteStack(nil)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("want %v, got %v", want, got)
		}
	})
}

func TestNextPiece(t *testing.T) {
	tests := []struct {
		shape tetris.Shape
		want  []string
	}{
		{tetris.J, []string{"\x1b[7m\x1b[34m[]\x1b[0m      ", "\x1b[7m\x1b[34m[]\x1b[0m\x1b[7m\x1b[34m[]\x1b[0m\x1b[7m\x1b[34m[]\x1b[0m  "}},
		{tetris.O, []string{"\x1b[7m\x1b[33m[]\x1b[0m\x1b[7m\x1b[33m[]\x1b[0m    ", "\x1b[7m\x1b[33m[]\x1b[0m\x1b[7m\x1b[33m[]\x1b[0m    "}},
		{tetris.I, []string{"        ", "\x1b[7m\x1b[36m[]\x1b[0m\x1b[7m\x1b[36m[]\x1b[0m\x1b[7m\x1b[36m[]\x1b[0m\x1b[7m\x1b[36m[]\x1b[0m"}},
	}
	for _, tt := range tests {
		t.Run(string(tt.shape), func(t *testing.T) {
			td := &templateData{Local: tetris.NewTestTetris(tt.shape)}
			got := nextPiece(td)
			if !reflect.DeepEqual(tt.want, got) {
				t.Errorf("want %v, got %v", tt.want, got)
			}
		})
	}
	t.Run("nextPiece with nil tetris returns emtpy spaces", func(t *testing.T) {
		want := []string{"        ", "        "}
		got := nextPiece(nil)
		if !reflect.DeepEqual(got, want) {
			t.Errorf("want %v, got %v", want, got)
		}
	})
}

func TestStack2Proto(t *testing.T) {
	got := stack2Proto(tetris.NewTestTetris(tetris.J))
	want := pb.Stack_builder{Rows: make([]*pb.Row, 20)}.Build()

	for i := range want.GetRows() {
		want.GetRows()[i] = pb.Row_builder{
			Cells: make([]string, 10),
		}.Build()
	}
	want.GetRows()[19].GetCells()[3] = "J"
	want.GetRows()[18].GetCells()[3] = "J"
	want.GetRows()[18].GetCells()[4] = "J"
	want.GetRows()[18].GetCells()[5] = "J"

	if !reflect.DeepEqual(want, got) {
		t.Errorf("want %v, got %v", want, got)
	}
}
