package main

import (
	"fmt"
	"log"
	"net"

	"github.com/alwedo/tetris/pb"
	"github.com/alwedo/tetris/server"
	"google.golang.org/grpc"
)

const port = ":9000"

var version = "dev"

func main() {
	lis, err := net.Listen("tcp", port) //nolint:gosec
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()
	s := grpc.NewServer()
	defer s.Stop()
	pb.RegisterTetrisServiceServer(s, server.New())

	fmt.Printf("Tetris Server %s starting in port %s...\n", version, port)
	if err := s.Serve(lis); err != nil {
		log.Printf("failed to serve: %v", err)
	}
}
