#####################
# make inspectre
#####################
BINARY_NAME=inspectre
VERSION_PATH=github.com/thushan/inspectre/internal/version
VERSION=$(shell git describe --tags --always --dirty)
GCOMMIT=$(shell git rev-parse --short HEAD)
TODAY=$(shell date --iso-8601=seconds)

PLATFORMS=linux darwin
ARCHITECTURES=amd64 arm64
DIST_FOLDERS=configs plugins examples

.PHONY: all
all: test cross-build

# all: lint test cross-build

.PHONY: ready
ready: lint test

.PHONY: lint
lint:
	golangci-lint run -c .golangci.yml ./...

.PHONY: test
test:
	go test -v ./...

.PHONY: cross-build
cross-build: clean
	$(foreach PLATFORM,$(PLATFORMS),\
		$(foreach ARCH,$(ARCHITECTURES),\
			$(MAKE) build-platform GOOS=$(PLATFORM) GOARCH=$(ARCH);)\
	)
	$(MAKE) copy-additional-folders

.PHONY: build-platform
build-platform:
	CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(GOARCH) go build \
		-ldflags " -X ${VERSION_PATH}.Date=$(TODAY) \
				   -X ${VERSION_PATH}.User=make \
				   -X ${VERSION_PATH}.Version=$(VERSION) \
				   -X ${VERSION_PATH}.Commit=$(GCOMMIT) \
				   -s -w" \
		-trimpath \
		-o dist/$(BINARY_NAME)-$(GOOS)-$(GOARCH)/$(BINARY_NAME)$(if $(filter windows,$(GOOS)),.exe,) .

.PHONY: copy-additional-folders
copy-additional-folders:
	@echo "Copying additional folders to dist..."
	@for dir in $(wildcard dist/$(BINARY_NAME)-*); do \
		if [ -d "$$dir" ]; then \
			$(foreach FOLDER,$(DIST_FOLDERS),\
				mkdir -p "$$dir/$(FOLDER)"; \
				cp -r "$(FOLDER)"/* "$$dir/$(FOLDER)/" 2>/dev/null || true;)\
		fi \
	done

.PHONY: build
build:
	CGO_ENABLED=1 go build -ldflags " -X ${VERSION_PATH}.Date=$(TODAY) \
                      -X ${VERSION_PATH}.User=make \
                      -X ${VERSION_PATH}.Version=$(VERSION) \
                      -X ${VERSION_PATH}.Commit=$(GCOMMIT) \
                      -s -w" \
            -trimpath \
            -o dist/$(BINARY_NAME) .
	@$(foreach FOLDER,$(DIST_FOLDERS),\
		mkdir -p dist/$(FOLDER); \
		cp -r "$(FOLDER)"/* dist/$(FOLDER)/ 2>/dev/null || true;)

.PHONY: docker-build
docker-build:
	docker build -t inspectre:latest -f Dockerfile .

.PHONY: release
release:
	MSYS_NO_PATHCONV=1 docker run -ti -v "$(PWD):/app" -w "/app" goreleaser/goreleaser:latest release --snapshot --clean

.PHONY: clean
clean:
	go clean
	rm -rf dist/*

.PHONY: clean-reports
clean-reports:
	rm -rf reports/*

.PHONY: duckdb-deps
duckdb-deps:
	@echo "Installing DuckDB build dependencies"
	@if [ -f /etc/debian_version ]; then \
		apt-get update && apt-get install -y build-essential cmake libssl-dev; \
	elif [ -f /etc/redhat-release ]; then \
		yum install -y gcc gcc-c++ make cmake openssl-devel; \
	elif [ -f /etc/arch-release ]; then \
		pacman -Sy --noconfirm gcc make cmake openssl; \
	elif [ -x /usr/local/bin/brew ] || [ -x /opt/homebrew/bin/brew ]; then \
		brew install cmake openssl; \
	else \
		echo "Please install C/C++ build tools, cmake, and openssl development libraries for your system"; \
	fi