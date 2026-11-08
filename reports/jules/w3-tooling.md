## Verdict

The `cati` repository maintains good tooling hygiene, but there are a few areas for cleanup and modularization, particularly around stale build configurations, caching, and release targets.

## Evidence

1. **Stale `fastpath0` double-run in `.github/workflows/ci.yaml`:**
   `.github/workflows/ci.yaml:104`
   `          CATI_FASTPATH=0 go test -tags="fastpath0,$TAGS" -json ./... >> full.json 2>&1`
   *Why it helps:* Issue 113 standardized the fastpath architecture and removed the `-tags=fastpath0` double-run from `Makefile`. The CI workflow still redundantly runs the test suite a second time. Removing it halves the `full` CI job time and CPU load.
   *Smallest step:* Delete the `CATI_FASTPATH=0 ...` line in the `full` job.

2. **Module cache key in CI uses `github.sha`, defeating the cache:**
   `.github/workflows/ci.yaml:44`
   `          key: go-lint-${{ hashFiles('go.sum') }}-${{ github.sha }}`
   *Why it helps:* The primary cache `key` includes `github.sha`, meaning the cache is *never* hit on a new commit (it always creates a new key and falls back to `restore-keys`). The cache should be shared across commits unless `go.sum` changes.
   *Smallest step:* Remove `-${{ github.sha }}` from the `key` line across all CI jobs.

3. **Missing `make release` target in `Makefile`:**
   `Makefile:61`
   `check: ⚙️  ## run checks and ensure install-web leaves the checkout clean`
   *Why it helps:* `docs/GoRelease.md:91` explicitly defines the release step as: "2.5 `Makefile` target — thin, one line... `release: check ⚙️  # release the project using harnez \n harnez release`" but the `Makefile` itself lacks this target where it should be (next to `check`).
   *Smallest step:* Add `release: check ⚙️  ## release the project using harnez` followed by `	harnez release` to `Makefile`.

4. **Missing `-tags=fastpath0` cleanup in `.github/workflows/ci.yaml` full job names:**
   `.github/workflows/ci.yaml:101`
   `      - name: go test (default and fastpath0)`
   *Why it helps:* The step title still references the deleted fastpath architecture, which should be removed along with the command itself.
   *Smallest step:* Rename the step and the summary argument on line 108 to `go test`.

## Smallest next step

Create a PR to remove the `-tags=fastpath0` run in the CI workflow, fix the CI cache keys, and add the missing `release` target to the `Makefile`.
