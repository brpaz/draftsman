# 09 — `publish` command: GitHub

**What to build:** `releaser publish --backend github --token <token>` promotes the existing draft release to published and tags it at the computed (or `--version`-overridden) version — the human-triggered finalization step of the continuous-draft workflow (ADR-0002).

**Blocked by:** 04, 07

- [x] `Backend.Publish` implemented for the GitHub adapter: finds the draft release matching the computed tag, flips it to published, ensures the tag exists at the current default-branch HEAD
- [x] `internal/commands/publish`'s `Action` calls `Plan()` then `Backend.Publish` instead of returning "not yet implemented"
- [x] `--version` flag (already scaffolded) overrides the computed suggestion — the tag/release use the override, not the auto-computed value, when set
- [x] Publishing when no matching draft exists produces a clear error, not a silent no-op or a wrong-release publish
- [x] GitHub adapter's `Publish` tested via `httptest` fixtures covering: normal promote, version-override promote, and no-matching-draft error
