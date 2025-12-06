package server

import (
	"context"
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/alwedo/tetris/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/proto"
)

func TestPlayTetris(t *testing.T) {
	t.Run("normal game flow with multiple players", func(t *testing.T) {
		lis, closer := testServer(t)
		defer closer()

		var wg sync.WaitGroup
		var players = 100
		wg.Add(players)
		for i := range players {
			go func() { testPlayer(t, i+1, lis); wg.Done() }()
		}
		wg.Wait()
	})

	t.Run("time out waiting for opponent", func(t *testing.T) {
		server := &tetrisServer{waitTimeout: 150 * time.Millisecond}
		lis, closer := testCustomServer(t, server)
		defer closer()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		conn := testClient(t, lis)
		game, err := pb.NewTetrisServiceClient(conn).PlayTetris(ctx)
		if err != nil {
			t.Errorf("error calling NewGame: %v", err)
		}

		if err := game.Send(pb.GameMessage_builder{Name: proto.String("test")}.Build()); err != nil {
			t.Errorf("error sending: %v", err)
			return
		}

		for err == nil {
			_, err = game.Recv()
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.DeadlineExceeded || st.Message() != "timeout waiting for opponent" {
			t.Errorf("expected DeadlineExceeded with message 'timeout waiting for opponent', got %v", err)
		}
		if server.waitList != nil {
			t.Errorf("expected waitListID pointer to be nil, got %p", server.waitList)
		}
	})

	t.Run("cancel waiting for opponent", func(t *testing.T) {
		server := &tetrisServer{waitTimeout: 150 * time.Millisecond}
		lis, closer := testCustomServer(t, server)
		defer closer()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		conn := testClient(t, lis)
		game, err := pb.NewTetrisServiceClient(conn).PlayTetris(ctx)
		if err != nil {
			t.Errorf("error calling NewGame: %v", err)
		}

		if err := game.Send(pb.GameMessage_builder{Name: proto.String("test")}.Build()); err != nil {
			t.Errorf("error sending: %v", err)
			return
		}

		time.AfterFunc(50*time.Millisecond, func() { cancel() })
		for err == nil {
			_, err = game.Recv()
		}
		st, ok := status.FromError(err)
		if !ok || st.Code() != codes.Canceled {
			t.Errorf("expected Canceled with message 'player disconnected', got %v", err)
		}
		time.Sleep(50 * time.Millisecond)
		if server.waitList != nil {
			t.Errorf("expected waitListID pointer to be nil, got %p", server.waitList)
		}
	})
}

func testServer(t testing.TB) (*bufconn.Listener, func()) {
	return testCustomServer(t, New())
}

func testCustomServer(t testing.TB, tss pb.TetrisServiceServer) (*bufconn.Listener, func()) {
	buffer := 101024 * 1024
	lis := bufconn.Listen(buffer)

	s := grpc.NewServer()
	pb.RegisterTetrisServiceServer(s, tss)
	go func() {
		if err := s.Serve(lis); err != nil {
			t.Fatalf("unable to serve: %v", err)
		}
	}()

	return lis, func() {
		if err := lis.Close(); err != nil {
			t.Fatalf("error closing listener: %v", err)
		}
		s.Stop()
	}
}

func testClient(t testing.TB, lis *bufconn.Listener) *grpc.ClientConn {
	conn, err := grpc.NewClient("foo.googleapis.com:8080", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("error connecting to server: %v", err)
	}
	return conn
}

func testPlayer(t *testing.T, n int, lis *bufconn.Listener) {
	ctx, timeout := context.WithTimeout(context.Background(), 10*time.Second)
	defer timeout()
	conn := testClient(t, lis)
	game, err := pb.NewTetrisServiceClient(conn).PlayTetris(ctx)
	if err != nil {
		t.Errorf("error calling NewGame for P%d: %v", n, err)
	}
	outMsg := pb.GameMessage_builder{Name: proto.String(fmt.Sprintf("player%d", n))}.Build()
	if err := game.Send(outMsg); err != nil {
		t.Errorf("error sending player name for P%d: %v", n, err)
	}
	// Waits for opponent
	var started bool
	for !started {
		gm, err := game.Recv()
		if err != nil {
			t.Fatalf("error receiving message while waiting for game to start for P%d: %v", n, err)
		}
		started = gm.GetIsStarted()
	}
	// Players send values back and forth
	for i := range 50 {
		outMsg.SetLinesClear(int32(i)) // nolint:gosec
		if err := game.Send(outMsg); err != nil {
			t.Errorf("error sending player name for P%d: %v", n, err)
			return
		}
		gm, err := game.Recv()
		if err != nil {
			t.Errorf("error receiving message from opponent for P%d: %v", n, err)
			return
		}
		if gm.GetLinesClear() != int32(i) { // nolint:gosec
			t.Errorf("expected %d lines cleared for player%d, got %d", i, n, gm.GetLinesClear())
			return
		}
	}
}
