.PHONY: build tui gtk demo-tui demo-gtk test run fmt clean icons install-gtk uninstall-gtk

# Throwaway DB for the demo targets
DEMO_DB ?= /tmp/minfin-demo.db

# User-level install prefix (no sudo). Override PREFIX=/usr/local for a system install.
PREFIX  ?= $(HOME)/.local
BINDIR  := $(PREFIX)/bin
DATADIR := $(PREFIX)/share
APPID   := com.zackbartel.minfin

all: build tui gtk

build:
	go build -o bin/minfin ./cmd/minfin

tui:
	go build -o bin/minfin-tui ./cmd/minfin-tui

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

# Regenerate icons from assets/icon.png (needs ImageMagick). Outputs are committed.
icons:
	scripts/generate_icons.sh

# Install the GTK app + desktop entry + icons under $(PREFIX)
install-gtk: gtk
	install -Dm755 bin/minfin-gtk $(BINDIR)/minfin-gtk
	mkdir -p $(DATADIR)/icons/hicolor $(DATADIR)/applications
	cp -a assets/icons/hicolor/. $(DATADIR)/icons/hicolor/
	sed 's|@BINDIR@|$(BINDIR)|g' packaging/$(APPID).desktop > $(DATADIR)/applications/$(APPID).desktop
	-update-desktop-database $(DATADIR)/applications
	-gtk-update-icon-cache -qtf $(DATADIR)/icons/hicolor

uninstall-gtk:
	rm -f $(BINDIR)/minfin-gtk $(DATADIR)/applications/$(APPID).desktop
	rm -f $(DATADIR)/icons/hicolor/*/apps/$(APPID).png
	-update-desktop-database $(DATADIR)/applications
	@echo "Removed minfin-gtk; DB left in place."
