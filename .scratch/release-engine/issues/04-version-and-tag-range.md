# 04 — Version computation + tag-format range-finding

**What to build:** `preview` shows a computed SemVer suggestion (highest-severity commit type in range wins: breaking > feat > fix) for the repo-wide version. The "since last release" commit range comes from the last git tag matching a configurable tag-format template (e.g. `v{{version}}`), not the full repo history — the tool derives a matcher from the template's placeholders rather than accepting free-form unparseable formats.

**Blocked by:** 02

- [ ] `tag-format` config field (template string with `{{version}}` placeholder at minimum; `{{package}}` placeholder recognized but inert until ticket 06/07 add multi-mode)
- [ ] SemVer bump computed from the set of Entries in range: any breaking change → major, else any `feat` → minor, else any `fix` → patch, else no suggested bump
- [ ] Range-finding locates the latest reachable tag matching the configured `tag-format` pattern and computes Entries only for commits after it (first-ever release: full history)
- [ ] `preview` output includes the computed suggested version alongside the changelog body
- [ ] `Plan()` tests cover: no prior tag (full history), a prior matching tag (bounded range), and a repo tag that doesn't match the configured format (ignored, not mistaken for a release tag)
