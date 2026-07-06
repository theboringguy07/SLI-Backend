.PHONY: build run test vet docker-build docker-up

build:
	go build ./...

run:
	go run ./cmd/server

test:
	go test ./...

vet:
	go vet ./...

docker-build:
	docker build -t sli-backend .

docker-up:
	docker compose up --build
