FROM golang:latest AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download
COPY . .

RUN make build-tetris
RUN make build-server

FROM alpine:latest

RUN apk add --no-cache openssh-server

COPY --from=builder /build/bin/tetris-server /usr/local/bin/
COPY --from=builder /build/bin/tetris /usr/local/bin/
COPY --from=builder /build/entrypoint.sh /usr/local/bin/

# Create wrapper script that runs tetris and exits
RUN echo '#!/bin/sh' > /usr/local/bin/tetris-wrapper && \
    echo 'exec /usr/local/bin/tetris --address=localhost:9000 --name="$USER"' >> /usr/local/bin/tetris-wrapper && \
    chmod +x /usr/local/bin/tetris-wrapper

EXPOSE 9000 22

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]
