package client

import (
	"context"
	"image/color"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	tetris "github.com/alwedo/tetris/tetrisv2"
)

type GameModel struct {
	gameInstance *tetris.Game
	gameState    tetris.GameMessage
	ctx          context.Context
	cancelCtx    context.CancelFunc
}

func NewGameModel() GameModel {
	return GameModel{}
}

func (m *GameModel) Init() tea.Cmd {
	ctx, cancel := context.WithCancel(context.Background())
	m.cancelCtx = cancel
	m.ctx = ctx
	m.gameInstance = tetris.Start(ctx)

	return m.listenToGameUpdates(ctx)
}

func (m *GameModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case AnimationMessage:
		if msg.frames == 0 {
			return m, m.listenToGameUpdates(m.ctx)
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
				complete[v] = m.gameState.Tetris.Stack[v]
			}
			return m, func() tea.Msg {
				return AnimationMessage{
					frames:        8,
					completedRows: complete,
				}
			}
		}
		return m, m.listenToGameUpdates(m.ctx)

	case tea.KeyPressMsg:
		switch msg.String() {
		// TODO: change to esc
		case "q":
			m.cancelCtx()
			return m, func() tea.Msg {
				return BackToLobbyMsg{Reason: "user_quit"}
			}

		case "left", "a":
			m.gameInstance.Do(tetris.MoveLeft())
			return m, nil

		case "right", "d":
			m.gameInstance.Do(tetris.MoveRight())
			return m, nil

		case "down", "j", "s":
			m.gameInstance.Do(tetris.MoveDown())
			return m, nil

		case "space":
			m.gameInstance.Do(tetris.DropDown())
			return m, nil

		case "z":
			m.gameInstance.Do(tetris.RotateLeft())
			return m, nil

		case "x", "up", "k":
			m.gameInstance.Do(tetris.RotateRight())
			return m, nil
		}
	}

	return m, nil
}

func (m *GameModel) View() tea.View {
	if m.gameState.Tetris.Tetromino == nil {
		return tea.NewView(lipgloss.NewStyle().
			Width(22).
			Height(22).
			Border(lipgloss.NormalBorder()).
			Render(),
		)
	}

	stack := make([][]string, 20)
	for i := range stack {
		stack[i] = make([]string, 10)
	}

	for y := range 20 {
		for x := range 10 {
			out := "  "

			v := m.gameState.Tetris.Stack[y][x]
			c, ok := colorMap[v]
			if ok {
				out = lipgloss.NewStyle().Background(c).Render("[]")
			}
			stack[19-y][x] = out
		}
	}

	for iy, y := range m.gameState.Tetris.Tetromino.Grid {
		for ix, x := range y {
			if x {
				// if !t.NoGhost { // TODO: enable ghostpiece
				stack[19-m.gameState.Tetris.Tetromino.GhostY+iy][m.gameState.Tetris.Tetromino.X+ix] = "[]"
				// }
				stack[19-m.gameState.Tetris.Tetromino.Y+iy][m.gameState.Tetris.Tetromino.X+ix] = lipgloss.NewStyle().Background(colorMap[m.gameState.Tetris.Tetromino.Shape]).Render("[]")
			}
		}
	}

	rows := []string{}
	for _, row := range stack {
		rows = append(rows, strings.Join(row, ""))

	}

	return tea.NewView(
		lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			Render(lipgloss.JoinVertical(lipgloss.Center, rows...)),
	)
}

func (m *GameModel) listenToGameUpdates(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		for {
			select {
			case msg, ok := <-m.gameInstance.GameMessageCh:
				if !ok {
					return BackToLobbyMsg{Reason: "game_ended"}
				}
				return msg

			case <-ctx.Done():
				return BackToLobbyMsg{Reason: "context_cancelled"}
			}
		}
	}
}

type AnimationMessage struct {
	frames        int
	completedRows map[int][]tetris.Shape
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
