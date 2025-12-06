package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/alwedo/tetris/client"
)

const VERSION = "v0.0.13"

const (
	hideCursor = "\033[2J\033[?25l" // also clear screen
	showCursor = "\n\033[22;0H\n\033[?25h"
	logFile    = ".tetrisLog"

	// Option Flags.
	debugFlag   = "debug"
	versionFlag = "version"
	noGhostFlag = "noghost"
	nameFlag    = "name"
	addressFlag = "address"
)

var (
	debug, noGhost bool
	name, address  string
)

func main() {
	evalOptions()
	c, err := client.New(initLogger(), &client.Options{
		NoGhost: noGhost,
		Address: address,
		Name:    name,
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print(hideCursor)
	defer fmt.Print(showCursor)
	c.Start()
}

func initLogger() *slog.Logger {
	out := io.Discard
	if debug {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("error getting home directory: %v", err)
		}

		out, err = os.OpenFile(filepath.Join(homeDir, logFile), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("unable to open log file: %v", err)
		}
	}
	handler := slog.NewJSONHandler(out, &slog.HandlerOptions{Level: slog.LevelDebug})
	return slog.New(handler)
}

func evalOptions() {
	flag.BoolFunc(versionFlag, "Prints version", version)
	flag.BoolVar(&debug, debugFlag, false, "Enables debugging into ~/.tetrisLog")
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
