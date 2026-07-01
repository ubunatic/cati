.PHONY: ⚙️  # make all commands phony
BINARY  := cati
PREFIX  ?= /usr/local
DEMO_DIR = ../emojig/spec/art/frames
DEMO     = $(DEMO_DIR)/emojig_fall_*.png
DEMO_WIDTH ?= 30
DEMO_STEPS ?= 2
VIDEO      ?= assets/baby-360p.mp4
VIDEO_AT   ?= 1s

export GOCACHE ?= /tmp/cati-gocache

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


demo-widths: ⚙️ build  ## render main demo assets as terminal comparison tables
	go run scripts/demo_widths.go -bin ./$(BINARY)

demo-darth: ⚙️ build  ## render the Darth Daughter sample at scaled demo widths
	@go run scripts/demo_widths.go -bin ./$(BINARY) -w $(DEMO_WIDTH) -n $(DEMO_STEPS) -i darth=assets/samples/sample-003-darth-daughter.jpg

demo-solder: ⚙️ build  ## render the soldering practice sample at scaled demo widths
	@go run scripts/demo_widths.go -bin ./$(BINARY) -w $(DEMO_WIDTH) -n $(DEMO_STEPS) -i solder=assets/samples/sample-001-soldering-practice-2025.jpg

demo-vacation: ⚙️ build  ## render the summer vacation sample at scaled demo widths
	@go run scripts/demo_widths.go -bin ./$(BINARY) -w $(DEMO_WIDTH) -n $(DEMO_STEPS) -i vacation=assets/samples/sample-002-summer-vacation.jpg


demo-baby: ⚙️ build  ## compare video frames from baby-360p.mp4 across all render modes (VIDEO_AT=t1,t2,... VIDEO=path)
	go run scripts/demo_widths.go -bin ./$(BINARY) -w $(DEMO_WIDTH) -n $(DEMO_STEPS) -v baby=$(VIDEO) -at $(VIDEO_AT)

preflight: ⚙️ build  ## pre-commit checks: vet + verify demo-widths renders without errors
	go vet ./...
	@echo "Checking demo-widths for render errors..."
	@go run scripts/demo_widths.go -bin ./$(BINARY) 2>&1 | grep -i "err\|panic\|fail" && echo "FAIL: render errors found" && exit 1 || echo "OK: no render errors"

generate: ⚙️  ## generate static assets/code (e.g., inlined website/index.html pixel colors)
	go run scripts/generate_pixels.go
	go run scripts/generate_summary.go

tidy: ⚙️  ## tidy go modules
	go mod tidy

clean: ⚙️  ## remove build artifacts
	rm -f $(BINARY)
	rm -r website/book

book: ⚙️  ## build the mdbook documentation
	go run scripts/generate_summary.go
	mdbook build
	@echo "✅ Book built (run: 'make serve' to start server)"

serve: ⚙️  # start book webserver (not for website)
	@killall -q mdbook || true
	mdbook serve

browse: ⚙️ book ## open website
	open website/index.html 2>/dev/null

edit-baby: ⚙️  ## open baby vid in Kdenlive
	open assets/baby.kdenlive

open-assets: ⚙️  ## open assets in file browser
	open assets
