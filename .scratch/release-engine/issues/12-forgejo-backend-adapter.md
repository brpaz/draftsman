# 12 — Forgejo backend adapter

**What to build:** `releaser draft`/`releaser publish --backend forgejo` work against a real Forgejo instance, implementing the same `Backend` interface (ticket 07). `ResolvePR` is not implemented for Forgejo either — per ADR-0001, Forgejo's commit→PR endpoint exists but is confirmed unreliable (returns wrong PRs in some cases), and using it risks attaching an incorrect link, which is worse than none. Only ticket 05's text extraction applies.

**Blocked by:** 05, 07

- [x] Forgejo adapter implements `UpsertDraft` and `Publish` against Forgejo's release API (draft flag supported, confirmed during design; Forgejo's API is Gitea-derived but should be verified/tested against Forgejo directly, not assumed identical — verified against Codeberg's live swagger.v1.json: same field names, base path, auth scheme, and pagination params as Gitea)
- [x] Forgejo adapter's `ResolvePR` explicitly returns "not supported" — deliberate exclusion, not a missing feature to revisit casually (test asserts the `commits/{sha}/pull` endpoint, confirmed to exist on Forgejo, is never called)
- [ ] `--backend forgejo` selects this adapter end-to-end through `draft`/`publish`/`preview`
- [ ] Tested via `httptest` fixtures mirroring ticket 07's coverage
