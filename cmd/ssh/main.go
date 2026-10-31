package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/ssh"
	"charm.land/wish/v2"
	"charm.land/wish/v2/activeterm"
	wishbubbletea "charm.land/wish/v2/bubbletea"

	"github.com/alwedo/tetris/client"
	"github.com/alwedo/tetris/pb"
	"github.com/alwedo/tetris/server"
	"google.golang.org/grpc"
)

var version = "dev"

var (
	httpPort = ":8080"
	grpcPort = ":9000"
	sshPort  = ":2222"
)

func main() {
	log.Printf("Starting Tetris SSH Server %s", version)

	if err := run(); err != nil {
		log.Fatalf("Shut Down With Errors: %v", err)
	}
	log.Println("Shut Down Complete")
}

func run() error {
	// set up the HTTP server
	httpServer := &http.Server{
		Addr:        httpPort,
		ReadTimeout: 2 * time.Second,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprintln(w, "Open your terminal and type:\n\nssh playtts.cc\n\nNot sure what are you doing? visit: https://github.com/alwedo/tetris")
		}),
	}

	// set up the gRPC tetris server
	// #nosec G102 -- Docker port publishing forwards external connections to this container listener.
	lis, err := net.Listen("tcp", grpcPort)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", grpcPort, err)
	}
	defer func() {
		if err := lis.Close(); err != nil {
			log.Printf("error closing the listener: %v", err)
		}
	}()
	grpcServer := grpc.NewServer()
	pb.RegisterTetrisServiceServer(grpcServer, server.New())

	// set up the wish SSH server
	pem := os.Getenv("SSH_HOST_KEY_PEM")
	if pem == "" {
		return errors.New("SSH_HOST_KEY_PEM env var not found or empty")
	}
	sshServer, err := wish.NewServer(
		wish.WithAddress(sshPort),
		wish.WithHostKeyPEM([]byte(pem)),
		wish.WithMiddleware(
			wishbubbletea.Middleware(func(sess ssh.Session) (tea.Model, []tea.ProgramOption) {
				return client.NewRootModel(sess.Context(), sess.User(), "127.0.0.1:9000"), []tea.ProgramOption{tea.WithFPS(25)}
			}),
			activeterm.Middleware(),
		),
	)
	if err != nil {
		return fmt.Errorf("failed to create SSH server: %w", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	errChan := make(chan error, 1)

	// start HTTP server
	go func() {
		log.Printf("HTTP server listening on %s\n", httpPort)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errChan <- fmt.Errorf("HTTP server error: %w", err)
		}
	}()

	// start gRPC server
	go func() {
		log.Printf("gRPC server listening on %s\n", grpcPort)
		if err := grpcServer.Serve(lis); err != nil {
			errChan <- fmt.Errorf("gRPC server error: %w", err)
		}
	}()

	// start SSH server
	go func() {
		log.Printf("SSH server listening on %s\n", sshPort)
		if err := sshServer.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			errChan <- fmt.Errorf("SSH server error: %w", err)
		}
	}()

	var runtimeErr error
	select {
	case <-ctx.Done():
	case runtimeErr = <-errChan:
	}

	log.Println("Shutting down...")
	ctxShutdown, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()

	var shutdownErr error
	// shutdown HTTP server
	if err := httpServer.Shutdown(ctxShutdown); err != nil {
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("error shutting down HTTP server: %w", err))
	}

	// shutdown SSH server
	if err := sshServer.Shutdown(ctxShutdown); err != nil {
		shutdownErr = errors.Join(shutdownErr, fmt.Errorf("error shutting down SSH server: %w", err))
	}

	// shutdown gRPC server
	grpcServer.Stop()

	return errors.Join(runtimeErr, shutdownErr)
}
