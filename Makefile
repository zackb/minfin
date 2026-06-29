.PHONY: build tui gtk demo-tui demo-gtk test run fmt clean

# Throwaway DB for the demo targets — never the live minfin.db.
DEMO_DB ?= /tmp/minfin-demo.db

all: build tui gtk

build:
	go build -o bin/minfin ./cmd/minfin

# Local thick clients — read the SQLite file directly, no server.
tui:
	go build -o bin/minfin-tui ./cmd/minfin-tui

# Needs GTK4 dev libraries (pkg-config gtk4); cgo via gotk4.
gtk:
	go build -o bin/minfin-gtk ./cmd/minfin-gtk

# Seed a fresh throwaway DB and launch the client against it.
demo-tui: tui
	rm -f "$(DEMO_DB)" && go run ./cmd/minfin-seed "$(DEMO_DB)"
	MINFIN_DB="$(DEMO_DB)" ./bin/minfin-tui

demo-gtk: gtk
	rm -f "$(DEMO_DB)" && go run ./cmd/minfin-seed "$(DEMO_DB)"
	MINFIN_DB="$(DEMO_DB)" ./bin/minfin-gtk

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
