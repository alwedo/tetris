# Terminal Tetris

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/alwedo/tetris) ![Test](https://github.com/alwedo/tetris/actions/workflows/test.yml/badge.svg) [![Release](https://img.shields.io/github/v/release/alwedo/tetris)](https://github.com/alwedo/tetris/releases/latest)

Play Tetris from the comfort of your terminal!

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

⚠️ the MacOS default terminal doesn't play very nice (pun intended). I'd higly recommend you to use [Ghostty](https://ghostty.org/).

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

### Installation

#### Homebrew (macOS/Linux)

```bash
brew install alwedo/tap/tetris
```

#### Download binary

Download the latest client from the [releases page](https://github.com/alwedo/tetris/releases/latest).

### Flags

```text
--version             Print the current version
--name string         Player name for Online mode (default "noName")
--address string      Tetris gRPC server address in host:port format (default "127.0.0.1:9000")
```

## The Server

The server pairs two players over a gRPC stream. It isn't distributed as a standalone binary or service; to use it, import the [`server`](./server) package and register it with a gRPC server.

## The SSH server

The SSH server lets people play the TUI remotely over SSH; it runs the game server alongside it in the same container. It is not part of the client release.

Set `SSH_HOST_KEY_PEM` to the SSH host key, then start the Docker service:

```bash
make ssh
```

Connect locally with `ssh -p 2222 localhost`.

To start it with a Cloudflare Tunnel, also set `CLOUDFLARE_TUNNEL_TOKEN` and run:

```bash
make ssh-cf
```

Configure the tunnel's public hostname to route SSH traffic to `tetris:2222`.
