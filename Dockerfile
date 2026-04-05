FROM golang:latest AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN make build-ssh

FROM scratch

COPY --from=builder /build/bin/tetris-ssh /usr/local/bin/
EXPOSE 9000 22
ENTRYPOINT ["/usr/local/bin/tetris-ssh"]
