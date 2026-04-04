package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "charm.land/bubbletea/v2"
	client "github.com/alwedo/tetris/clientv2"
)

const VERSION = "v0.0.13"

const (
	versionFlag = "version"
	noGhostFlag = "noghost"
	nameFlag    = "name"
	addressFlag = "address"
)

var (
	noGhost       bool
	name, address string
)

func main() {
	evalOptions()

	p := tea.NewProgram(client.NewRootModel(), tea.WithFPS(25))
	if _, err := p.Run(); err != nil {
		fmt.Printf("Alas, there's been an error: %v", err)
		os.Exit(1)
	}
}

func evalOptions() {
	flag.BoolFunc(versionFlag, "Prints version", version)
	flag.BoolVar(&noGhost, noGhostFlag, false, "Disables Ghost Piece")
	flag.StringVar(&name, nameFlag, "noName", "Current player's name")
	flag.StringVar(&address, addressFlag, "127.0.0.1", "Tetris server address")
	if err := flag.CommandLine.Parse(os.Args[1:]); err != nil {
		log.Fatal(err)
	}
}

func version(string) error {
	fmt.Println(VERSION)
	os.Exit(0)

	return nil
}
