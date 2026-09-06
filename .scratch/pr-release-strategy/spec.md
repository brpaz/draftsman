# Spec: PR-based release strategy (version-bump release PRs)

Status: ready-for-agent

## Problem Statement

Some ecosystems (npm, Cargo, Python, PHP) require the release version to be physically written into a tracked manifest file (`package.json`, `Cargo.toml`, `pyproject.toml`, `composer.json`) before a release is cut — the tag alone isn't enough, downstream tooling reads the file. Maintainers on these ecosystems also want that bump reviewed like any other change, not pushed straight to the default branch by a bot.

release-please already solves this well, but only on GitHub. Draftsman's actual gap is Gitea and Forgejo (self-hosted), where no equivalent tool exists today. Draftsman's existing `draft` Release Mode (continuously-updated Draft Release object, no repo file writes, per ADR/spec decision "draftsman never writes a changelog file") doesn't cover this case at all — it has no concept of a version-bump commit, a PR, or a repo write path of any kind. Everything the tool does today is either a pure local git *read* (log/diff/tags) or a backend *API* write (`UpsertDraft`, `Publish`) — it never commits or pushes anything.

## Solution

A second Release Mode, `release-strategy: pr`, selectable per repo alongside the existing `mode: single|multi` axis (orthogonal: a repo can be `multi` + `pr`, `single` + `draft`, etc.). Default remains `draft` — the current continuous-draft behavior is unchanged and unaffected for any repo that doesn't set this field.

Under `release-strategy: pr`, `draftsman draft` (run on every push to the default branch, same trigger as today) opens or updates a **release PR per package** (one PR even in `single` mode, trivially "the one package") instead of upserting a Draft Release. The branch carries one commit: the version bump written into an auto-detected manifest file (or a plain `VERSION` file fallback) plus an updated `CHANGELOG.md` — both committed to the repo, which is a deliberate, scoped exception to the "no changelog file" rule for this strategy only. The PR body previews the same notes.

When a human merges that PR, a second, explicit CI step calls `draftsman publish` (existing command, unchanged trigger point — no auto-detection of "this push is a release-PR merge" inside `draft`). Under `pr` strategy, `publish` reads the version back out of the same manifest file it bumped (canonical, already merged) and the release body verbatim from the merged `CHANGELOG.md`'s top entry (guarantees the published release matches exactly what was reviewed, no recomputation drift), then calls a new `Backend.CreateRelease` to create an already-published release directly — there is no backend Draft Release object in this strategy to promote, since `Backend.Publish` only ever promotes a pre-existing draft.

## User Stories

1. As a maintainer on a self-hosted Gitea/Forgejo instance, I want a version-bump release PR flow, so that I get release-please-equivalent behavior where no such tool exists.
2. As a maintainer, I want the version bumped into my package's manifest file automatically, so that I don't hand-edit `package.json`/`Cargo.toml`/etc. on every release.
3. As a maintainer, I want the version bump to land via a PR I can review and merge, so that nothing is committed to my default branch without my sign-off.
4. As a maintainer, I want the release PR to stay up to date as more commits land before I merge it, so that I don't have to manually recreate it every time.
5. As a maintainer, I want draftsman to stop touching a release PR once I've manually edited it, so that my deliberate review changes aren't silently overwritten on the next push.
6. As a maintainer, I want merging the release PR to be the trigger for the actual tag/release (via an explicit publish step in my CI), so that the release only happens once, at a point I control.
7. As a maintainer, I want the published release's notes to exactly match what was in the merged PR, so that there's no surprise drift between review and publish.
8. As a maintainer of a Go package, I want version-bump detection to correctly do nothing to `go.mod` (which has no version field), so that the tool doesn't invent a bogus file write for an ecosystem that doesn't need one.
9. As a maintainer whose repo layout doesn't match auto-detection (e.g. multiple manifests in one package directory, or an unsupported format), I want to explicitly configure the version file per package, so that I'm not blocked by a wrong guess.
10. As a monorepo maintainer in `multi` mode, I want an independent release PR per package, so that reviewing/merging one package's release doesn't force a decision on another's.
11. As a maintainer already happy with continuous Draft Releases (`release-strategy: draft`, the default), I want zero behavior change, so that adopting this feature is strictly opt-in.

## Implementation Decisions

**Config — `release-strategy` (new field, repo-wide only, no per-package override):**
- `draft` (default): current behavior, unchanged.
- `pr`: version-bump release PR flow described here.
- Orthogonal to `mode` (`single`/`multi`) — all four combinations are valid.

**`draft` command under `release-strategy: pr`:**
- Per pending package (in `multi` mode, one independent PR per package with pending Entries; in `single` mode, one PR), upsert a release branch + PR:
  - Branch: stable, deterministic name per package (not versioned — it gets updated in place as more commits land before merge).
  - Commit: version bump written into the package's version file (see below) + updated `CHANGELOG.md`, authored by draftsman's own bot identity.
  - PR: created if none open for that branch; title/body reflect the computed version and rendered notes (reuses `Engine.Plan`'s existing rendering, same template mechanism as `draft` strategy).
- **Idempotent update, human-edit backoff:** on a later run, if the branch's HEAD commit was authored by draftsman's own bot identity, force-update it (recompute + overwrite) to reflect new commits landed since. If the HEAD commit was authored by anyone else (a human pushed a manual edit), stop auto-updating that branch/PR — leave it untouched until it's merged or closed.
- No backend Draft Release object is created in this strategy — the PR body and the committed `CHANGELOG.md` are the sole representation of pending release notes prior to merge.

**Version-file bump — auto-detection with explicit override:**
- Per package directory, detect (in some fixed priority order) a known manifest and apply a format-aware bump:
  - `package.json` → JSON `"version"` key.
  - `Cargo.toml` → TOML `version` key under `[package]`.
  - `pyproject.toml` → TOML `version` key (`[project]` or `[tool.poetry]`).
  - `composer.json` → JSON `"version"` key.
  - `go.mod` → recognized explicitly as a **no-op**: Go modules carry no in-repo version field (SemVer lives in git tags, with only the major-version suffix appearing in the module path for v2+); detecting a Go module means "this package's PR carries no version-file bump," not "skip the PR."
  - No recognized manifest → fall back to a plain-text `VERSION` file (created if absent, whole-file content replaced with the bare version string).
- **Override:** a `Package` config entry may explicitly set the version file path and format, taking precedence over auto-detection — for ambiguous directories (multiple manifests present) or formats not yet supported.
- The same file is read back at `publish` time as the authoritative version for the tag (not parsed from the changelog).

**Git write capability (new):** a new local git-write path — create/checkout branch, write file(s), commit, push — authenticated with the same token already configured for backend auth. This is new; every existing git interaction in the codebase is read-only (log/diff/tags via `internal/git`).

**Backend interface additions:**
- `CreateRelease(ctx, tag, name, body) error` — creates an already-published release directly, no pre-existing draft required. Used by `publish` under `release-strategy: pr` (parallel to, not replacing, the existing draft-then-`Publish` path used by `release-strategy: draft`).
- A PR/MR create-or-update capability (naming follows the codebase's existing convention of using "PR" as the umbrella term for GitHub PRs/GitLab MRs/Gitea & Forgejo PRs alike, per `PRReference` in `internal/commit`): open a PR for a branch if none exists for it, or return the existing open one; update an existing PR's title/body.
- Both are implemented per adapter (GitHub, GitLab, Gitea, Forgejo) like every other `Backend` method, even though the primary motivating gap is Gitea/Forgejo — GitHub/GitLab users remain free to use draftsman's PR strategy, but release-please is the recommended path for GitHub specifically (out of scope here to steer users toward either; this is a documentation note, not a code gate).

**`publish` command under `release-strategy: pr`:**
- Per package (same `--package` scoping semantics as today's multi-mode publish): read the version back from the package's version file (auto-detected or overridden path, same as `draft` used to write it), read the release body from the merged `CHANGELOG.md`'s newest entry for that package, compute the tag via the existing `tag-format` template, then call `Backend.CreateRelease`.
- No change to `release-strategy: draft`'s existing publish path (`Backend.Publish` promoting an existing draft) — the two strategies' publish logic branch cleanly on config, sharing tag-format rendering and CLI flag wiring only.
- The merge → publish trigger is an explicit second CI step the user wires up (e.g. "on PR merged to default branch, run `draftsman publish`") — `draft` does not auto-detect a release-PR merge and does not call `publish` itself.

## Testing Decisions

- Version-file detection/bump/read-back exercised as black-box scenarios against real temp-directory fixtures per format (`package.json`, `Cargo.toml`, `pyproject.toml`, `composer.json`, `go.mod` no-op, `VERSION` fallback, and the explicit-override path) — no mocking of file parsing internals.
- Git write path (branch/commit/push) tested against a real local git repository fixture (consistent with the project's existing "no mocked git" testing practice for `Engine.Plan`), asserting on resulting branch/commit state; push itself tested against a local bare-repo remote fixture, not a live backend.
- Human-edit backoff logic tested via commit-author-identity scenarios on a fixture branch (bot-authored HEAD → updates; human-authored HEAD → no-op), independent of any real backend.
- `Backend.CreateRelease` and the PR create/update capability: `httptest`-backed contract tests per adapter, same pattern as the existing `UpsertDraft`/`Publish`/`ResolvePR` coverage.
- `publish` under `release-strategy: pr`: black-box scenarios over a fixture repo with a merged release commit already present, asserting the correct version/tag/body are extracted and the right `Backend` calls made — via a fake in-memory `Backend`, consistent with `publish`'s existing test style.

## Out of Scope

- Auto-detecting that a given push to default *is* a release-PR merge, and auto-triggering publish from inside `draft` — an explicit second CI step is the only trigger (Q6 resolution).
- A backend-API-only ("create file via API", no local git checkout needed) implementation path — local git write + push is the only mechanism (Q13 resolution).
- Per-package `release-strategy` override — the setting is repo-wide only (Q15 resolution).
- Combining all pending packages into a single release PR in `multi` mode — always one PR per package (Q12 resolution).
- Steering or gating GitHub users away from this strategy in favor of release-please — a documentation recommendation only, not enforced in code.
- Version-file formats beyond the initial set (`package.json`, `Cargo.toml`, `pyproject.toml`, `composer.json`, `go.mod` no-op, `VERSION` fallback) — more formats are additive later without a design change, per the override escape hatch already covering unsupported formats.
- Reconciling an abandoned/closed (never merged) release PR beyond simply reopening a fresh one on the next `draft` run — no special cleanup or archival behavior specified.
- Recomputing/re-validating the release body via `Engine.Plan` at publish time as a consistency check against the merged `CHANGELOG.md` — the changelog file is trusted verbatim (Q8 resolution).

## Further Notes

- This spec **amends** the release-engine spec's decision #30 ("the tool never writes or commits a `CHANGELOG.md` file itself... no sync drift") — that rule continues to hold for `release-strategy: draft` (the default and, until now, the only strategy) but is explicitly not true for `release-strategy: pr`, where a committed `CHANGELOG.md` is the deliberate design (see `.scratch/release-engine/spec.md`).
- Scope is materially larger than a typical single-ticket change: a new git-write subsystem, two new `Backend` interface methods implemented across four adapters, and four format-aware version-file parsers plus a fallback. Expect this to break down into a ticket sequence similar in shape to `.scratch/release-engine/issues/`, not a single issue.
- The primary motivating gap is Gitea/Forgejo (self-hosted); GitHub users are better served by release-please today for the same problem — this shapes prioritization (Gitea/Forgejo adapters and their CI wiring first) but not the code surface, which covers all four backends uniformly.
