# draftsman

CLI tool that generates release notes from Conventional Commits, maintaining a continuously-updated draft release across GitHub, GitLab, Gitea, and Forgejo.

Unlike stateless generators, draftsman mirrors [Release Drafter](https://github.com/release-drafter/release-drafter)'s model: a live draft release is upserted on the backend on every push to the default branch, and finalized later — by a human, or by `draftsman publish` — rather than generated once on demand. See [ADR-0002](adr/0002-continuous-draft-model.md) for why.

## Where to start

- **New to draftsman?** → [Installation](installation.md), then [Provider setup](providers/index.md) for the backend you're targeting.
- **Configuring an existing setup?** → [Configuration](configuration.md) for the full `.draftsman.yml` reference.
- **Wiring it into CI?** → [Provider setup](providers/index.md) has GitHub Actions, GitLab CI, Gitea Actions, and Forgejo Actions examples per backend.
- **Using the CLI directly?** → [CLI reference](cli.md).
- **Trunk-based, GitHub Flow, or Git Flow?** → [Git Workflows](workflows.md) covers which branch to trigger on and what merge strategy to use for each.
- **Coming from another tool?** → [How it compares](#how-it-compares-with-other-release-notes-tools) below, or the [migration guide from release-drafter](migrating/from-release-drafter.md) if that's you.
- **Curious how it's built, or contributing code?** → [Development](development/index.md) for environment setup, [Architecture](development/architecture.md), and the ADRs behind the non-obvious choices.

## Core ideas

- **One Entry per commit**, parsed as a Conventional Commit — not per-PR, so it works identically for squash-merge, rebase-merge, and direct-push workflows. See [ADR-0003](adr/0003-commit-based-entries.md).
- **PR references are optional enrichment**, resolved from commit-message text where the merge strategy embeds it, with a live API fallback on GitHub and GitLab, where it's reliable. See [ADR-0001](adr/0001-pr-linkage-strategy.md).
- **Monorepos are a config choice, not the default shape** — a single-package repo pays no monorepo tax; a repo that opts into `mode: multi` gets one independent Draft Release, version, and tag per Package. See [ADR-0004](adr/0004-single-vs-multi-release-mode.md).

## How it compares with other release notes tools?

### At a glance

✅ built-in and default · ⚠️ possible, but not the default / partial · ❌ not supported

| Capability | draftsman | release-please | release-drafter | semantic-release |
| --- | :---: | :---: | :---: | :---: |
| Conventional-Commit-driven categorization | ✅ | ✅ | ⚠️ opt-in per category | ✅ Angular preset by default |
| Works with direct-push / rebase-merge (no PR needed) | ✅ | ✅ | ❌ | ✅ |
| Continuously-updated Draft Release | ✅ | ❌ uses a Release PR instead | ✅ | ❌ no draft concept — publishes atomically per run |
| Automatic version bump, no manual labeling | ✅ | ✅ | ⚠️ label-based by default | ✅ |
| Updates in-repo version files (`package.json`, etc.) | ❌ | ✅ | ❌ | ⚠️ via plugins (`@semantic-release/npm`, etc.), not built-in |
| Monorepo / multi-package support | ✅ | ✅ | ❌ | ⚠️ community plugin (`multi-semantic-release`), not core |
| Self-hosted backends (Gitea, Forgejo) | ✅ | ❌ | ❌ | ⚠️ plugin-based, not built-in |
| Usable as a local CLI outside CI | ✅ | ⚠️ npm CLI exists, built around the GitHub PR flow | ❌ Action-only | ✅ npm CLI, `--dry-run` for local preview |

### Mechanics in detail

| | **draftsman** | **release-please** | **release-drafter** | **semantic-release** |
| --- | --- | --- | --- | --- |
| Changelog entry source | One Entry per commit, parsed as a [Conventional Commit](https://www.conventionalcommits.org/) | One Entry per commit, parsed as a Conventional Commit | One entry per **merged pull request** — PR title/labels, not commits | One entry per commit, Angular convention by default (configurable analyzer) |
| Release mechanism | Continuously-updated **Draft Release**, finalized by `draftsman publish` or the backend UI | **Release PR** — a PR containing the changelog + version bump; merging it creates the GitHub release | Continuously-updated **Draft Release**, finalized via the GitHub UI | Fully automated pipeline — analyze, version, notes, and **publish, all atomically in one CI run**; no pending/draft state (`--dry-run` previews without publishing) |
| Works with direct-push / rebase-merge | Yes — no PR required for an entry to exist | Yes — commit-based, same as draftsman | No — needs a merged PR to have anything to categorize | Yes — commit-based, same as draftsman |
| Version bump source | Conventional Commit types in range (breaking → major, `feat` → minor, else → patch) | Same: Conventional Commit types in range | PR labels (`major`/`minor`/`patch`), or an opt-in `conventional` matcher on PR title | Same: Conventional Commit types in range (Angular preset, swappable) |
| Updates in-repo version files (`package.json`, `Cargo.toml`, etc.) | No — computes and tags a version, doesn't rewrite manifest files | **Yes** — this is release-please's defining feature | No | Yes, via plugins (`@semantic-release/npm` updates `package.json`, `@semantic-release/git` commits it back) — opt-in per plugin, not automatic |
| Monorepo support | `packages`: path-prefix → name, independent Draft Release/version/tag per package in `multi` mode | `release-please-config.json` + manifest: independent version per component | No first-class concept — one config per repo | Not in core; the community `multi-semantic-release` package runs the pipeline per package |
| Git backends | GitHub, GitLab, Gitea, Forgejo behind one interface | GitHub only | GitHub only | GitHub, GitLab, Bitbucket via official plugins; others via community plugins |
| PR linkage | Optional enrichment: text extraction from commit trailers, live API fallback on GitHub | Inherent — GitHub-native, commit-derived | Inherent — PRs are the primary data source, not enrichment | Not tracked — changelog is commit-message text only |
| Config file | `.draftsman.yml` | `release-please-config.json` + `.release-please-manifest.json` | `.github/release-drafter.yml` | `.releaserc.json`/`.yml` or `release.config.js` |
| Distribution | CLI binary (Nix, Go install, Docker, prebuilt binaries) + GitHub composite Action | GitHub Action / npm CLI | GitHub Action only | npm CLI (Node.js required) |

### Picking one

- **Publish packages/libraries with in-repo version files that need bumping** (npm, Cargo, Maven, etc.) → **release-please** does this natively; draftsman and release-drafter don't touch version files at all.
- **Merge everything through labeled PRs, want changelog grouping by label** → **release-drafter**'s label-driven categorization is purpose-built for that workflow.
- **Want one CI run that computes, tags, and publishes atomically — no pending draft state — plus a plugin pipeline that can push to npm/PyPI/other registries** → **semantic-release** is built exactly for that. draftsman can approximate the atomic part (run `draft` then `publish` back-to-back in the same job — see [Git Workflows → no release object at all](workflows.md#no-release-object-at-all)), but has no plugin system and never publishes packages anywhere.
- **Want Conventional-Commit-driven changelogs that work identically for squash-merge, rebase-merge, or direct-push, across GitHub, GitLab, *and* self-hosted Gitea/Forgejo, with first-class monorepo support and no Node.js toolchain requirement** → that's draftsman's specific niche; see [ADR-0002](adr/0002-continuous-draft-model.md) and [ADR-0003](adr/0003-commit-based-entries.md) for the reasoning.

Already on release-drafter and considering a switch? See the [migration guide](migrating/from-release-drafter.md).
