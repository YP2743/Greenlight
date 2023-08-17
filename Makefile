run:
	go run ./cmd/api/

build-test:
	go build -gcflags=all="-N -l" ./cmd/api/