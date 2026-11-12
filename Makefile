.DEFAULT_GOAL := build

GO ?= go
GOTOOLCHAIN ?= go1.27.4
CGO_ENABLED ?= 0
export GOTOOLCHAIN CGO_ENABLED

COMMAND := ./cmd/minecraft-input-guard
BINARY := bin/MinecraftInputGuard.exe

.PHONY: all fmt vet test check build resources run observe clean

all: check build

fmt:
	$(GO) fmt ./...

vet:
	$(GO) vet ./...

test:
	$(GO) test ./...

check: vet test

build:
	$(GO) build -trimpath -ldflags="-H=windowsgui" -o $(BINARY) $(COMMAND)

resources:
	$(GO) run github.com/tc-hib/go-winres@v0.3.3 make --in assets/winres.json --out internal/win32/icon --arch amd64,arm64

run:
	$(GO) run $(COMMAND)

observe:
	$(GO) run $(COMMAND) -observe

clean:
	powershell.exe -NoProfile -Command "if (Test-Path -LiteralPath '$(BINARY)') { Remove-Item -LiteralPath '$(BINARY)' -ErrorAction Stop }"
