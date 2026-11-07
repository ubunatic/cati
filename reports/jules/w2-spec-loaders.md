# w2-spec-loaders

## Findings

### 1. spec/load.go:155
- **Severity:** M
- **Effort:** M
- **Smallest next step:** Add a package-level cache in `spec/` for parsed specs or change `LoadButtons` to accept an `fs.FS` parameter like `LoadStyleFrom` to allow injecting a mock or cached file system.
- **Evidence:**
  ```go
  func LoadButtons() (ButtonsSpec, error) {
  ```

### 2. cmd/browser_config.go:236
- **Severity:** S
- **Effort:** S
- **Smallest next step:** Consolidate `loadButtonActions`, `loadAltButtonActions`, `loadButtonPriorities`, `loadButtonKeyDefs`, and `loadButtons` into a single `loadButtonConfig` function that calls `spec.LoadButtons()` once and returns a combined struct.
- **Evidence:**
  ```go
	btnSpec, err := spec.LoadButtons()
  ```

### 3. spec/load.go:117
- **Severity:** S
- **Effort:** S
- **Smallest next step:** Update `LoadTheme`, `LoadLabels`, `LoadButtons`, `LoadViews`, and `LoadZoomLevels` to follow the `Load*From(fsys fs.FS)` pattern used by `LoadStyle`, `LoadRenderModes`, `LoadControls`, and `LoadConfigDefaults`.
- **Evidence:**
  ```go
  func LoadTheme() (ThemeSpec, error) {
  ```

### 4. cmd/browser_config.go:483
- **Severity:** S
- **Effort:** S
- **Smallest next step:** Pass the configuration directory path as an argument to `loadConfig` and `saveConfig` instead of repeatedly calling `getConfigDir()` which calls `os.UserHomeDir()`.
- **Evidence:**
  ```go
	dir := getConfigDir()
  ```

### 5. cmd/browser_config.go:494
- **Severity:** S
- **Effort:** S
- **Smallest next step:** Move the `applyConfigOverrides` logic out of `browser_config.go` into a dedicated config parser package or testable struct method, as it contains logic for parsing key-value overrides.
- **Evidence:**
  ```go
  func applyConfigOverrides(cfg Settings, data []byte) Settings {
  ```
