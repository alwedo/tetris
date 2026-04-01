package server

import (
	"errors"
	"io"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/alwedo/tetris/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const defaultTimeOut = 30 * time.Second

type game struct {
	p1Ch, p2Ch   chan *pb.GameMessage
	waitOpponent atomic.Bool
}

func newGame() *game {
	g := &game{
		p1Ch: make(chan *pb.GameMessage),
		p2Ch: make(chan *pb.GameMessage),
	}
	g.waitOpponent.Store(true)
	return g
}

type tetrisServer struct {
	pb.UnimplementedTetrisServiceServer

	waitList    atomic.Pointer[game]
	waitTimeout time.Duration
	mu          sync.Mutex
}

func New() pb.TetrisServiceServer {
	return &tetrisServer{waitTimeout: defaultTimeOut}
}

func (t *tetrisServer) PlayTetris(stream grpc.BidiStreamingServer[pb.GameMessage, pb.GameMessage]) error {
	var gameInstance *game
	var playerCh chan *pb.GameMessage
	var opponentCh chan *pb.GameMessage

	// The new game setup sequence happens under mutex lock to prevent
	// multiple concurrent connections reading the waiting list as nil.
	t.mu.Lock()
	switch t.waitList.Load() {
	case nil:
		gameInstance = newGame()
		t.waitList.Store(gameInstance)
		playerCh, opponentCh = gameInstance.p1Ch, gameInstance.p2Ch
	default:
		gameInstance = t.waitList.Swap(nil)
		playerCh, opponentCh = gameInstance.p2Ch, gameInstance.p1Ch
		gameInstance.waitOpponent.Store(false)
	}
	t.mu.Unlock()

	gm, err := stream.Recv()
	if err != nil {
		return status.Errorf(codes.Canceled, "error receiving first stream message: %v", err)
	}
	name := gm.GetName()
	log.Printf("%s connected to game %p\n", name, gameInstance)

	// Wait for opponent
	to := time.After(t.waitTimeout)
	for gameInstance.waitOpponent.Load() {
		var code codes.Code
		var msg string

		select {
		case <-to:
			code, msg = codes.DeadlineExceeded, "timeout waiting for opponent"
			log.Printf("%s timed out waiting to start game %p\n", name, gameInstance)
		case <-stream.Context().Done():
			code, msg = codes.Canceled, "player disconnected"
			log.Printf("%s disconnected waiting to start game %p\n", name, gameInstance)
		default:
			continue
		}
		t.waitList.CompareAndSwap(gameInstance, nil)
		return status.Error(code, msg)
	}

	if err := stream.Send(pb.GameMessage_builder{IsStarted: new(true)}.Build()); err != nil {
		return status.Errorf(codes.Canceled, "failed to send gameMessage isStarted for %s: %v", name, err)
	}

	go func() {
		defer close(playerCh)
		for {
			msg, err := stream.Recv()
			if err != nil {
				st, ok := status.FromError(err)
				if ok && st.Code() == codes.Canceled || errors.Is(err, io.EOF) {
					return
				}
				log.Printf("error receiving stream message in %s: %v", name, err)
				return
			}
			select {
			case playerCh <- msg:
			case <-stream.Context().Done():
				return
			}
		}
	}()

	for {
		select {
		case msg, ok := <-opponentCh:
			if !ok {
				log.Printf("opponent channel closed for %s in game %p", name, gameInstance)
				return nil
			}
			if err := stream.Send(msg); err != nil {
				return status.Errorf(codes.Canceled, "failed to send opponent message for %s: %v", name, err)
			}
		case <-stream.Context().Done():
			log.Printf("%s disconnected from game %p", name, gameInstance)
			return status.Errorf(codes.Canceled, "context canceled %s: %v", name, stream.Context().Err())
		}
	}
}
