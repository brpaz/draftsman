# 08 — GitHub PR API fallback enrichment

**What to build:** When ticket 05's text extraction finds no PR Reference on a commit, and `--backend github` is in use with a token, a live `commits/{sha}/pulls` API lookup resolves the PR Reference instead — GitHub only, per ADR-0001 (Gitea has no equivalent endpoint; Forgejo's is confirmed unreliable and deliberately excluded, not just deferred).

**Blocked by:** 05, 07

- [ ] `Backend.ResolvePR` implemented for the GitHub adapter using `commits/{sha}/pulls`
- [ ] `Plan()`'s PR-resolution step tries text extraction (05) first, falls back to `Backend.ResolvePR` only when the backend is GitHub and a token is present; other backends/no-token skip straight to "no reference" as before
- [ ] `preview --backend github --token ...` and `draft --backend github` both show/attach the API-resolved reference for a commit with no text-extractable one
- [ ] GitHub adapter's `ResolvePR` tested via `httptest` fixtures; `Plan()` tested with a fake `Backend` to confirm the fallback-only-on-GitHub rule
