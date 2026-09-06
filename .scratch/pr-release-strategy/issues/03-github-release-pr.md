# 03 — GitHub: release PR open/update

**What to build:** Running `draftsman draft` under `release-strategy: pr` against a GitHub repo pushes the release branch (via ticket 02) and opens a PR for it if none is open, or updates the existing one's branch/body if it already exists — the first true end-to-end demo of the draft side of this feature.

**Blocked by:** 02.

**Status:** done

- [x] `backend.Backend` gains a PR create-or-update method (find an existing open PR for a branch, or open a new one; update title/body of an existing one) — implemented for the GitHub adapter first, other adapters get a not-yet-implemented stub
- [x] `internal/commands/draft`'s `Action`, under `release-strategy: pr`, computes the plan (ticket 01), pushes the branch (ticket 02), and calls the new Backend method instead of `UpsertDraft`
- [x] PR title/body reflect the computed version and rendered changelog notes (reuses `Engine.Plan`'s existing rendering/template mechanism)
- [x] Branch name is stable and deterministic per package (not versioned), so re-running against the same pending release updates the same PR rather than opening a new one
- [x] GitHub adapter's PR create-or-update is tested via `httptest` fixtures, same pattern as the existing `UpsertDraft`/`Publish`/`ResolvePR` coverage
- [x] Running `draftsman draft` twice against a fixture with no new commits between runs results in one PR, updated in place, not two
- [x] `go test ./...` passes
