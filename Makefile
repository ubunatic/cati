.PHONY: ⚙️  # make all commands phony
BINARY  := cati
PREFIX  ?= /usr/local
DEMO_DIR = ../emojig/spec/art/frames
DEMO     = $(DEMO_DIR)/emojig_fall_*.png

help: ⚙️  ## show this help
	@grep -E '^[a-zA-Z_-]+:.*##' $(MAKEFILE_LIST) | \
	awk 'BEGIN {FS = ":.*## "}; {printf "  %-10s %s\n", $$1, $$2}'

dev: ⚙️ install test  ## build, test, run demo
	cati -i assets

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

reuse: ⚙️  ## verify license compliance linting
	reuse lint


demo-widths: ⚙️ install  ## render main demo assets at widths 1..6, printed as a table
	go run scripts/demo_widths.go

preflight: ⚙️ install  ## pre-commit checks: vet + verify demo-widths renders without errors
	go vet ./...
	@echo "Checking demo-widths for render errors..."
	@go run scripts/demo_widths.go 2>&1 | grep -i "err\|panic\|fail" && echo "FAIL: render errors found" && exit 1 || echo "OK: no render errors"

generate: ⚙️  ## generate static assets/code (e.g., inlined docs/index.html pixel colors)
	go run scripts/generate_pixels.go

tidy: ⚙️  ## tidy go modules
	go mod tidy

clean: ⚙️  ## remove build artifacts
	rm -f $(BINARY)


browse: ⚙️  ## open website
	open docs/index.html

edit-baby: ⚙️  ## open baby vid in Kdenlive
	open assets/baby.kdenlive

open-assets: ⚙️  ## open assets in file browser
	open assets
