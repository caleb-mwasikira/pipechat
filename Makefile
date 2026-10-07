# Build the terminal chat client for each platform into dist/, ready to upload at /admin/binaries/upload.
#
#   make build              Linux and Windows (amd64)
#   make build-all          also macOS and Linux arm64
#   make build VERSION=1.2.3
#   make checksums          print the MD5 of everything in dist/
#   make test | clean

VERSION ?= 1.2.0
DIST    := dist
SRC     := pipechat_terminal
# The version is built into the program, which is how it knows whether a newer one has been published.
GOFLAGS := -trimpath -ldflags "-s -w -X pipechat/internal/client/update.Current=$(VERSION)"

# Static binaries: no C compiler or runtime libraries needed on the target.
GOBUILD = cd $(SRC) && CGO_ENABLED=0 GOOS=$(1) GOARCH=$(2) go build $(GOFLAGS) -o ../$(DIST)/pipechat-$(VERSION)-$(3) ./cmd/chat

.PHONY: build build-all linux windows macos linux-arm64 checksums test clean

build: linux windows checksums

build-all: build macos linux-arm64 checksums

linux:
	@mkdir -p $(DIST)
	$(call GOBUILD,linux,amd64,linux-amd64)

windows:
	@mkdir -p $(DIST)
	$(call GOBUILD,windows,amd64,windows-amd64.exe)

macos:
	@mkdir -p $(DIST)
	$(call GOBUILD,darwin,arm64,macos-arm64)
	$(call GOBUILD,darwin,amd64,macos-amd64)

linux-arm64:
	@mkdir -p $(DIST)
	$(call GOBUILD,linux,arm64,linux-arm64)

checksums:
	@cd $(DIST) && md5sum pipechat-$(VERSION)-*

test:
	cd $(SRC) && go vet ./... && go test ./...

clean:
	rm -rf $(DIST)
