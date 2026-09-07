# 07 — Gitea backend adapter

**What to build:** The full `release-strategy: pr` flow (release PR open/update, and `CreateRelease` at publish time) works against a Gitea instance — this is the primary motivating gap this feature exists for.

**Blocked by:** 03 and 05.

**Status:** done

- [x] Gitea adapter implements the PR create-or-update method from ticket 03 against Gitea's pull request API
- [x] Gitea adapter implements `CreateRelease` from ticket 05 against Gitea's release API
- [x] Both are tested via `httptest` fixtures, same pattern as the GitHub adapter's coverage
- [x] `draft` and `publish` under `release-strategy: pr` work end-to-end against the Gitea adapter with no branching in the command layer beyond backend selection (same command code path as GitHub, just a different `Backend` implementation)
- [x] `go test ./...` passes
