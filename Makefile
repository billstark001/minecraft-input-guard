.DEFAULT_GOAL := build

GO ?= go
CGO_ENABLED ?= 0
export CGO_ENABLED

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
	powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/update-resources.ps1 -Go "$(GO)"

run:
	$(GO) run $(COMMAND)

observe:
	$(GO) run $(COMMAND) -observe

clean:
	powershell.exe -NoProfile -Command "if (Test-Path -LiteralPath '$(BINARY)') { Remove-Item -LiteralPath '$(BINARY)' -ErrorAction Stop }"
