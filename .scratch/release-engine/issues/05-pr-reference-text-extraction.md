# 05 — PR Reference text extraction

**What to build:** `preview` shows a linked PR number on an Entry when it's extractable directly from the commit message text — GitHub squash-merge's `(#N)` title suffix, or Gitea/Forgejo squash-merge's `Reviewed-on: .../pulls/N` footer trailer. No backend credentials needed; this is pure text parsing per ADR-0001.

**Blocked by:** 01

- [ ] Per-backend-format regex/parser recognizes GitHub's `(#N)` suffix and Gitea/Forgejo's `Reviewed-on:` trailer, producing a PR Reference (number + inferred link) when present
- [ ] A commit with neither pattern present yields no PR Reference — no error, no guess
- [ ] `Plan()` attaches the resolved PR Reference to the corresponding Entry; `preview`'s rendered output includes it when present
- [ ] Tests cover both text formats matching, and the no-match case, via commit message fixtures (unit-level on the parser; also exercised through `Plan()` with real git fixtures for at least one case)
