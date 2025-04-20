#####################
# make inspectre
#####################

BINARY_NAME   := inspectre
VERSION_PATH  := github.com/thushan/inspectre/internal/version
VERSION       := $(shell git describe --tags --always --dirty)
GCOMMIT       := $(shell git rev-parse --short HEAD)
TODAY         := $(shell date --iso-8601=seconds)
LDFLAGS       := \
  -X $(VERSION_PATH).Date=$(TODAY) \
  -X $(VERSION_PATH).Version=$(VERSION) \
  -X $(VERSION_PATH).Commit=$(GCOMMIT) \
  -s -w

DIST_DIR      := dist
DIST_FOLDERS  := configs plugins examples

.PHONY: all lint test build install dev copy cross-build clean

all: lint test build cross-build

lint:
	golangci-lint run -c .golangci.yml ./...

test:
	go test -v ./...

build: copy
	go build -ldflags "$(LDFLAGS)" -trimpath \
	  -o $(DIST_DIR)/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

install:
	go install -ldflags "$(LDFLAGS)" -trimpath ./cmd/$(BINARY_NAME)

dev:
	@echo "→ running $(BINARY_NAME)…"
	go run -ldflags "$(LDFLAGS)" -trimpath ./cmd/$(BINARY_NAME) -- $(ARGS)

copy:
	@for d in $(DIST_FOLDERS); do \
	  mkdir -p $(DIST_DIR)/$$d && cp -r $$d/* $(DIST_DIR)/$$d/ 2>/dev/null || true; \
	done

cross-build: clean
	$(foreach os,windows linux darwin, \
	  $(foreach arch,amd64 arm64, \
	    GOOS=$(os) GOARCH=$(arch) go build -ldflags "$(LDFLAGS)" -trimpath \
	      -o $(DIST_DIR)/$(os)/$(arch)/$(BINARY_NAME)$(if $(filter $(os),windows),.exe,) ./cmd/$(BINARY_NAME) && \
	    $(MAKE) copy; \
	  ) \
	)

clean:
	go clean
	rm -rf $(DIST_DIR)/*