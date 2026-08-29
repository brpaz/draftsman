# 02 — `.releaser.yml` config loading + skip-changelog trailer

**What to build:** A repo-root `.releaser.yml` overrides the built-in defaults from ticket 01 — `mode`, `categories` (type→section name/order), the skip-changelog trailer key, and the changelog `template`. Absent config keeps the ticket-01 built-ins. Additionally, any commit carrying the configured skip-changelog trailer (default `Skip-Changelog: true`) is excluded from `preview` output entirely.

**Blocked by:** 01

- [ ] `internal/config` loads and validates `.releaser.yml`, defaulting every field when the file or a field is absent
- [ ] `--config` flag (already scaffolded on all three commands) is honored — a missing file at the default path (`.releaser.yml`) is not an error, an explicit `--config` path that doesn't exist is
- [ ] `categories` config remaps/reorders/adds to the type→section mapping; unmapped types still land in "Other"
- [ ] `template` config overrides the built-in default template used to render `Plan` output
- [ ] Skip-changelog trailer key is configurable; a commit whose footer contains that trailer (matched case-appropriately per git trailer conventions) produces no Entry, verified via `preview` output
- [ ] `Plan()` tests cover: custom category mapping changes sectioning, custom template changes rendered output, skip-trailer commits are absent from the result — all via real git fixtures + config fixtures, no internals mocked
