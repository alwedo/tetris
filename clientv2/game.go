package client

import (
	"context"
	"fmt"
	"image/color"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/help"
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	tetris "github.com/alwedo/tetris/tetrisv2"
)

type GameModel struct {
	gameInstance *tetris.Game
	gameState    tetris.GameMessage
	ctx          context.Context
	cancelCtx    context.CancelFunc
	keys         gameKeyMap
	help         help.Model
}

func NewGameModel() GameModel {
	return GameModel{
		keys: gameKeys,
		help: help.New(),
	}
}

func (m *GameModel) Init() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelCtx = cancel
	m.ctx = ctx
	m.gameInstance = tetris.Start(ctx)
	m.help.SetWidth(22)

	return m.listenToGameUpdates(ctx)
}

func (m *GameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case AnimationMessage:
		if msg.frames == 0 {
			return m, m.listenToGameUpdates(m.ctx)
		}
		// we want the animation to skip rendering the tetromino
		if m.gameState.Tetris.Tetromino != nil {
			m.gameState.Tetris.Tetromino = nil
		}
		for k, v := range msg.completedRows {
			if msg.frames%2 == 0 {
				m.gameState.Tetris.Stack[k] = make([]tetris.Shape, 10)
			} else {
				m.gameState.Tetris.Stack[k] = v
			}
		}
		msg.frames--
		return m, tea.Tick(40*time.Millisecond, func(time.Time) tea.Msg {
			return msg
		})

	case tetris.GameMessage:
		m.gameState = msg
		if len(m.gameState.ClearedLines) > 0 {
			complete := make(map[int][]tetris.Shape)
			for _, v := range msg.ClearedLines {
				complete[v] = msg.Tetris.Stack[v]
			}
			return m, newAnimationMsg(complete)
		}
		return m, m.listenToGameUpdates(m.ctx)

	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Quit):
			m.cancelCtx()
			return m, func() tea.Msg {
				return BackToLobbyMsg{localGameState: m.gameState}
			}
		case key.Matches(msg, m.keys.MoveLeft):
			m.gameInstance.Do(tetris.MoveLeft())
		case key.Matches(msg, m.keys.MoveRight):
			m.gameInstance.Do(tetris.MoveRight())
		case key.Matches(msg, m.keys.MoveDown):
			m.gameInstance.Do(tetris.MoveDown())
		case key.Matches(msg, m.keys.DropDown):
			m.gameInstance.Do(tetris.DropDown())
		case key.Matches(msg, m.keys.RotateLeft):
			m.gameInstance.Do(tetris.RotateLeft())
		case key.Matches(msg, m.keys.RotateRight):
			m.gameInstance.Do(tetris.RotateRight())
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = !m.help.ShowAll
		}
	}

	return m, nil
}

var emptyRow = []string{"  ", "  ", "  ", "  ", "  ", "  ", "  ", "  ", "  ", "  "}

func (m *GameModel) View() tea.View {
	stack := renderStack(m.gameState.Tetris)

	gameName := lipgloss.NewStyle().Bold(true).Render(appName)
	nextPiece := renderNextPiece(m.gameState.Tetris)

	stats := lipgloss.NewStyle().Width(22).Align(lipgloss.Center).
		Border(lipgloss.RoundedBorder()).Render(lipgloss.JoinVertical(lipgloss.Center,
		gameName, fmt.Sprintf("Level: %d\nLines Cleared: %d\n", m.gameState.Tetris.Level, m.gameState.Tetris.Lines), nextPiece,
	))

	help := lipgloss.NewStyle().Width(22).Align(lipgloss.Center).
		Border(lipgloss.RoundedBorder()).
		Foreground(lipgloss.Color("#FF75B7")).Render(m.help.View(m.keys))
	centerPiece := lipgloss.JoinVertical(lipgloss.Center, stats, help)

	return tea.NewView(
		lipgloss.JoinHorizontal(lipgloss.Bottom, stack, centerPiece),
	)
}

func (m *GameModel) listenToGameUpdates(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		for {
			select {
			case msg, ok := <-m.gameInstance.GameMessageCh:
				if !ok {
					return BackToLobbyMsg{localGameState: m.gameState}
				}
				return msg

			case <-ctx.Done():
				return BackToLobbyMsg{localGameState: m.gameState}
			}
		}
	}
}

type AnimationMessage struct {
	frames        int
	completedRows map[int][]tetris.Shape
}

func newAnimationMsg(c map[int][]tetris.Shape) tea.Cmd {
	return func() tea.Msg {
		return AnimationMessage{
			frames:        8, // TODO: move to constant?
			completedRows: c,
		}
	}
}

var colorMap = map[tetris.Shape]color.Color{
	tetris.I: lipgloss.Color("#01EDFA"), // cyan
	tetris.J: lipgloss.Color("#0077D3"), // blue
	tetris.L: lipgloss.Color("#FFC82E"), // orange
	tetris.O: lipgloss.Color("#FEFB34"), // yellow
	tetris.S: lipgloss.Color("#53DA3F"), // green
	tetris.Z: lipgloss.Color("#EA141C"), // red
	tetris.T: lipgloss.Color("#DD0AB2"), // magenta
}

// renderStack returns a rounded background with the
// stack and tetrominos rendered. It returns an empty
// rounded stack of the right dimensions when t is empty.
func renderStack(t tetris.Tetris) string {
	stack := make([][]string, 20)
	for i := range stack {
		stack[i] = slices.Clone(emptyRow)
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
					// if !t.NoGhost { // TODO: enable ghostpiece
					stack[19-t.Tetromino.GhostY+iy][t.Tetromino.X+ix] = "[]"
					// }
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
