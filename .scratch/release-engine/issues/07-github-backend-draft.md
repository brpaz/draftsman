# 07 — Backend interface + GitHub adapter: `draft` upserts a real Draft Release

**What to build:** `releaser draft --backend github --token <token>` creates a GitHub draft release if none exists for the current (single-mode) version, or updates its body if one does — idempotent on repeated runs, using the same `Plan()` computation `preview` already renders. This is the first ticket introducing real network I/O.

**Blocked by:** 02, 04

- [ ] `internal/backend` defines the `Backend` interface (`UpsertDraft`, `Publish`, best-effort `ResolvePR` — the last one may be a stub returning "not supported" until ticket 08) that all backend adapters (this ticket's GitHub, and tickets 11/12's Gitea/Forgejo) implement identically
- [ ] GitHub adapter implements `UpsertDraft` against the GitHub releases API: finds an existing draft release for the computed tag, updates its body, or creates one if absent
- [ ] `internal/commands/draft`'s `Action` calls `Plan()` then `Backend.UpsertDraft` instead of returning "not yet implemented"; `--backend github` selects this adapter
- [ ] Running `releaser draft --backend github` twice in a row against the same repo state is idempotent — second run doesn't duplicate or error
- [ ] GitHub adapter tested via `httptest`-backed fixtures exercising create-when-absent and update-when-present, independent of `Plan()`
