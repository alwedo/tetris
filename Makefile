AWS_REGION ?= eu-west-1
AWS_ACCOUNT_ID ?= your-account-id
APP_NAME ?= tetris-server
AWS_ENV ?= dev
APP_VERSION ?= latest

# Version injection using git describe with component-specific tags
# Use tags like: client/v0.1.0 and server/v0.1.0
CLIENT_VERSION ?= $(shell git describe --tags --match 'client/v*' --always --dirty 2>/dev/null | sed 's/^client\///' || echo "dev")
SERVER_VERSION ?= $(shell git describe --tags --match 'server/v*' --always --dirty 2>/dev/null | sed 's/^server\///' || echo "dev")

CLIENT_LDFLAGS := -ldflags "-s -w -X main.version=$(CLIENT_VERSION)"
SERVER_LDFLAGS := -ldflags "-s -w -X main.version=$(SERVER_VERSION)"

.PHONY: check
check: lint test

.PHONY: test
test:
	@go test ./...

.PHONY: lint
lint:
	@golangci-lint run

.PHONY: run-tetris
run-tetris: mod
	@go run cmd/client/main.go

.PHONY: tetris-version
tetris-version: build-tetris
	@./bin/tetris -version

.PHONY: version
version:
	@echo "Client: $(CLIENT_VERSION)"
	@echo "Server: $(SERVER_VERSION)"

.PHONY: client-version
client-version:
	@echo $(CLIENT_VERSION)

.PHONY: server-version
server-version:
	@echo $(SERVER_VERSION)

.PHONY: build-tetris
build-tetris: mod
	@echo "Building tetris client version: $(CLIENT_VERSION)"
	@CGO_ENABLED=0 go build -trimpath $(CLIENT_LDFLAGS) -o ./bin/tetris ./cmd/client
	@chmod +x ./bin/tetris

.PHONY: build-server
build-server: mod
	@echo "Building tetris server version: $(SERVER_VERSION)"
	@CGO_ENABLED=0 go build -trimpath $(SERVER_LDFLAGS) -o ./bin/tetris-server ./cmd/server
	@chmod +x ./bin/tetris-server

.PHONY: build-all
build-all: build-tetris build-server

.PHONY: run-server
run-server: mod
	@go run cmd/server/main.go

.PHONY: mod
mod:
	@go mod download

.PHONY: proto
proto:
	@protoc --go_out=./ --go_opt=paths=source_relative --go-grpc_out=./ --go-grpc_opt=paths=source_relative ./pb/server.proto

.PHONY: docker-build
docker-build:
	docker build --platform linux/amd64 -t $(APP_NAME)-$(AWS_ENV):$(APP_VERSION) .

.PHONY: docker-push
docker-push: docker-build
	aws ecr get-login-password --region $(AWS_REGION) | docker login --username AWS --password-stdin $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com
	docker tag $(APP_NAME)-$(AWS_ENV):$(APP_VERSION) $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/ecr-$(APP_NAME)-$(AWS_ENV):$(APP_VERSION)
	docker push $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/ecr-$(APP_NAME)-$(AWS_ENV):$(APP_VERSION)

.PHONY: deploy-ecs
deploy-ecs: docker-push
	aws ecs update-service --cluster tetris-cluster --service tetris-service --force-new-deployment --region $(AWS_REGION)
