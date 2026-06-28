.PHONY: build test run fmt clean

build:
	go build -o bin/minfin ./cmd/minfin

test:
	go test ./...

run:
	go run ./cmd/minfin

demo: build
	MINFIN_DB=demo.db ./bin/minfin

fmt:
	go fmt ./...

clean:
	rm -rf bin
