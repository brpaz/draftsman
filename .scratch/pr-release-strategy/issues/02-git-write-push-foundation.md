# 02 — Git-write + push foundation

**What to build:** A new local git-write capability — create/checkout a branch, write the file changes computed in ticket 01 (version-file bump + `CHANGELOG.md`), commit as draftsman's own bot identity, and push to a remote — verified against a local bare-repo fixture remote. Every existing git interaction in this codebase (`internal/git`) is read-only (log/diff/tags); this is the first write path.

**Blocked by:** 01.

**Status:** done

- [x] `internal/git` (or a new sibling package) gains a write API: create a branch at a given ref, write a set of file contents, commit with a fixed, recognizable bot author identity, and push the branch to a remote
- [x] Pushing authenticates using the same token already configured for backend auth (no new credential concept)
- [x] Re-invoking against a branch that already exists updates it in place (matches ticket 01's computed plan) rather than erroring or creating a duplicate
- [x] Tested against a real local git repository with a local bare-repo fixture as the "remote" — branch/commit/push state asserted by inspecting the bare repo afterward, no live backend involved
- [x] `go test ./...` passes
