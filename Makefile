BINARY := nyt
PKG := gitea.jonn.me/jons-org/nyt_cli
VERSION ?= dev
LDFLAGS := -X $(PKG)/cmd.Version=$(VERSION)
SKILL_DIR := .claude/skills/nyt-cli

# Cross-compile targets (os/arch). Override with PLATFORMS="linux/amd64 ...".
PLATFORMS ?= darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64

.PHONY: build install test vet fmt lint tidy clean run release skill

build: ## Build the ./nyt binary
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

install: ## Install nyt onto your PATH
	go install -ldflags "-X $(PKG)/cmd.Version=$(VERSION)" .

test: ## Run the unit tests
	go test ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Format all Go files
	gofmt -w .

tidy: ## Tidy go.mod/go.sum
	go mod tidy

release: ## Cross-compile binaries into dist/ for all PLATFORMS
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; arch=$${platform#*/}; \
		ext=""; [ "$$os" = "windows" ] && ext=".exe"; \
		out="dist/$(BINARY)-$$os-$$arch$$ext"; \
		echo "building $$out"; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
			go build -ldflags "$(LDFLAGS)" -o "$$out" . || exit 1; \
	done

skill: ## Build the darwin/arm64 binary into the vendored skill's bin/
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 \
		go build -ldflags "$(LDFLAGS)" -o "$(SKILL_DIR)/bin/$(BINARY)" .

clean: ## Remove build artifacts
	rm -f $(BINARY)
	rm -rf dist

run: build ## Build then run (use ARGS="topstories home")
	./$(BINARY) $(ARGS)
