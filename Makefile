AWS_REGION ?= eu-west-1
AWS_ACCOUNT_ID ?= your-account-id
APP_NAME ?= tetris-server
AWS_ENV ?= dev
APP_VERSION ?= latest

.PHONY: check test lint run-tetris tetris-version build-tetris mod proto docker-build docker-push deploy-ecs

check: lint test

test:
	@go test ./...

lint:
	@golangci-lint run

run-tetris: mod
	@go run cmd/client/main.go

tetris-version: mod
	@go run cmd/client/main.go -version

build-tetris: mod
	@CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o ./bin/tetris ./cmd/client/main.go
	@chmod +x ./bin/tetris

run-server: mod
	@go run cmd/server/main.go

mod:
	@go mod download

proto:
	@protoc --go_out=./ --go_opt=paths=source_relative --go-grpc_out=./ --go-grpc_opt=paths=source_relative ./pb/server.proto

docker-build:
	docker build --platform linux/amd64 -t $(APP_NAME)-$(AWS_ENV):$(APP_VERSION) .

docker-push: docker-build
	aws ecr get-login-password --region $(AWS_REGION) | docker login --username AWS --password-stdin $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com
	docker tag $(APP_NAME)-$(AWS_ENV):$(APP_VERSION) $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/ecr-$(APP_NAME)-$(AWS_ENV):$(APP_VERSION)
	docker push $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/ecr-$(APP_NAME)-$(AWS_ENV):$(APP_VERSION)

deploy-ecs: docker-push
	aws ecs update-service --cluster tetris-cluster --service tetris-service --force-new-deployment --region $(AWS_REGION)
