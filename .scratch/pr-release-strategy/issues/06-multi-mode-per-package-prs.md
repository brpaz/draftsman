# 06 — Multi-mode: one release PR per package

**What to build:** In `mode: multi` with `release-strategy: pr`, both `draft` (open/update) and `publish` (create release) operate independently per package — one branch/PR/release each — mirroring how `multi` mode already keeps packages fully independent under `release-strategy: draft`.

**Blocked by:** 03 and 05.

**Status:** done

- [x] `draft`, under `mode: multi` + `release-strategy: pr`, loops over every package with a pending release and opens/updates one branch/PR per package (branch naming stays stable and package-scoped, per ticket 03)
- [x] `publish`, under the same combination, supports the existing `--package` scoping (publish one package's already-merged release) and the existing "no `--package`" behavior (publish every package with a pending merged release), consistent with `release-strategy: draft`'s existing multi-mode publish semantics
- [x] A commit touching multiple packages produces the expected bump/changelog change in every affected package's branch, not just one
- [x] Human-edit backoff (ticket 04) applies independently per package branch
- [x] Tested against a multi-package fixture repo: two packages, one with a human-edited branch (skipped) and one without (updated), asserting each is handled independently
- [x] `go test ./...` passes
