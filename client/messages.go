package client

import (
	"github.com/alwedo/tetris/pb"
	"github.com/alwedo/tetris/tetris"
	"google.golang.org/grpc"
)

type TransitionToLobbyMsg struct {
	Message         string
	LocalGameState  tetris.GameMessage
	RemoteGameState *pb.GameMessage
}

type TransitionToSingleGameMsg struct{}

type TransitionToMPGameMsg struct {
	Conn          *grpc.ClientConn
	Stream        grpc.BidiStreamingClient[pb.GameMessage, pb.GameMessage]
	OpponentState *pb.GameMessage
}

type connectionSuccessMsg struct {
	conn   *grpc.ClientConn
	stream grpc.BidiStreamingClient[pb.GameMessage, pb.GameMessage]
}

type connectionErrorMsg struct {
	err error
}

type streamErrorMsg struct {
	err error
}
