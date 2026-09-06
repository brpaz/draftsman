# 01 — Config: `release-strategy` field + version-file bump plan in `preview`

**What to build:** `.draftsman.yml` accepts a new `release-strategy: draft | pr` field (default `draft`, orthogonal to `mode`). Under `release-strategy: pr`, `draftsman preview` shows which version file would be bumped and to what value, plus the `CHANGELOG.md` diff that would be committed — pure computation, no git writes, no backend calls. `release-strategy: draft` (the default) is completely unaffected.

**Blocked by:** None — can start immediately.

- [ ] `internal/config.Config` gains `ReleaseStrategy` (`"draft"`/`"pr"`, default `"draft"`), defaulted/overridden the same way every other field is in `Default()`/`Load()`
- [ ] Per-package version-file detection: given a package directory, detect in priority order `package.json` (JSON `"version"` key), `Cargo.toml` (TOML `version` under `[package]`), `pyproject.toml` (TOML `version` under `[project]` or `[tool.poetry]`), `composer.json` (JSON `"version"` key), `go.mod` (explicit no-op — no file write), falling back to a plain-text `VERSION` file when nothing else matches
- [ ] A `Package` config entry can explicitly override the detected version-file path and format, taking precedence over auto-detection
- [ ] A pure computation function takes a package + its detected/overridden version file + the suggested next version and returns: the new file content for the version file (or "no-op" for Go), and the new `CHANGELOG.md` content with the release's entry prepended
- [ ] `internal/commands/preview`'s output, under `release-strategy: pr`, includes the computed version-file change (path, old → new value) and the `CHANGELOG.md` diff for each package with a pending release; under `release-strategy: draft` output is byte-for-byte unchanged from today
- [ ] Format detection and content computation are tested against real temp-directory fixtures per format (each manifest format, the Go no-op, the `VERSION` fallback, and the explicit-override path) — no mocked file parsing
- [ ] `go test ./...` passes
