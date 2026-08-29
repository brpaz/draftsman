# Migrating from release-drafter

The mechanical setup swap is small — replace the Action, translate the config. The real migration is a **process** change: release-drafter categorizes by PR label, draftsman categorizes by Conventional Commit type. Read the [comparison](../README.md#how-it-compares-with-other-release-notes-tools) first if you haven't — this guide assumes you've decided the switch makes sense.

Doing this by hand is one option. There's also a [Claude Code skill](https://github.com/brpaz/draftsman/tree/main/skills/migrate-from-release-drafter) that walks through the same steps as an agent — detects your existing `.github/release-drafter.yml`, proposes the category translation, validates with `draftsman preview` against your real history, and swaps the workflow file:

```shell
npx skills add brpaz/draftsman --skill migrate-from-release-drafter --agent claude-code
```

## Before you start: commit discipline

release-drafter doesn't care what your commit messages look like — it reads PR titles and labels. draftsman reads commit messages directly and needs them to be [Conventional Commits](https://www.conventionalcommits.org/) (`feat: ...`, `fix: ...`, `feat!: ...`, etc.) to categorize correctly. If your repo squash-merges PRs with a free-form title as the commit message, start enforcing Conventional Commit PR titles (e.g. via [commitlint](https://commitlint.js.org/) or a GitHub Actions PR-title check) *before* switching — otherwise everything lands in the uncategorized "Other" section.

## Config translation

release-drafter's `.github/release-drafter.yml` and draftsman's `.draftsman.yml` solve overlapping problems differently. There's no 1:1 field mapping for everything — some release-drafter features have no draftsman equivalent (called out below).

| release-drafter | draftsman | Notes |
| --- | --- | --- |
| `tag-template: 'v$RESOLVED_VERSION'` | `tag-format: "v{{version}}"` | Same idea, different placeholder syntax. `{{version}}` in draftsman is always the raw resolved version. |
| `name-template: 'v$RESOLVED_VERSION 🌈'` | *(none)* | draftsman doesn't template a separate release title — the title is always the tag. Drop any custom title decoration. |
| `categories: [{ title: '🚀 Features', label: 'feature' }]` | `categories: [{ type: feat, section: Features }]` | The key change: match on the commit's **Conventional Commit type**, not a PR **label**. Map each label-based category to its Conventional Commit type equivalent — see the table below. |
| `exclude-labels: ['skip-changelog']` | `skip-changelog-trailer: Skip-Changelog` | Behavior change: this is now a commit-message footer trailer, not a GitHub label — `Skip-Changelog: true` in the commit body, not a label applied to the PR. |
| `version-resolver` (labels → major/minor/patch) | *(automatic)* | draftsman computes the bump from commit types itself (breaking → major, `feat` → minor, else → patch) — no manual labeling step, and no config for it. |
| `autolabeler` | *(none)* | draftsman doesn't label PRs; it doesn't need to, since it reads commits directly. Drop this section entirely. |
| `template: '$CHANGES'` / `$CONTRIBUTORS` / `$PREVIOUS_TAG` | `template:` (Go `text/template`) | Different templating engine and variable set — see [Configuration → template](../configuration.md#template). The built-in default covers the common case; port custom formatting by hand. |
| `replacers` | *(none)* | No equivalent regex-replace pass over the rendered body. If you rely on this for e.g. redacting or rewriting text, that step has to move outside draftsman (e.g. a post-processing script before `draftsman publish` picks up the draft — not currently supported as a hook). |
| Prerelease workflow (branch-based) | *(none)* | draftsman has no prerelease concept. Not currently a supported migration path. |

**Common category mapping**, translating the typical release-drafter label set into Conventional Commit types:

```yaml
categories:
  - type: feat
    section: Features
  - type: fix
    section: Bug Fixes
  - type: perf
    section: Performance
  - type: refactor
    section: Refactors
  - type: docs
    section: Documentation
```

Full field reference: [Configuration](../configuration.md).

## Workflow file

**Before** (release-drafter):

```yaml
name: Release Drafter

on:
  push:
    branches: [main]
  pull_request:
    types: [opened, reopened, synchronize] # only needed for autolabeler

jobs:
  update_release_draft:
    runs-on: ubuntu-latest
    steps:
      - uses: release-drafter/release-drafter@v6
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

**After** (draftsman):

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
          fetch-depth: 0 # full history — draftsman walks commits since the last tag
      - uses: brpaz/draftsman@v1
        with:
          command: draft
          backend: github
          token: ${{ secrets.GITHUB_TOKEN }}
```

The `pull_request` trigger is gone — it existed only to feed release-drafter's autolabeler, which draftsman has no equivalent of. `fetch-depth: 0` is required now (draftsman walks commit history locally rather than querying the GitHub API for merged PRs), where release-drafter didn't need it.

## Publishing

No change to your team's workflow here: both tools leave the draft on the backend for a human to publish via the UI. If you want to automate that step too, add a `draftsman publish` job — see [CLI reference](../cli.md#draftsman-publish) — release-drafter has no equivalent built-in publish command.

## Checklist

- [ ] Start enforcing Conventional Commit messages (or PR titles, if you squash-merge) before cutting over.
- [ ] Translate `categories` from label-based to type-based (table above).
- [ ] Move any `Skip-Changelog`-equivalent label to a commit-footer trailer.
- [ ] Drop `autolabeler`, `version-resolver`, `replacers` — no equivalent; confirm nothing downstream depends on them.
- [ ] Port any custom `template` to draftsman's `text/template` syntax, or keep the built-in default.
- [ ] Swap the workflow file (above); remove the now-unneeded `pull_request` trigger.
- [ ] Run `draftsman preview` against your existing commit history first to sanity-check categorization before wiring up `draft`.
