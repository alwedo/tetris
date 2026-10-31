package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	tea "charm.land/bubbletea/v2"
	client "github.com/alwedo/tetris/client"
)

var version = "dev"

var (
	name, address string
)

func main() {
	flag.BoolFunc("version", "Prints version", versionFunc)
	flag.StringVar(&name, "name", "noName", "Current player's name")
	flag.StringVar(&address, "address", "127.0.0.1:9000", "Tetris gRPC server address (host:port)")
	flag.Parse()

	if _, err := tea.NewProgram(client.NewRootModel(context.Background(), name, address), tea.WithFPS(25)).Run(); err != nil {
		log.Fatalf("Alas, there's been an error: %v", err)
	}
}

func versionFunc(string) error {
	fmt.Printf("Terminal Tetris verion %s", version)
	os.Exit(0)

	return nil
}
