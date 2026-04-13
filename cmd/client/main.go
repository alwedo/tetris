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

const (
	versionFlag = "version"
	nameFlag    = "name"
	addressFlag = "address"
)

var (
	name, address string
)

func main() {
	evalOptions()

	p := tea.NewProgram(client.NewRootModel(context.Background(), name), tea.WithFPS(25))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

func evalOptions() {
	flag.BoolFunc(versionFlag, "Prints version", versionFunc)
	flag.StringVar(&name, nameFlag, "noName", "Current player's name")
	flag.StringVar(&address, addressFlag, "127.0.0.1", "Tetris server address")
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func versionFunc(string) error {
	fmt.Println(version)
	os.Exit(0)

	return nil
}
