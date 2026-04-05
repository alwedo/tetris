package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	tea "charm.land/bubbletea/v2"
	"charm.land/wish/v2"
	"charm.land/wish/v2/activeterm"
	wishbubbletea "charm.land/wish/v2/bubbletea"
	"charm.land/wish/v2/logging"
	"github.com/charmbracelet/ssh"

	"github.com/alwedo/tetris/client"
	"github.com/alwedo/tetris/pb"
	"github.com/alwedo/tetris/server"
	"google.golang.org/grpc"
)

var version = "dev"

func main() {
	sshPort := flag.String("ssh-port", "22", "SSH listen port")
	grpcPort := flag.String("grpc-port", "9000", "gRPC server port")
	hostKeyPath := flag.String("host-key", "/data/ssh_host_ed25519", "Path to SSH host key (auto-generated if missing)")
	flag.Parse()

	grpcAddr := ":" + *grpcPort

	// Start the gRPC tetris server
	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", grpcAddr, err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterTetrisServiceServer(grpcServer, server.New())

	go func() {
		fmt.Printf("Tetris gRPC server %s listening on %s\n", version, grpcAddr)
		if err := grpcServer.Serve(lis); err != nil {
			log.Printf("gRPC server error: %v", err)
		}
	}()

	// Configure host key: prefer SSH_HOST_KEY_PEM env var, fall back to file path
	var hostKeyOption ssh.Option
	if pem := os.Getenv("SSH_HOST_KEY_PEM"); pem != "" {
		hostKeyOption = wish.WithHostKeyPEM([]byte(pem))
	} else {
		hostKeyOption = wish.WithHostKeyPath(*hostKeyPath)
	}

	// Start the wish SSH server
	sshServer, err := wish.NewServer(
		wish.WithAddress(":"+*sshPort),
		hostKeyOption,
		wish.WithMiddleware(
			wishbubbletea.Middleware(func(sess ssh.Session) (tea.Model, []tea.ProgramOption) {
				return client.NewRootModel(sess.Context(), sess.User()), []tea.ProgramOption{tea.WithFPS(25)}
			}),
			activeterm.Middleware(),
			logging.Middleware(),
		),
	)
	if err != nil {
		log.Fatalf("failed to create SSH server: %v", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		fmt.Printf("Tetris SSH server listening on :%s\n", *sshPort)
		if err := sshServer.ListenAndServe(); err != nil {
			log.Fatalf("SSH server error: %v", err)
		}
	}()

	<-done
	fmt.Println("\nShutting down...")
	grpcServer.GracefulStop()
	sshServer.Shutdown(context.Background()) //nolint: errcheck
}
