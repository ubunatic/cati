# w3-spec-hardcode

## Verdict
I audited the `cati` repository for hardcoded values in Go that should reside in the `spec/` system, adhering strictly to the `CLAUDE.md` rules. I identified multiple instances where keys, defaults, colors, and CLI strings are hardcoded in Go rather than loaded from the single source of truth in `spec/*.yaml`.

## Evidence

1. **`cmd/browser.go:907` (Hardcoded UI Action string)**
   `path:line` `cmd/browser.go:907`
   `									case "quit":`
   *Severity:* High
   *Effort:* M
   *Why it helps:* Actions dispatched from UI events should be driven by the action definitions mapped in the spec system. Hardcoding action strings makes the application brittle and harder to reconfigure.
   *Smallest next step:* Ensure the action strings are strictly matched against the loaded spec definitions, or use an enumerated constant generated from the spec.

2. **`cmd/modes_cards.go:86` (Hardcoded Mode Sort Keys)**
   `path:line` `cmd/modes_cards.go:86`
   `	case "ssim", "psnr", "best", "quality":`
   *Severity:* Moderate
   *Effort:* M
   *Why it helps:* Mode sorting and filtering aliases are hardcoded in multiple switch statements. Moving these to a spec file ensures the CLI logic does not duplicate spec values and makes adding new sort options easier.
   *Smallest next step:* Define sort/filter aliases in `spec/cli.yaml` and load them into a map for sorting modes.

3. **`cmd/modes_cards.go:100` (Hardcoded Mode Sort Keys)**
   `path:line` `cmd/modes_cards.go:100`
   `	case "time", "dur", "fastest":`
   *Severity:* Moderate
   *Effort:* M
   *Why it helps:* Mode sorting and filtering aliases are hardcoded in multiple switch statements. Moving these to a spec file ensures the CLI logic does not duplicate spec values and makes adding new sort options easier.
   *Smallest next step:* Define sort/filter aliases in `spec/cli.yaml` and load them into a map for sorting modes.

4. **`cmd/modes.go:209` (Hardcoded CLI Flag Description and Values)**
   `path:line` `cmd/modes.go:209`
   `	cmd.Flags().StringVar(&sortOrder, "sort", "", "sort modes by: ssim (highest SSIM first), -ssim, time (fastest), -time, eff (or efficiency: lowest (1-ssim)*time), -eff, name, -name")`
   *Severity:* Moderate
   *Effort:* M
   *Why it helps:* `CLAUDE.md` specifies that CLI defaults live in the spec. Hardcoding the flag description and valid choices duplicates what should be in the spec, leading to potential drift between documentation and implementation.
   *Smallest next step:* Move the description and available choices to `spec/cli.yaml`.

5. **`cmd/root.go:147` (Hardcoded ANSI CLI Flag)**
   `path:line` `cmd/root.go:147`
   `	root.Flags().BoolVar(&ansiMode, "ansi", false, "render with 24-bit ANSI true-color")`
   *Severity:* Low
   *Effort:* S
   *Why it helps:* `CLAUDE.md` specifies that CLI flags and defaults live in the spec. Hardcoding the flag description duplicates what should be in the spec.
   *Smallest next step:* Move the description and available choices to `spec/cli.yaml`.

## Smallest next step
Move mode sort keys from `cmd/modes_cards.go` to `spec/cli.yaml` and load them dynamically.

## Ticket corrections
None.
