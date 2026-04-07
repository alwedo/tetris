# Terminal Tetris

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/alwedo/tetris) ![Test](https://github.com/alwedo/tetris/actions/workflows/test.yml/badge.svg) ![Latest Release](https://img.shields.io/github/v/release/alwedo/tetris?color=blue&label=Latest%20Release)

Play Tetris from the comfort of your terminal!

![example](./doc/example.gif)

⚠️ this assumes you know how to use the terminal! If you don't you can find out how [here](https://www.google.com/search?q=how+to+use+the+terminal).

## I don't care about your project just let me play Terminal Tetris NOW!

Ok, I got you!

1. Open your terminal and type: 
```sh
ssh playtts.cc
```

If you get this message:
```sh
The authenticity of host 'playtts.cc (<some ip address>)' can't be established.
ED25519 key fingerprint is: SHA256:MBrRN7xVN6Q1Ihod2VK95nbcaXgmrYEyznK072Xn2GE
This key is not known by any other names.
Are you sure you want to continue connecting (yes/no/[fingerprint])?
```
Type `yes` and press enter.

2. Play!
3. (optional) Make sure you're not connecting to a fake Terminal Tetris server:
```sh
cat ~/.ssh/known_hosts | grep playtts.cc
``` 
you should get:
```sh
playtts.cc ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHEKS48gvoxj3bLsO2wBTgCoUiIVEscR7xeh0D0CWznk
```

⚠️ the MacOS default terminal doesn't play very nice (pun intended). I'd recommend you to use any another terminal client like [iTerm2](https://iterm2.com/) or the like.

## The Go Tetris Engine

This project contains an implementation agnostic Tetris engine written in go. You'll find that I created a TUI with it but you might as well create any other kind of implementation.

Example usage:

```go
import (
	"context"
	"fmt"

	"github.com/alwedo/tetris"
)

	func main() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel() // you can cancel the game at any time
		t := tetris.Start(ctx)

		// asynchronously receive updates from the game
		go func() {
			for {
				select {
				case msg, ok := <-t.UpdateCh:
					if !ok { // game over
						return
					}
					// use the Tetris status
					fmt.Println(msg)
				case <-ctx.Done():
					// use cancel func to end the game if needed
					return
				}
			}
		}()

		// send commands to the game
		t.Do(tetris.MoveRight()) // or any other action
	}
```

## The Client (TUI)

You'll find my implementation in the [client](./client) folder. It's based on [Bubble Tea](https://github.com/charmbracelet/bubbletea) and it's currently a WIP so please be gentle when judging the design :) 

You'll find the latest version of it in the release section.

### Client options

Prints current version.

```bash
tetris -version
```

Sets player's name for Online mode.

```bash
tetris -name="YOUR_NAME"
```

Sets the server address for Online mode.

```bash
tetris -address="YOUR_SERVER_ADDRESS"
```


## The Server

The Tetris server connects two players over a gRPC bi-directional streaming connection. Implementation is in the [server](./server) folder and you'll also find the latest version of it in the release section.

## The SSH server
