package server

import (
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"time"

	"github.com/alwedo/tetris/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

const (
	player1 = 1
	player2 = 2

	// Default timeout for waiting for opponent.
	defaultTimeOut = 30 * time.Second
)

type game struct {
	p1Ch, p2Ch chan *pb.GameMessage
	p1, p2     bool
	closed     bool
	mu         sync.Mutex
}

func newGame() *game {
	return &game{
		p1Ch: make(chan *pb.GameMessage),
		p2Ch: make(chan *pb.GameMessage),
	}
}

func (g *game) isStarted() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.p1 && g.p2
}

func (g *game) ready(p int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	switch p {
	case player1:
		g.p1 = true
	case player2:
		g.p2 = true
	}
}

func (g *game) close(p int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return
	}
	log.Printf("game instance %p has been closed by player%d", g, p)
	close(g.p1Ch)
	close(g.p2Ch)
	g.closed = true
}

func (g *game) isClosed() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.closed
}

type tetrisServer struct {
	pb.UnimplementedTetrisServiceServer
	waitList    *game
	waitTimeout time.Duration
	mu          sync.Mutex
}

func New() pb.TetrisServiceServer {
	return &tetrisServer{waitTimeout: defaultTimeOut}
}

func (t *tetrisServer) resetWL() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.waitList = nil
}

func (t *tetrisServer) PlayTetris(stream grpc.BidiStreamingServer[pb.GameMessage, pb.GameMessage]) error {
	var gameInstance *game
	var player = player1
	var name string
	var opponentCh chan *pb.GameMessage

	// The new game setup sequence happens under mutex lock to prevent
	// multiple concurrent connections reading the wating list as nil.
	t.mu.Lock()
	switch t.waitList {
	case nil:
		gameInstance = newGame()
		gameInstance.ready(player1)
		t.waitList = gameInstance
		opponentCh = gameInstance.p2Ch
	default:
		player = player2
		gameInstance = t.waitList
		gameInstance.ready(player2)
		t.waitList = nil
		opponentCh = gameInstance.p1Ch
	}
	t.mu.Unlock()
	defer gameInstance.close(player)

	gm, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.Canceled, "error receiving first stream message: %v", err)
	}
	name = gm.GetName()
	log.Printf("%s (player %d) connected to game %p\n", name, player, gameInstance)

	// Only player 1 waits for the opponent.
	if player == player1 {
		log.Printf("%s (player %d) is waiting to start game %p\n", name, player, gameInstance)
		to := time.After(t.waitTimeout)
		for !gameInstance.isStarted() {
			select {
			case <-to:
				// If player 1 times out waiting for opponent we clean up the gameInstance and waitingListID.
				t.resetWL()
				log.Printf("%s (player %d) timed out waiting to start game %p\n", name, player, gameInstance)
				return status.Error(codes.DeadlineExceeded, "timeout waiting for opponent")
			case <-stream.Context().Done():
				t.resetWL()
				log.Printf("%s (player %d) disconnected waiting to start game %p\n", name, player, gameInstance)
				return status.Error(codes.Canceled, "player disconnected")
			default:
				time.Sleep(10 * time.Millisecond)
			}
		}
	}
	if err := stream.Send(pb.GameMessage_builder{IsStarted: proto.Bool(true)}.Build()); err != nil {
		return status.Errorf(codes.Canceled, "failed to send gameMessage isStarted for %s (player%d): %v", name, player, err)
	}

	// Receive msg from stream and send to opponent's channel.
	ctx, cancel := context.WithCancel(context.Background()) //nolint: gosec
	go func() {
		defer cancel()
		ch := gameInstance.p1Ch
		if player == player2 {
			ch = gameInstance.p2Ch
		}
		for {
			gm, err := stream.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					return
				}
				st, ok := status.FromError(err)
				if ok && st.Code() == codes.Canceled {
					return
				}
				log.Printf("error receiving stream message in %s (player%d): %v", name, player, err)
				return
			}
			if gameInstance.isClosed() {
				return
			}
			ch <- gm
		}
	}()

	// Receive from opponent's channel and send to stream.
	for {
		select {
		case om, ok := <-opponentCh:
			if !ok {
				log.Printf("opponent channel closed for %s (player%d) in game %p", name, player, gameInstance)
				return nil
			}
			if err := stream.Send(om); err != nil {
				return status.Errorf(codes.Canceled, "failed to send opponent message for %s (player%d): %v", name, player, err)
			}
		case <-ctx.Done():
			log.Printf("%s (player %d) disconnected from game %p", name, player, gameInstance)
			return status.Errorf(codes.Canceled, "context canceled %s (player%d): %v", name, player, err)
		}
	}
}
