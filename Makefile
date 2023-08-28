.PHONY: run
run:
	go run ./cmd/api/

.PHONY: build-test
build-test:
	go build -gcflags=all="-N -l" ./cmd/api/