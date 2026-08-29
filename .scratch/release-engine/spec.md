# Spec: Release Engine (core generation + backend publishing)

Status: ready-for-agent

## Problem Statement

Maintainers who commit to Conventional Commits want release notes generated automatically as they merge to their default branch, the way Release Drafter does for GitHub PRs — a live draft that's always current, finalized with one click when they're ready to ship. But their repos aren't always PR-driven (some commits land via direct push or rebase-merge), and their hosting isn't always GitHub — some run Gitea or Forgejo, self-hosted. Existing tools force a choice: Release Drafter's continuous-draft UX only works on GitHub and only understands PRs; commit-log tools like git-cliff work on any backend but are stateless, one-shot generators with no live draft and no backend publishing step. Monorepo maintainers additionally need this per-package, not per-repo — a change to `api` shouldn't force a version bump on `shared`.

## Solution

A CLI (`releaser`) that parses Conventional Commits directly (not PR metadata) to build one changelog Entry per commit, maintains a continuously-updated Draft Release on the backend (GitHub, Gitea, or Forgejo — chosen explicitly, not auto-detected) as commits land, and lets a human publish it when ready. Monorepos configure path-prefix → Package mappings; each Package gets its own versioned changelog and, in `multi` Release Mode, its own Draft Release and tag sequence. A `.releaser.yml` config drives tag format, category mapping, and the changelog body template. Three commands cover the lifecycle: `draft` (upsert the live draft), `preview` (same computation, printed instead of published, no credentials required), `publish` (promote a draft to a real release and tag it). A GitHub composite Action wraps the binary for easy CI wiring.

## User Stories

1. As a maintainer using Conventional Commits, I want every commit parsed into a changelog Entry, so that I don't have to manually curate release notes.
2. As a maintainer who doesn't merge every change through a PR, I want changelog generation to work from commits alone, so that direct pushes and rebase-merges are represented too.
3. As a maintainer, I want a live Draft Release that updates on every push to the default branch, so that the changelog is always current without me running anything by hand.
4. As a maintainer, I want to review the draft and publish it myself, so that nothing ships without my explicit sign-off.
5. As a maintainer, I want `releaser publish` to tag the release and promote the draft, so that I can finalize a release from CI without touching the backend UI.
6. As a Gitea user, I want the same continuous-draft workflow GitHub users get, so that switching hosting doesn't mean losing this tooling.
7. As a Forgejo user, I want the same continuous-draft workflow, so that self-hosting doesn't mean losing this tooling.
8. As a maintainer, I want my commit's originating PR linked in the changelog when one exists, so that readers can jump to the discussion/review.
9. As a maintainer on a backend where PR-linkage is unreliable (Forgejo) or unsupported (Gitea) via API, I want the tool to skip the link rather than attach a wrong one, so that my changelog stays trustworthy.
10. As a maintainer who squash-merges, I want the PR reference extracted from my commit message automatically (no extra config), so that linkage works out of the box.
11. As a GitHub maintainer whose commits don't carry a text PR reference (rebase-merge), I want a live API lookup as a fallback, so that linkage still happens where the backend supports it reliably.
12. As a monorepo maintainer, I want commits mapped to a Package by the paths they touch, so that each component's changelog only shows what actually changed in it.
13. As a monorepo maintainer, I want a commit that touches multiple Packages to appear in every affected Package's changelog, so that no reader misses a change relevant to them.
14. As a monorepo maintainer, I want each Package versioned and tagged independently (`api-v1.2.0`, `shared-v0.3.1`), so that an unrelated Package's release doesn't force my version to bump.
15. As a maintainer of a single-package repo, I want one Draft Release with one repo-wide version, so that I don't have to think about monorepo concepts I don't need.
16. As a maintainer, I want to declare `mode: single` or `mode: multi` explicitly in config, so that the tool's behavior is predictable and not guessed from repo shape.
17. As a maintainer, I want the next version computed automatically from commit types (`fix`→patch, `feat`→minor, breaking→major), so that I don't do SemVer arithmetic by hand.
18. As a maintainer, I want to override the computed version at publish time, so that I retain final say when the automatic suggestion is wrong.
19. As a maintainer, I want the tag naming pattern configurable (e.g. `{{package}}-v{{version}}`), so that it matches conventions my org already has.
20. As a maintainer, I want the tool to find the correct "since last release" boundary per Package by matching its configured tag pattern, so that ranges never mix up one Package's history with another's.
21. As a maintainer, I want commit types grouped into named sections (Features, Bug Fixes, ...) with sensible defaults, so that the changelog is readable without config.
22. As a maintainer, I want to remap or add to the type→section mapping, so that I can use commit types or names specific to my project.
23. As a maintainer, I want unmapped commit types caught in an "Other" section by default, so that nothing silently disappears without me choosing that.
24. As a maintainer, I want to mark an individual commit to be excluded from the changelog via a footer trailer (e.g. `Skip-Changelog: true`), so that noise commits (version bumps, merge artifacts) don't pollute release notes.
25. As a maintainer, I want the skip-changelog trailer key configurable, so that it can match an existing convention in my commits.
26. As a maintainer, I want a single `.releaser.yml` at the repo root (not a GitHub-specific path), so that the same config works regardless of which backend I'm on.
27. As a maintainer, I want a `preview` command that prints computed notes to stdout without needing write credentials, so that I can dry-run changes locally or in a read-only CI check.
28. As a maintainer, I want the changelog body's overall structure customizable via a free-form template, so that I can match my project's existing changelog style.
29. As a maintainer, I want a sane default template out of the box, so that I don't have to write one before getting useful output.
30. As a maintainer, I want the tool to never write or commit a `CHANGELOG.md` file itself, so that there's exactly one source of truth (the backend Draft Release) and no sync drift.
31. As a GitHub Actions user, I want a composite Action wrapping the binary, so that I don't have to hand-write download/install steps in my workflow.
32. As a GitHub Actions user, I want to select the subcommand (`draft`/`preview`/`publish`) via an Action input, so that one Action covers my whole workflow.
33. As a platform engineer rolling this out org-wide, I want the backend explicitly declared (flag or config), so that behavior is deterministic across self-hosted and cloud instances that can't be told apart by URL alone.
34. As a maintainer, I want clear, non-silent failure when commits don't parse as Conventional Commits under my declared setup, so that changelog quality issues surface immediately instead of shipping degraded notes.

## Implementation Decisions

**Test seams (confirmed with user):**
- Primary seam: `Engine.Plan(ctx, repoPath, cfg, opts) (*Plan, error)` — the single entry point for all pure computation: reads a real git repo's log/tags (no mocked git — tests use real temp git repo fixtures with real commits), parses Conventional Commits, resolves Packages by path prefix, applies cross-Package duplication, computes the SemVer suggestion, buckets Entries into categories, renders the template. Takes a `Backend` interface for optional PR-reference enrichment.
- Secondary seam: the `Backend` interface itself (`UpsertDraft`, `Publish`, and best-effort `ResolvePR`) — GitHub/Gitea/Forgejo adapters do real network I/O and are each tested against their own `httptest` fixture, independent of `Plan`.
- Commands (`draft`/`preview`/`publish`, already scaffolded as stubs in `internal/commands/`) stay thin: call `Plan`, then either print (`preview`) or call `Backend.UpsertDraft`/`Backend.Publish` (`draft`/`publish`). No business logic lives in the command layer.

**Commit parsing (ADR-0003):** one Entry per commit, parsed as Conventional Commits (type, optional scope, `!`/`BREAKING CHANGE:` for breaking, subject, footers). PR data is enrichment only, never required for an Entry to exist.

**PR Reference resolution (ADR-0001):** try text extraction first — GitHub squash-merge `(#N)` suffix, Gitea/Forgejo squash-merge `Reviewed-on: .../pulls/N` trailer. If empty, fall back to a live `Backend.ResolvePR` API call **only on GitHub** (`commits/{sha}/pulls`, confirmed reliable). No fallback on Gitea (no such endpoint) or Forgejo (endpoint exists but confirmed unreliable) — a commit with no resolvable reference ships with no PR link, never a guessed one.

**Package resolution:** config maps path-prefix → package name. A commit's changed files (via `git diff --name-only`) determine its Package(s), purely from git — no backend API involved. A commit touching multiple Packages' prefixes produces a duplicate Entry in each affected Package.

**Release Mode (ADR-0004):** `.releaser.yml` declares `mode: single | multi`. `single` = one Draft Release, one repo-wide computed version, Packages used only to section Entries within the body. `multi` = one Draft Release per Package, each with its own computed version and tag sequence.

**Version computation:** SemVer bump derived from the highest-severity commit type in range (breaking > feat > fix), computed automatically; shown as a suggestion, overridable via `publish --version`.

**Tag format:** configurable template string in `.releaser.yml` (e.g. `{{package}}-v{{version}}` for `multi`, `v{{version}}` for `single`). The tool derives a matcher from the template's placeholders to find the previous tag per Package — no free-form/unparseable tag formats.

**Config file:** `.releaser.yml` at repo root, backend-agnostic (not under `.github/`). Fields: `mode`, `packages` (path-prefix → name), `tag-format`, `categories` (type → section name/order, with a built-in default set), `skip-changelog-trailer` (default key `Skip-Changelog`), `template` (free-form, with a built-in default).

**Backend selection:** explicit only — `--backend github|gitea|forgejo` flag or config field. No auto-detection from git remote URL (self-hosted Gitea/Forgejo are indistinguishable by host alone). Token via `--token` flag or `RELEASER_TOKEN`/backend-specific env var, per `internal/commands/shared` flags already scaffolded.

**CLI commands (continuous draft model, ADR-0002):** `draft` (idempotent upsert of the live Draft Release, run on every push to default branch), `preview` (same `Plan()` output, printed, no backend credentials required), `publish` (promote Draft Release to published, tag it). Already scaffolded in `internal/commands/{draft,preview,publish}` as stubs — this spec fills in their `Action` bodies via `Engine.Plan` and the `Backend` interface.

**No CHANGELOG.md file:** the tool never writes or commits a file. The backend Draft Release is the only output. A committed changelog file, if wanted, is CI's responsibility on publish (out of scope here).

**Distribution:** Go, `urfave/cli/v3` (already scaffolded), Goreleaser. A GitHub composite Action (single `action.yml`, `command` input selecting `draft`/`preview`/`publish`, downloads the pinned Goreleaser binary) wraps the binary for GitHub Actions users; Marketplace-publishable per confirmed requirements (public repo, unique name, Developer Agreement, explicit `shell:` on run steps).

## Testing Decisions

- Tests target external behavior at the two seams above, not internals — no mocking of git itself; `Plan()` tests run against real temporary git repositories seeded with real commits (branches, tags, multi-package file layouts) and assert on the resulting `Plan` (Entries, sections, suggested version), per the project's "prefer in-memory stubs / real fixtures over brittle internal mocks" testing practice.
- `Backend` interface: exercised via a fake in-memory implementation for `Plan()`'s enrichment path (no network in domain-logic tests), and via `httptest`-backed contract tests per concrete adapter (GitHub, Gitea, Forgejo) asserting each adapter satisfies the same interface behavior against realistic API responses/fixtures.
- Commit parsing, Package resolution, version computation, tag-pattern matching, category mapping, and skip-changelog-trailer handling are all exercised through `Plan()` as black-box scenarios (given this commit history + this config, expect this `Plan`) rather than unit-tested as isolated internal functions.
- Command layer (`draft`/`preview`/`publish` `Action` functions): thin, tested only for correct flag→`Plan`/`Backend` call wiring, not for domain logic duplication.
- No prior art in this repo yet (greenfield) — the scaffolded stub commands and `internal/commands/shared` flags are the only existing code this spec builds on.

## Out of Scope

- Backend auto-detection from git remote URL (Q14 — explicit selection only).
- PR-label-based skip-changelog signal (Q10 resolution — footer trailer only).
- A Forgejo/Gitea API fallback for PR-reference resolution (ADR-0001 — confirmed unreliable/absent, explicitly excluded, not just deferred).
- Writing/committing a `CHANGELOG.md` file (Q16 — backend Draft Release is the only output).
- Backends beyond GitHub, Gitea, and Forgejo (e.g. GitLab, Bitbucket).
- Tooling to migrate a repo from `single` to `multi` Release Mode (ADR-0004 notes this is a real re-partitioning, not a flag flip — no migration assistant planned).
- Gitea/Forgejo Actions packaging (only a GitHub composite Action is in scope here, though ADR-0002's Action design is expected to be YAML-compatible with Forgejo Actions as a side effect, unverified/untested).
- The full template placeholder set/DSL design — this spec establishes that free-form templating with a sane default exists; the exact placeholder vocabulary is an implementation detail for whoever picks up the templating Entry.

## Further Notes

- Full design rationale lives in `docs/adr/0001` through `0004` and the project glossary in `CONTEXT.md` — read both before implementing; this spec uses their vocabulary (Entry, Package, Draft Release, Release Mode, Backend, PR Reference) throughout and assumes it.
- The CLI shell (`cmd/releaser`, `internal/app`, `internal/commands/{draft,preview,publish,root,shared}`) already exists as stubs returning "not yet implemented" — this spec is scoped to replacing those stubs with real `Engine.Plan`/`Backend` wiring, plus the new `internal/config`, `internal/git`, `internal/commit`, `internal/backend` (interface + 3 adapters), and `internal/engine` packages that don't exist yet.
- The GitHub composite Action (`action.yml`) is a separate, smaller deliverable that depends on this spec's CLI surface being stable — sequence it after, not in parallel.
