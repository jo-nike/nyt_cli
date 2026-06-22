BINARY := nyt
PKG := github.com/derter/nyt
VERSION ?= dev

.PHONY: build install test vet fmt lint tidy clean run

build: ## Build the ./nyt binary
	go build -ldflags "-X $(PKG)/cmd.Version=$(VERSION)" -o $(BINARY) .

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

clean: ## Remove build artifacts
	rm -f $(BINARY)

run: build ## Build then run (use ARGS="topstories home")
	./$(BINARY) $(ARGS)
