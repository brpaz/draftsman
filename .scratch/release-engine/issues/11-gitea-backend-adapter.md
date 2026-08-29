# 11 — Gitea backend adapter

**What to build:** `releaser draft`/`releaser publish --backend gitea` work against a real Gitea instance, implementing the same `Backend` interface as the GitHub adapter (ticket 07). `ResolvePR` is not implemented for Gitea (returns "not supported") — per ADR-0001, Gitea has no commit→PR API; only ticket 05's text extraction applies.

**Blocked by:** 05, 07

- [x] Gitea adapter implements `UpsertDraft` and `Publish` against Gitea's release API (draft flag supported, confirmed during design)
- [x] Gitea adapter's `ResolvePR` explicitly returns "not supported" rather than attempting an unreliable/nonexistent lookup — `Plan()` treats this the same as "no reference found," never an error
- [ ] `--backend gitea` selects this adapter end-to-end through `draft`/`publish`/`preview`
- [ ] Tested via `httptest` fixtures mirroring ticket 07's coverage (create-when-absent, update-when-present, publish, no-matching-draft error)
