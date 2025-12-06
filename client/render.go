package client

import (
	_ "embed"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"text/template"

	"github.com/alwedo/tetris/pb"
	"github.com/alwedo/tetris/tetris"
)

type msgSetter func(io.Writer)

const (
	// ASCII colors.
	Cyan    = "36"
	Blue    = "34"
	Orange  = "38;5;214"
	Yellow  = "33"
	Green   = "32"
	Red     = "31"
	Magenta = "35"

	resetPos = "\033[H" // Reset cursor position to 0,0
)

var (
	//go:embed "layout_sp.tmpl"
	layoutSP string
	//go:embed "layout_mp.tmpl"
	layoutMP string
)

var colorMap = map[tetris.Shape]string{
	tetris.I: Cyan,
	tetris.J: Blue,
	tetris.L: Orange,
	tetris.O: Yellow,
	tetris.S: Green,
	tetris.Z: Red,
	tetris.T: Magenta,
}

type templateData struct {
	Local   *tetris.Tetris
	Remote  *pb.GameMessage
	Name    string
	NoGhost bool
}

type render struct {
	writer   io.Writer
	logger   *slog.Logger
	template *template.Template
	*templateData
}

func newRender(l *slog.Logger, ng bool, name string) *render {
	return &render{
		writer:   os.Stdout,
		logger:   l,
		template: loadTemplate(),
		templateData: &templateData{
			Name:    name,
			NoGhost: ng,
		},
	}
}

func (r *render) lobby(msg msgSetter) {
	fmt.Fprint(r.writer, "\033[10;9H+--------------------------------------+\033[11;9H|                                      |\033[12;9H|                                      |\033[13;9H|                                      |\033[14;9H+--------------------------------------+")
	msg(r.writer)
}

func (r *render) singlePlayer(t *tetris.Tetris) {
	if r.Remote != nil {
		// ensures no remote data is in templateData from previous games
		r.Remote = nil
	}
	r.Local = t
	if err := r.template.ExecuteTemplate(r.writer, "layoutSP", r.templateData); err != nil {
		r.logger.Error("unable to execute template", slog.String("error", err.Error()))
	}
}

type mpData struct {
	remote *pb.GameMessage
	local  *tetris.Tetris
}

func (r *render) multiPlayer(mpd *mpData) {
	if mpd != nil {
		if mpd.remote != nil {
			r.Remote = mpd.remote
		}
		if mpd.local != nil {
			r.Local = mpd.local
		}
	}
	if err := r.template.ExecuteTemplate(r.writer, "layoutMP", r.templateData); err != nil {
		r.logger.Error("unable to execute template", slog.String("error", err.Error()))
	}
}

func loadTemplate() *template.Template {
	funcMap := template.FuncMap{
		"localStack":       localStack,
		"remoteStack":      remoteStack,
		"nextPiece":        nextPiece,
		"remoteName":       remoteName,
		"remoteLinesClear": remoteLinesClear,
	}

	// we use the console raw so new lines don't automatically transform into carriage return
	// to fix that we add a carriage return to every new line in the layout.
	layoutSP = resetPos + layoutSP
	layoutSP = strings.ReplaceAll(layoutSP, "\n", "\r\n")
	layoutSP = strings.ReplaceAll(layoutSP, "Terminal Tetris", "\033[1mTerminal Tetris\033[0m")

	layoutMP = resetPos + layoutMP
	layoutMP = strings.ReplaceAll(layoutMP, "\n", "\r\n")
	layoutMP = strings.ReplaceAll(layoutMP, "Terminal Tetris", "\033[1mTerminal Tetris\033[0m")

	tmpl := template.New("").Funcs(funcMap)
	tmpl = template.Must(tmpl.New("layoutSP").Parse(layoutSP))
	tmpl = template.Must(tmpl.New("layoutMP").Parse(layoutMP))

	return tmpl
}

func localStack(t *templateData) [20][10]string {
	rendered := [20][10]string{}
	for y := range 20 {
		for x := range 10 {
			out := "  "
			if t != nil && t.Local != nil {
				v := t.Local.Stack[y][x]
				c, ok := colorMap[v]
				if ok {
					out = fmt.Sprintf("\x1b[7m\x1b[%sm[]\x1b[0m", c)
				}
			}
			// we deduct 19 from the 'y' index because the range over function
			// in the tempalate can only range over from 0 upwards. we do the
			// same again when rendering the current tetromino to the screen.
			rendered[19-y][x] = out
		}
	}

	// renders the current tetromino if exist
	if t != nil && t.Local != nil && t.Local.Tetromino != nil {
		for iy, y := range t.Local.Tetromino.Grid {
			for ix, x := range y {
				if x {
					if !t.NoGhost {
						rendered[19-t.Local.Tetromino.GhostY+iy][t.Local.Tetromino.X+ix] = "[]"
					}
					rendered[19-t.Local.Tetromino.Y+iy][t.Local.Tetromino.X+ix] = fmt.Sprintf("\x1b[7m\x1b[%sm[]\x1b[0m", colorMap[t.Local.Tetromino.Shape])
				}
			}
		}
	}
	return rendered
}

func remoteStack(t *templateData) [20][10]string {
	rendered := [20][10]string{}
	for y := range 20 {
		for x := range 10 {
			out := "  "
			if t != nil && t.Remote != nil {
				c, ok := colorMap[tetris.Shape(t.Remote.GetStack().GetRows()[y].GetCells()[x])]
				if ok {
					out = fmt.Sprintf("\x1b[7m\x1b[%sm[]\x1b[0m", c)
				}
			}
			// we deduct 19 from the 'y' index because the range over function
			// in the tempalate can only range over from 0 upwards. we do the
			// same again when rendering the current tetromino to the screen.
			rendered[19-y][x] = out
		}
	}
	return rendered
}

func nextPiece(t *templateData) []string {
	var rendered []string
	for i := range 2 {
		row := []string{"  ", "  ", "  ", "  "}
		if t != nil && t.Local != nil {
			for iv, v := range t.Local.NexTetromino.Grid[i] {
				if v {
					row[iv] = fmt.Sprintf("\x1b[7m\x1b[%sm[]\x1b[0m", colorMap[t.Local.NexTetromino.Shape])
				}
			}
		}
		rendered = append(rendered, strings.Join(row, ""))
	}
	return rendered
}

func stack2Proto(t *tetris.Tetris) *pb.Stack {
	rendered := pb.Stack_builder{Rows: make([]*pb.Row, 20)}.Build()

	for i := range rendered.GetRows() {
		rendered.GetRows()[i] = pb.Row_builder{
			Cells: make([]string, 10),
		}.Build()
	}

	for iy, y := range t.Stack {
		for ix, x := range y {
			if x != tetris.Shape("") {
				rendered.GetRows()[iy].GetCells()[ix] = string(x)
			}
		}
	}

	// renders the current tetromino if exist
	if t.Tetromino != nil {
		for iy, y := range t.Tetromino.Grid {
			for ix, x := range y {
				if x {
					rendered.GetRows()[t.Tetromino.Y-iy].GetCells()[t.Tetromino.X+ix] = string(t.Tetromino.Shape)
				}
			}
		}
	}
	return rendered
}

func remoteName(t *templateData) string { return t.Remote.GetName() }

func remoteLinesClear(t *templateData) int32 { return t.Remote.GetLinesClear() }

func defaultLobby() msgSetter {
	return func(w io.Writer) {
		fmt.Fprint(w, "\033[11;9H|      Welcome to Terminal Tetris      |\033[13;9H|      (p)lay   (o)nline   (q)uit      |")
	}
}

func gameOver() msgSetter {
	return func(w io.Writer) {
		fmt.Fprint(w, "\033[11;9H|             Game Over :)             |\033[13;9H|      (p)lay   (o)nline   (q)uit      |")
	}
}

func youWon() msgSetter {
	return func(w io.Writer) {
		fmt.Fprint(w, "\033[11;9H|              You Won :)              |\033[13;9H|      (p)lay   (o)nline   (q)uit      |")
	}
}

func waitingOpponent() msgSetter {
	return func(w io.Writer) {
		fmt.Fprint(w, "\033[11;9H|       waiting for opponent...        |\033[13;9H|               (c)ancel               |")
	}
}

func waitingOpponentError() msgSetter {
	return func(w io.Writer) {
		fmt.Fprint(w, "\033[11;9H|   there is no one to play with :(    |\033[13;9H|      (p)lay   (o)nline   (q)uit      |")
	}
}

func opponentLeft() msgSetter {
	return func(w io.Writer) {
		fmt.Fprint(w, "\033[11;9H|  opponent left the game ¯\\_(ツ)_/¯   |\033[13;9H|      (p)lay   (o)nline   (q)uit      |")
	}
}

func errorMessage() msgSetter {
	return func(w io.Writer) {
		fmt.Fprint(w, "\033[11;9H|      oops! something went wrong      |\033[13;9H|      (p)lay   (o)nline   (q)uit      |")
	}
}
