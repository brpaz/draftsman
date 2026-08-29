# 06 — Multi Release Mode

**What to build:** With `mode: multi` set in `.releaser.yml`, each configured Package gets its own independently computed version and its own tag sequence (per ADR-0004) instead of one repo-wide version — `preview` shows N separate version/changelog computations, one per Package, each range-bounded by that Package's own last matching tag (using `{{package}}` in `tag-format`, e.g. `{{package}}-v{{version}}`).

**Blocked by:** 03, 04

- [ ] `mode` config field: `single` (default, ticket-01/04 behavior — one repo-wide version, Packages only section the body) or `multi`
- [ ] In `multi` mode, range-finding and version computation (ticket 04) run once per Package, scoped to that Package's Entries and that Package's own tag-format-matched tags
- [ ] `preview` output in `multi` mode presents each Package's suggested version and changelog independently
- [ ] `Plan()` tests cover: `multi` mode with 2+ packages at different version-bump levels (e.g. one package has only `fix`s, another has a `feat`) producing correctly independent suggestions and correctly independent tag ranges
