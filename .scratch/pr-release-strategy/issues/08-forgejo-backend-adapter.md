# 08 — Forgejo backend adapter

**What to build:** The full `release-strategy: pr` flow (release PR open/update, and `CreateRelease` at publish time) works against a Forgejo instance, same shape as the Gitea adapter — the other primary motivating gap this feature exists for.

**Blocked by:** 03 and 05 (independent of, and can run in parallel with, 07).

**Status:** done

- [x] Forgejo adapter implements the PR create-or-update method from ticket 03 against Forgejo's pull request API
- [x] Forgejo adapter implements `CreateRelease` from ticket 05 against Forgejo's release API
- [x] Both are tested via `httptest` fixtures, same pattern as the GitHub adapter's coverage
- [x] `draft` and `publish` under `release-strategy: pr` work end-to-end against the Forgejo adapter with no branching in the command layer beyond backend selection (same command code path as GitHub/Gitea, just a different `Backend` implementation)
- [x] `go test ./...` passes
