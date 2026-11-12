GO      ?= mise exec go -- go
BINARY  := apidiff
PREFIX  ?= $(HOME)/.local
BINDIR  := $(PREFIX)/bin
PKG     := ./cmd/apidiff

.PHONY: all build test check install uninstall clean

all: test build

# Vet and the full race-enabled suite. CI runs the same command.
test:
	$(GO) vet ./...
	$(GO) test -race -count=1 ./...

# Compile into the repo-local artifact bin/$(BINARY) (gitignored). Never installs.
build:
	$(GO) build -o bin/$(BINARY) $(PKG)
	@echo "✓ built bin/$(BINARY)"

# One-shot local gate. The trailing `go build ./...` also compiles packages that
# are not reachable from the binary.
check: build test
	$(GO) build ./...

# Copy the already-built artifact to its final location. Honors PREFIX and
# DESTDIR for staged installs.
install: build
	install -d $(DESTDIR)$(BINDIR)
	install -m 0755 bin/$(BINARY) $(DESTDIR)$(BINDIR)/$(BINARY)
	@echo "✓ installed to $(DESTDIR)$(BINDIR)/$(BINARY)"

uninstall:
	rm -f $(DESTDIR)$(BINDIR)/$(BINARY)

clean:
	rm -rf bin/
