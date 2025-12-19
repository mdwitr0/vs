.PHONY: build run test swagger clean

build:
	go build -o bin/indexer ./cmd/main.go

run:
	go run ./cmd/main.go

test:
	go test ./...

swagger:
	swag init -g cmd/main.go -o ./docs --parseDependency --parseInternal

clean:
	rm -rf bin/ docs/
