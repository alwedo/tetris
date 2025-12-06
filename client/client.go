package client

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/alwedo/tetris/pb"
	"github.com/alwedo/tetris/tetris"
	"github.com/eiannone/keyboard"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

type clientState int

const (
	lobby clientState = iota
	waiting
	playing

	serverPort = ":9000"
)

type state struct {
	current clientState
	mu      sync.Mutex
}

func (s *state) get() clientState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.current
}

func (s *state) set(c clientState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = c
}

type tetrisGame interface {
	Start()
	GetUpdate() <-chan *tetris.Tetris
	Action(tetris.Action)
	Stop()
	RemoteLines(i int32)
}

type renderer interface {
	singlePlayer(*tetris.Tetris)
	multiPlayer(*mpData)
	lobby(msgSetter)
}

type Client struct {
	tetris  tetrisGame
	render  renderer
	options *Options
	logger  *slog.Logger
	kbCh    <-chan keyboard.KeyEvent
	state   *state
}

type Options struct {
	NoGhost bool
	Address string
	Name    string
}

func New(l *slog.Logger, o *Options) (*Client, error) {
	kb, err := keyboard.GetKeys(20)
	if err != nil {
		return nil, fmt.Errorf("failed to open keyboard: %w", err)
	}
	return &Client{
		tetris:  tetris.NewGame(),
		render:  newRender(l, o.NoGhost, o.Name),
		options: o,
		logger:  l,
		kbCh:    kb,
		state:   &state{current: lobby},
	}, nil
}

func (c *Client) Start() {
	c.render.singlePlayer(nil)
	c.render.lobby(defaultLobby())
	var wg sync.WaitGroup
	wg.Add(1)
	go c.listenKB(&wg)
	wg.Wait()
}

func (c *Client) listenKB(wg *sync.WaitGroup) {
	defer wg.Done()
	var ctx context.Context
	var cancel context.CancelFunc
	for {
		event, ok := <-c.kbCh
		if !ok {
			c.logger.Error("Keyboard events channel closed unexpectedly")
			return
		}
		if event.Err != nil {
			c.logger.Error("keysEvents error", slog.String("error", event.Err.Error()))
			return
		}
		if event.Key == keyboard.KeyCtrlC {
			return
		}
		switch c.state.get() {
		case lobby:
			switch event.Rune {
			case 'p':
				go c.listenTetris()
				c.state.set(playing)
			case 'o':
				ctx, cancel = context.WithCancel(context.Background())
				defer cancel()
				go c.listenOnlineTetris(ctx)
				c.state.set(waiting)
			case 'q':
				return
			default:
				continue
			}
		case waiting:
			switch event.Rune {
			case 'c':
				cancel()
				c.render.lobby(defaultLobby())
			default:
				continue
			}
		case playing:
			var a tetris.Action
			switch {
			case event.Key == keyboard.KeyArrowDown || event.Rune == 's':
				a = tetris.MoveDown
			case event.Key == keyboard.KeyArrowLeft || event.Rune == 'a':
				a = tetris.MoveLeft
			case event.Key == keyboard.KeyArrowRight || event.Rune == 'd':
				a = tetris.MoveRight
			case event.Key == keyboard.KeyArrowUp || event.Rune == 'e':
				a = tetris.RotateRight
			case event.Rune == 'q':
				a = tetris.RotateLeft
			case event.Key == keyboard.KeySpace:
				a = tetris.DropDown
			}
			c.tetris.Action(a)
		}
	}
}

func (c *Client) listenTetris() {
	go c.tetris.Start()
	for u := range c.tetris.GetUpdate() {
		c.render.singlePlayer(u)
		if u.GameOver {
			c.state.set(lobby)
			c.render.lobby(gameOver())
			return
		}
	}
}

func (c *Client) listenOnlineTetris(ctx context.Context) {
	defer func() {
		c.state.set(lobby)
		c.tetris.Stop()
	}()

	// Start connection
	conn, err := grpc.NewClient(c.options.Address+serverPort, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		c.logger.Error("unable to create gRPC client", slog.String("error", err.Error()))
		c.render.lobby(errorMessage())
		return
	}
	defer func() {
		if err := conn.Close(); err != nil {
			c.logger.Error("unable to close gRPC client", slog.String("error", err.Error()))
		}
	}()
	stream, err := pb.NewTetrisServiceClient(conn).PlayTetris(ctx)
	if err != nil {
		c.logger.Error("unable to create gRPC PlayTetris stream", slog.String("error", err.Error()))
		c.render.lobby(errorMessage())
		return
	}
	defer stream.CloseSend() //nolint: errcheck
	c.render.lobby(waitingOpponent())

	// Set receiver channel
	rcvCh := make(chan *pb.GameMessage)
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		defer func() {
			cancel()
			close(rcvCh)
		}()
		for {
			rcv, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					c.logger.Debug("stream.Recv() closed with EOF", slog.String("msg", err.Error()))
					return
				}
				st, ok := status.FromError(err)
				if ok && st.Code() == codes.Canceled { //nolint: gocritic
					c.logger.Debug("stream.Recv() closed with Cancel", slog.String("msg", st.Message()))
				} else if ok && st.Code() == codes.DeadlineExceeded {
					c.logger.Debug("stream.Recv() closed with DeadlineExceeded", slog.String("msg", st.Message()))
					c.render.lobby(waitingOpponentError())
				} else {
					c.logger.Error("stream.Recv() unable to receive message", slog.String("error", err.Error()))
					c.render.lobby(errorMessage())
				}
				return
			}
			rcvCh <- rcv
		}
	}()

	// Send initial message, wait for game to start.
	if err := stream.Send(pb.GameMessage_builder{Name: proto.String(c.options.Name)}.Build()); err != nil {
		c.logger.Error("unable to send initial message", slog.String("error", err.Error()))
		return
	}
	c.render.multiPlayer(&mpData{remote: &pb.GameMessage{}})
start:
	for {
		select {
		case rcv := <-rcvCh:
			if rcv.GetIsStarted() {
				break start
			}
		case <-ctx.Done():
			c.logger.Debug("start for loop ctx.Done() was closed")
			return
		}
	}

	// start game
	c.state.set(playing)
	go c.tetris.Start()

	for {
		select {
		case lu, ok := <-c.tetris.GetUpdate():
			if !ok {
				c.logger.Error("listenOnline tetris update channel closed unexpectedly")
				return
			}
			c.render.multiPlayer(&mpData{local: lu})
			if err := stream.Send(pb.GameMessage_builder{
				Name:       proto.String(c.options.Name),
				IsGameOver: proto.Bool(lu.GameOver),
				IsStarted:  proto.Bool(true),
				LinesClear: proto.Int32(int32(lu.LinesClear)), // nolint:gosec
				Stack:      stack2Proto(lu),
			}.Build()); err != nil {
				if err == io.EOF {
					c.logger.Debug("send() opponent closed the game with EOF", slog.String("debug", err.Error()))
					return
				}
				st, ok := status.FromError(err)
				if ok && st.Code() == codes.Canceled {
					c.logger.Debug("send() opponent closed the game with Cancel", slog.String("debug", err.Error()))
					return
				}
				c.logger.Error("send() unable to send message", slog.String("error", err.Error()))
				return
			}
			if lu.GameOver {
				c.logger.Debug("listenOnline closed through local.GameOver")
				c.render.lobby(gameOver())
				return
			}
		case ru, ok := <-rcvCh:
			if !ok {
				c.logger.Error("listenOnline remote update channel closed unexpectedly")
				return
			}
			c.tetris.RemoteLines(ru.GetLinesClear())
			c.render.multiPlayer(&mpData{remote: ru})
			if ru.GetIsGameOver() {
				c.logger.Debug("listenOnline closed through remote.GetIsGameOver()")
				c.render.lobby(youWon())
				return
			}
		case <-ctx.Done():
			c.logger.Debug("listenOnline ctx.Done() was closed")
			c.render.lobby(opponentLeft())
			return
		}
	}
}
