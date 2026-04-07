#!/bin/sh

set -e

/usr/local/bin/tetris-server &
SERVER_PID=$!

sleep 1

exec /usr/sbin/sshd -D -e

# If sshd exits, kill the server
kill $SERVER_PID 2>/dev/null || true
