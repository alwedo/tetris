.PHONY: check
check: lint test

.PHONY: test
test:
	@go test ./...

.PHONY: lint
lint:
	@golangci-lint run

.PHONY: run
run: mod
	@go run cmd/client/main.go

.PHONY: ssh
ssh:
	@docker compose down || true
	@docker compose -f docker-compose.yml up -d --build

.PHONY: ssh-cf
ssh-cf:
	@docker compose down || true
	@docker compose -f docker-compose.yml -f docker-compose-cloudflare.yml up -d --build

.PHONY: mod
mod:
	@go mod download

.PHONY: proto
proto:
	@protoc --go_out=./ --go_opt=paths=source_relative --go-grpc_out=./ --go-grpc_opt=paths=source_relative ./pb/server.proto
