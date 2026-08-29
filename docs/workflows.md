# Git workflows

draftsman has no concept of a "default branch" or a git workflow built in — `draftsman draft`/`draftsman publish` just read whatever's checked out at `HEAD` when they run (`internal/git.Log` walks `HEAD`, or `since..HEAD`; `Tags` are `git tag --merged HEAD`). Which workflow you're using only affects two things you control from the outside: **which branch your CI trigger runs `draft` on**, and **what merge strategy lands commits on that branch** (since [ADR-0003](adr/0003-commit-based-entries.md) means every individual commit reachable from `HEAD` is a candidate Entry — `git log` isn't restricted to `--first-parent`).

## Merge strategy matters more than the workflow name

Whichever workflow below you use, this is the actual decision that shapes your changelog:

- **Squash-merge** (GitHub's default) — one commit per PR on the target branch. If the PR title is a Conventional Commit, you get exactly one clean Entry per PR, and GitHub's `(#N)` title suffix gives PR linkage for free (see [ADR-0001](adr/0001-pr-linkage-strategy.md)).
- **Rebase-merge** — every commit from the branch lands individually, in its original form. Fine if your team writes Conventional Commits per-commit; noisy otherwise. No text-extractable PR reference (falls back to the GitHub API, if configured).
- **Merge commit** (`--no-ff`, git flow's traditional default) — same as rebase-merge (every branch commit lands individually) *plus* the merge commit itself, which isn't a Conventional Commit and gets silently dropped (by design, per ADR-0003). No PR reference from text; GitHub API fallback still works.

If your team doesn't squash-merge, commit hygiene on feature branches (no `wip`/`fixup!` commits reaching the target branch, or a Conventional-Commits-compliant final commit) matters as much as picking a workflow.

## Trunk-based development

One long-lived branch (`main`/`trunk`), short-lived feature branches merged frequently, often several times a day. This is the workflow draftsman's continuous-draft model ([ADR-0002](adr/0002-continuous-draft-model.md)) was built around — draft on every push, publish whenever you're ready to cut a release.

```yaml
name: Update draft release

on:
  push:
    branches: [main]

permissions:
  contents: write

jobs:
  draft:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: brpaz/draftsman@v1
        with:
          command: draft
          backend: github
          token: ${{ secrets.GITHUB_TOKEN }}
```

- `mode: single` is the natural fit unless the repo is also a monorepo — that's an orthogonal choice (see [Configuration → mode](configuration.md#mode)).
- Direct pushes to `main` work identically to merged PRs (ADR-0003) — trunk-based teams that skip PRs for small changes lose nothing.
- Publish either on demand (`workflow_dispatch`, see the [CLI reference](cli.md#draftsman-publish)) or on a schedule, depending on how often you actually want to cut releases versus just keep the draft current.

## GitHub Flow

Effectively trunk-based development with PRs as the mandatory unit of change: `main` is always deployable, every change goes through a PR, and there's no `develop`/release-branch layer. The setup is identical to trunk-based development above — same trigger, same `mode` choice.

The one thing worth being deliberate about here is merge strategy, since GitHub Flow is where it's most visible: teams following it are usually squash-merging by convention, which is also the config that gets you free PR linkage (GitHub's `(#N)` suffix) with zero extra setup. If your repo's branch protection doesn't already enforce "squash and merge" as the only merge option, that's worth locking down before wiring up draftsman, not after — inconsistent merge strategies produce an inconsistent changelog.

## Git Flow

Two long-lived branches (`main`, `develop`) plus supporting branches (`feature/*`, `release/*`, `hotfix/*`). Releases are periodic, not continuous — cut from a `release/*` branch, merged into `main` (tagged) and back into `develop`.

**Only trigger `draft`/`publish` on pushes to `main`, never `develop`:**

```yaml
name: Update draft release

on:
  push:
    branches: [main]

permissions:
  contents: write

jobs:
  draft:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: brpaz/draftsman@v1
        with:
          command: draft
          backend: github
          token: ${{ secrets.GITHUB_TOKEN }}
```

Why this matters more here than in the other two workflows:

- `develop` accumulates commits between releases that haven't shipped yet. Running `draftsman draft` against `develop`'s `HEAD` would draft a release for work that isn't actually ready to cut — `main` only advances when a `release/*` or `hotfix/*` branch merges in, which is exactly the cadence you want a draft release to track.
- Tag range-finding (`git tag --merged HEAD`) only considers tags reachable from whatever's checked out. As long as you only ever run draftsman against `main`, this is correct automatically — no extra config needed, just don't add a second workflow triggered on `develop`.
- Traditional Git Flow uses `--no-ff` merge commits for `release/*` → `main`, which brings in every individual commit from the release branch (and anything merged into it from `feature/*` branches along the way) — see [merge strategy](#merge-strategy-matters-more-than-the-workflow-name) above. If those feature branches were merged into `develop` with messy, non-Conventional-Commit messages, expect a noisier changelog than trunk-based/GitHub Flow produces; squash-merging feature branches into `develop` fixes this even though the outer `release/*` → `main` merge stays a merge commit.
- Hotfix branches merging directly into `main` trigger the same `draft` workflow — no separate handling needed, since it's just another push to `main`.

`mode`, `packages`, and everything else in [Configuration](configuration.md) work identically to the other two workflows — none of it is workflow-specific.

## No release object at all

Everything above assumes you want a GitHub/GitLab/Gitea/Forgejo Draft Release. Two variants for teams that don't, layered on top of any workflow above — pick based on how far you want to go:

### Atomic publish, no lingering draft

`draftsman publish` requires a matching draft to already exist — `internal/backend/github/github.go`'s `Publish` errors `"no draft release found for tag"` if you call it cold. But nothing requires a time gap between the two: run `draft` immediately followed by `publish`, same job, on every push. The draft exists for the duration of one CI job and gets promoted before anyone sees it — functionally atomic, closer to how [semantic-release](README.md#how-it-compares-with-other-release-notes-tools) behaves (compute → tag → publish in one run), without changing anything about `mode`/`packages`/config:

```yaml
jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: brpaz/draftsman@v1
        with:
          command: draft
          backend: github
          token: ${{ secrets.GITHUB_TOKEN }}
      - uses: brpaz/draftsman@v1
        with:
          command: publish
          backend: github
          token: ${{ secrets.GITHUB_TOKEN }}
```

A release object still gets created on the backend (this doesn't skip that) — it just never sits in `draft: true` state for a human to review first.

### No release object, no tag, just changelog text

`draftsman preview` never calls the backend for writes — no `UpsertDraft`, no `Publish`, confirmed in `internal/commands/preview/preview.go`: it computes the `Plan` and writes to stdout, nothing else. `--backend`/`--token`/`--repo` stay fully optional (only used for GitHub PR-link enrichment, not required). Pipe the output wherever you want it — a committed `CHANGELOG.md`, a PR comment, a Slack message:

```shell
draftsman preview >> CHANGELOG.md
```

Two things to know before leaning on this as a repeatable pattern:

- **Range-finding is tag-driven, with no other state mechanism.** `latestMatchingTag` (`internal/engine/engine.go`) bounds "since the last release" purely by scanning existing tags matching `tag-format` — there's no separate marker file or last-run state. If no tags matching `tag-format` exist anywhere in the repo (not created by draftsman, not created manually, not created by anything else), every `preview` run recomputes from the *start of history*, not incrementally. Fine for a one-off "notes for this range" invocation; not fine for "what's new since I last checked," run repeatedly, unless *something* is tagging the repo — it doesn't have to be `draftsman draft`/`publish`, a plain `git tag` in CI works just as well as the boundary marker.
- **No built-in file-writing mode.** draftsman doesn't append to or maintain `CHANGELOG.md` itself — only stdout. The `>>` above, plus committing the result, is on you to wire up; there's no equivalent to semantic-release's `@semantic-release/changelog` + `@semantic-release/git` plugin pair that do this automatically.
