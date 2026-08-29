---
name: migrate-from-release-drafter
version: "1.0.0"
description: "Migrate a repo off release-drafter (release-drafter/release-drafter) onto draftsman (github.com/brpaz/draftsman) — translate .github/release-drafter.yml into .draftsman.yml, swap the GitHub Actions workflow, validate against real commit history, and surface features with no direct equivalent instead of silently dropping them. Use when asked to \"migrate from release-drafter\", \"replace release-drafter\", \"switch off release-drafter\", or when a repo has .github/release-drafter.yml and the user wants Conventional-Commit-driven release notes instead of PR-label-driven ones."
tags: [draftsman, release-drafter, migration, changelog, conventional-commits, github-actions, release-management]
---

# Migrate from release-drafter to draftsman

Use when a repo currently runs [release-drafter](https://github.com/release-drafter/release-drafter) and wants to switch to [draftsman](https://github.com/brpaz/draftsman). Full reference: [brpaz.github.io/draftsman/migrating/from-release-drafter](https://brpaz.github.io/draftsman/migrating/from-release-drafter/) and [brpaz.github.io/draftsman](https://brpaz.github.io/draftsman/).

The mechanical config/workflow swap is small. The real migration is a **process** change: release-drafter categorizes changelog entries by PR label; draftsman categorizes by [Conventional Commit](https://www.conventionalcommits.org/) type, parsed from commit messages directly (one Entry per commit, not per merged PR). Don't skip the commit-discipline step below — a config-only migration onto a repo that doesn't write Conventional Commits produces a changelog that's entirely "Other".

## Non-Goals

- Not a general Conventional Commits linting/enforcement setup — flag that it's needed, don't implement commitlint/PR-title-check tooling as part of this skill unless separately asked.
- Not for a from-scratch draftsman setup on a repo with no prior release-drafter config — that's just normal [installation](https://brpaz.github.io/draftsman/installation/) + [configuration](https://brpaz.github.io/draftsman/configuration/), this skill is specifically about translating an *existing* release-drafter setup.
- Doesn't invent a category mapping the user hasn't confirmed — use the translation table below as a starting proposal, but confirm the mapping (especially for custom/non-standard labels) rather than guessing silently.

## Procedure

### 1. Detect the existing setup

- Find `.github/release-drafter.yml` (or a custom path referenced via `with: config-name:` in the workflow). Read it in full before proposing anything.
- Find the workflow file invoking `release-drafter/release-drafter@...` (commonly `.github/workflows/release-drafter.yml` or similar) — note its trigger config (`push` branches, and whether `pull_request` is present for autolabeler).
- If neither exists, this isn't a migration — stop and point the user at the normal [installation](https://brpaz.github.io/draftsman/installation/) guide instead.

### 2. Check commit discipline before translating config

release-drafter reads PR titles/labels; draftsman reads commit messages as Conventional Commits. Check recent history (`git log --oneline -30`) against the target branch:

- If PRs are squash-merged with Conventional-Commit-style titles (`feat: ...`, `fix: ...`) already — good, proceed.
- If not — tell the user explicitly that switching now will dump most history into an "Other" section until commit messages (or squash-merge PR titles) follow Conventional Commits. Offer to continue anyway (their call — new commits going forward will still categorize correctly) rather than blocking.

### 3. Translate `.github/release-drafter.yml` → `.draftsman.yml`

| release-drafter | draftsman | Notes |
| --- | --- | --- |
| `tag-template: 'v$RESOLVED_VERSION'` | `tag-format: "v{{version}}"` | Same idea, different placeholder syntax. |
| `name-template` | *(none)* | draftsman's release title is always the tag — drop any custom title decoration. |
| `categories: [{ title, label }]` | `categories: [{ type, section }]` | Match each label-based category to its Conventional Commit type. Common mapping: `feature`/`enhancement` → `feat`, `fix`/`bug`/`bugfix` → `fix`, `documentation`/`docs` → `docs`, `performance` → `perf`. **Confirm this mapping with the user** for anything not in this common set — don't guess silently. |
| `exclude-labels: ['skip-changelog']` | `skip-changelog-trailer: Skip-Changelog` (footer trailer) **or** a `[skip changelog]` tag anywhere in the commit message | Behavior change: this moves from a GitHub label to a commit-message marker. Mention both options — the tag form works in a single `git commit -m` without needing a footer. |
| `version-resolver` (label → major/minor/patch) | *(automatic)* | draftsman computes the bump itself from commit types (breaking → major, `feat` → minor, else → patch) — drop this section entirely, no config needed. |
| `autolabeler` | *(none)* | draftsman doesn't label PRs. Drop this section; confirm nothing downstream (e.g. a separate workflow) depends on the labels it was applying. |
| `template` (`$CHANGES`, `$CONTRIBUTORS`, etc.) | `template:` (Go `text/template`, different variable set — see the [placeholder table](https://brpaz.github.io/draftsman/configuration/#available-placeholders)) | Port by hand if customized; otherwise omit and let draftsman's built-in default apply. |
| `replacers` | *(none — known gap)* | No regex-replace pass over the rendered body exists. If relied on, tell the user this step has no draftsman equivalent and must move outside the tool (e.g. a post-processing step before `draftsman publish`, not currently a supported hook). |
| Prerelease branch workflow | *(none — known gap)* | draftsman has no prerelease concept. Tell the user this isn't currently supported rather than approximating it. |

Write the resulting `.draftsman.yml` at the repo root. Add the schema modeline for editor validation:

```yaml
# yaml-language-server: $schema=https://brpaz.github.io/draftsman/schema/config.schema.json
```

### 4. Validate before touching CI

Before replacing anything, install draftsman locally (`nix run github:brpaz/draftsman -- --version`, `go install github.com/brpaz/draftsman/cmd/draftsman@latest`, or the `ghcr.io/brpaz/draftsman:latest` Docker image — pick whichever fits the repo's existing toolchain) and run:

```shell
draftsman preview
```

against the repo's real history. Check the categorization looks right — this is the sanity check for step 2/3 above, before wiring anything into CI.

### 5. Replace the workflow

Swap the release-drafter step for the draftsman composite Action. Key differences from a typical release-drafter workflow:

- Drop the `pull_request` trigger if it's only there for release-drafter's autolabeler (draftsman has no autolabeler).
- Add `fetch-depth: 0` to the checkout step — draftsman walks commit history locally, release-drafter queried the GitHub API for merged PRs and didn't need full history.

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

If the target repo isn't on GitHub (Gitea/Forgejo), see the provider-specific CI examples at [brpaz.github.io/draftsman/providers](https://brpaz.github.io/draftsman/providers/) instead of the snippet above.

### 6. Clean up

- Remove `.github/release-drafter.yml` and the old workflow file — confirm with the user before deleting rather than assuming.
- If `draftsman publish` should also be automated (release-drafter had no built-in equivalent — publishing was always manual via the GitHub UI), ask whether they want that as a separate on-demand workflow; don't add it unprompted.

## Report back

Summarize: the category mapping applied (and anything left for the user to confirm), any known-gap features (`autolabeler`, `version-resolver`, `replacers`, prerelease workflow) that were dropped with no equivalent, and the `draftsman preview` output so the user can see the real result before merging.
