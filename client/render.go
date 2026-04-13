package client

import (
	"fmt"
	"image/color"
	"slices"
	"strconv"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alwedo/tetris"
	"github.com/alwedo/tetris/pb"
)

const (
	gameName            = "Terminal Tetris"
	centerPanelMinWidth = 22
	centerPanelHeight   = 22
)

var colorMap = map[tetris.Shape]color.Color{
	tetris.I: lipgloss.Color("#01EDFA"), // cyan
	tetris.J: lipgloss.Color("#0077D3"), // blue
	tetris.L: lipgloss.Color("#FFC82E"), // orange
	tetris.O: lipgloss.Color("#FEFB34"), // yellow
	tetris.S: lipgloss.Color("#53DA3F"), // green
	tetris.Z: lipgloss.Color("#EA141C"), // red
	tetris.T: lipgloss.Color("#DD0AB2"), // magenta
}

// renderStack returns a rounded background with the stack and tetrominos rendered
func renderStack(t tetris.Tetris) string {
	stack := make([][]string, 20)
	for i := range stack {
		stack[i] = []string{"  ", "  ", "  ", "  ", "  ", "  ", "  ", "  ", "  ", "  "}
	}

	if len(t.Stack) > 0 {
		for y := range 20 {
			for x := range 10 {
				c, ok := colorMap[t.Stack[y][x]]
				if ok {
					stack[19-y][x] = lipgloss.NewStyle().Background(c).Render("[]")
				}
			}
		}
	}

	if t.Tetromino != nil {
		for iy, y := range t.Tetromino.Grid {
			for ix, x := range y {
				if x {
					// Ghost piece
					stack[19-t.Tetromino.GhostY+iy][t.Tetromino.X+ix] = "[]"
					// Actual piece
					stack[19-t.Tetromino.Y+iy][t.Tetromino.X+ix] = lipgloss.NewStyle().Background(colorMap[t.Tetromino.Shape]).Render("[]")
				}
			}
		}
	}

	rows := []string{}
	for _, row := range stack {
		rows = append(rows, strings.Join(row, ""))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Render(lipgloss.JoinVertical(lipgloss.Center, rows...))
}

// renderNextPiece renders the next tetromino preview
func renderNextPiece(t tetris.Tetris) string {
	var rendered []string
	for i := range 2 {
		row := []string{"  ", "  ", "  ", "  "}
		if t.NextTetromino != nil && len(t.NextTetromino.Grid) > 0 {
			for iv, v := range t.NextTetromino.Grid[i] {
				if v {
					row[iv] = lipgloss.NewStyle().Background(colorMap[t.NextTetromino.Shape]).Render("[]")
				}
			}
		}
		rendered = append(rendered, strings.Join(row, ""))
	}

	return lipgloss.JoinVertical(lipgloss.Center, rendered...)
}

// renderRemoteStack renders opponent's tetris board from protobuf
func renderRemoteStack(t *pb.GameMessage) string {
	emptyRow := []string{"  ", "  ", "  ", "  ", "  ", "  ", "  ", "  ", "  ", "  "}
	stack := make([][]string, 20)
	for i := range stack {
		stack[i] = slices.Clone(emptyRow)
	}

	if t != nil && t.GetStack() != nil && len(t.GetStack().GetRows()) == 20 {
		for y := range 20 {
			if len(t.GetStack().GetRows()[y].GetCells()) == 10 {
				for x := range 10 {
					c, ok := colorMap[tetris.Shape(t.GetStack().GetRows()[y].GetCells()[x])]
					if ok {
						stack[19-y][x] = lipgloss.NewStyle().Background(c).Render("[]")
					}
				}
			}
		}
	}

	rows := []string{}
	for _, row := range stack {
		rows = append(rows, strings.Join(row, ""))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Render(lipgloss.JoinVertical(lipgloss.Center, rows...))
}

func renderCenterPanel(local tetris.Tetris, name string, remote *pb.GameMessage) string {
	var stats string

	switch remote {
	case nil:
		statKeys := lipgloss.NewStyle().Align(lipgloss.Right).Render("Level: \nLines: \nNext: ")
		statValues := lipgloss.NewStyle().Render(
			lipgloss.JoinVertical(
				lipgloss.Left,
				boldStyle.Render(strconv.Itoa(local.Level)),
				boldStyle.Render(strconv.Itoa(local.Lines)),
				renderNextPiece(local),
			),
		)
		stats = lipgloss.JoinHorizontal(lipgloss.Top, statKeys, statValues)
	default:
		stats = lipgloss.JoinVertical(
			lipgloss.Center,
			// TODO: format this better
			fmt.Sprintf("%s vs %s", name, remote.GetName()),
			fmt.Sprintf("%d: Lines :%d", local.Lines, remote.GetLinesClear()),
			fmt.Sprintf("Next:\n%s", renderNextPiece(local)),
		)
	}

	w := centerPanelMinWidth
	sw := lipgloss.Width(stats)
	if sw > w {
		w = sw
	}
	title := boldStyle.
		Width(w).
		Align(lipgloss.Center).
		PaddingTop(1).
		Render(gameName)

	styleStats := lipgloss.NewStyle().
		Width(w).
		Align(lipgloss.Center).
		PaddingTop(2).
		Render(lipgloss.JoinVertical(lipgloss.Top, stats))

	return lipgloss.NewStyle().
		Height(centerPanelHeight).
		Render(lipgloss.JoinVertical(lipgloss.Top, title, styleStats))
}
