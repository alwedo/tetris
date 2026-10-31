FROM golang:latest AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o ./bin/tetris-ssh ./cmd/ssh

FROM scratch

COPY --from=builder /build/bin/tetris-ssh /usr/local/bin/
EXPOSE 2222 9000 8080
ENTRYPOINT ["/usr/local/bin/tetris-ssh"]
