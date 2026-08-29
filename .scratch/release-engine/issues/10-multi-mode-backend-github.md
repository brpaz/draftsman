# 10 — Multi-mode backend wiring: GitHub

**What to build:** In `mode: multi`, `releaser draft --backend github` and `releaser publish --backend github` manage N independent Draft Releases (one per Package, per ADR-0004) instead of one — each with its own tag (`{{package}}-v{{version}}`) and independent publish lifecycle. `--package <name>` (already scaffolded) scopes an invocation to a single Package's draft/publish when set; omitted, all Packages with pending changes are processed.

**Blocked by:** 06, 07, 09

- [x] `draft --backend github` in `multi` mode upserts one draft release per Package that has Entries since its last matching tag
- [x] `publish --backend github` in `multi` mode, with `--package <name>`, publishes only that Package's draft; without `--package`, behavior is explicit (process all pending Packages, or require `--package` — decide and document during implementation, since publishing every pending Package in one invocation is a real UX choice not yet locked)
- [x] Two Packages with independent version bumps produce two independently tagged, independently versioned releases on the same repo
- [x] Tested via fake-backend fixtures with 2+ packages, confirming no cross-package interference in tag/version selection (GitHub HTTP mechanics already covered by ticket 07/09's httptest suite in `internal/backend/github`; this ticket's own logic is the command-layer per-package iteration, not new HTTP calls)
