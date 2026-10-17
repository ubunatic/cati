# 101 — Add GitHub Actions CI for build, vet and tests

**Status**: In Progress — Implement CI and container test parity
**Priority**: P1 (High)
**Severity**: Moderate
**Category**: Infrastructure
**Related**: [100](100-skip-media-tests-when-git-lfs-assets-are-pointer-files.md), [Testing](../docs/Testing.md), `Makefile`, `Dockerfile`

## Problem

The repository had no automated checks, so pull requests could merge without
build, vet, formatting, or test verification. The existing Docker test path
used a stale Go version and required Docker directly.

## Requirements

- Run CI for pushes to `main` and pull requests, with read-only permissions,
  ref-scoped cancellation, major-pinned actions, Go version from `go.mod`, and
  no secrets.
- Pull LFS assets in CI. Real media checks skip missing fixtures, pointer files,
  or unavailable tools through `internal/testutil.RequireMedia`.
- Run gofmt, `go vet ./...`, `go build ./...`, both fastpath test modes, and
  `go test -race ./...` on Linux.
- Provide a podman-first/docker-fallback container test path running the same
  checks. The container is read-only except for `/tmp`; only the designated
  output directory is writable from the host.
- Container-specific skip: `internal/audio.TestPlayAudio` checks
  `CATI_CONTAINER_TEST=1` and skips audio output. The PTY test
  `TestInteractiveSmallGradientViaPTY` also skips with that flag and is tagged
  `integration`, outside the default suite. No default tests require GPU or
  network access.

## Verification

- `make container-test` could not start in the current tool environment:
  rootless podman is denied namespace cloning, and the quota wrapper cannot see
  the host podman socket. The target must still be run on a host where podman or
  docker is permitted before closing this ticket.
- Validate `.github/workflows/ci.yaml` with actionlint or PyYAML.
- Make the CI test and REUSE jobs required status checks in repository settings
  after the workflow is merged.
