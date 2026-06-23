.PHONY: ⚙️  # make all commands phony
BINARY  := cati
PREFIX  ?= /usr/local
DEMO_DIR = ../emojig/spec/art/frames
DEMO     = $(DEMO_DIR)/emojig_fall_*.png

help: ⚙️  ## show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS = ":.*## "}; {printf "  %-10s %s\n", $$1, $$2}'

build: ⚙️  ## build the binary
	go build -o $(BINARY) .

run: ⚙️ build ## build and run (requires an image arg: make run IMG=foo.png)
	./$(BINARY) $(IMG)

demo: ⚙️ build ## play the emojig falling animation (q or Ctrl+C to stop)
	@if test -d $(DEMO_DIR); \
	 then ./$(BINARY) --play --fps 12 $(DEMO); \
	 else echo "No demo found in DEMO_DIR=$(DEMO_DIR)"; \
	 fi

logo: ⚙️ build ## animate the cati logo (q or Ctrl+C to stop)
	./$(BINARY) --play --fps 4 assets/

install: ⚙️ build  ## install to ~/go/bin (user)
	go install .

test: ⚙️  ## run linter and tests
	go vet ./...
	go test ./...

tidy: ⚙️  ## tidy go modules
	go mod tidy

clean: ⚙️  ## remove build artifacts
	rm -f $(BINARY)
