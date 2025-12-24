.PHONY: build run dev test clean docker-build docker-run

build:
	go build -o server ./cmd/server

run:
	go run ./cmd/server

dev:
	air

test:
	go test -v ./...

clean:
	rm -f server
	go clean

docker-build:
	docker build -t player-monitor-backend .

docker-run:
	docker run -p 8080:8080 \
		-e MONGO_URL=mongodb://host.docker.internal:27017 \
		-e JWT_SECRET=secret \
		-e ADMIN_PASSWORD=admin123 \
		player-monitor-backend

tidy:
	go mod tidy

fmt:
	go fmt ./...

vet:
	go vet ./...

lint: fmt vet
	@echo "Linting complete"
