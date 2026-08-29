# draftsman

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/brpaz/draftsman?style=for-the-badge)
![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/brpaz/draftsman/ci.yml?branch=main&style=for-the-badge)
![Go Report Card](https://goreportcard.com/badge/github.com/brpaz/draftsman?style=for-the-badge)
![License](https://img.shields.io/github/license/brpaz/draftsman?style=for-the-badge)

> CLI tool that generates release notes from Conventional Commits, maintaining a continuously-updated draft release across GitHub, Gitea, and Forgejo

## ✨ Features

- **Conventional Commits parsing** — one changelog entry per commit, no PR metadata required, so it works identically for squash-merge, rebase-merge, and direct-push workflows.
- **Continuous draft releases** — a live draft release is upserted on every push to the default branch (mirroring Release Drafter's model), finalized later via `draftsman publish` or the backend's own UI.
- **Multi-backend** — GitHub, Gitea, and Forgejo behind one common interface; switch backend with a single flag.
- **PR reference enrichment** — best-effort PR linkage extracted from commit-message text (squash-merge trailers), with a live API fallback on GitHub where it's reliable.
- **Monorepo aware** — map changed file paths to named Packages, each with its own changelog section (`single` mode) or its own independent Draft Release, version, and tag (`multi` mode).
- **Automatic SemVer** — the next version is computed from the Conventional Commit types in range (breaking → major, `feat` → minor, `fix`/other → patch); override it explicitly when needed.
- **Zero required config** — sensible built-in defaults (category mapping, template, tag format) mean `draftsman preview` works in any repo with no `.draftsman.yml` at all.
- **Configurable** — category → section mapping, changelog template, tag format, and the skip-changelog trailer key are all overridable per repo.
- **Ships as a GitHub composite Action** — no separate install step needed in a GitHub Actions workflow.

## 🚀 Quick Start

```shell
# Nix
nix run github:brpaz/draftsman -- --help

# Go
go install github.com/brpaz/draftsman/cmd/draftsman@latest

# Docker
docker run --rm -v "$(pwd):/repo" -w /repo ghcr.io/brpaz/draftsman:latest preview
```

Requires `git` on `PATH`; a backend API token is only needed for `draft`/`publish`, not `preview`.

Prebuilt binaries, prerequisites, and every install method in full: **[docs/installation.md](docs/installation.md)**.

## 🔌 Provider setup

GitHub, Gitea, and Forgejo all work behind one common `--backend` flag. Token setup, `--base-url`, and CI examples per provider: **[docs/providers/](docs/providers/)**.

## Usage

### GitHub Action

A composite Action (`action.yml` at the repo root) wraps the CLI so a workflow can `uses:` it directly instead of hand-installing the binary.

**Keep the draft release up to date on every push to the default branch:**

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

**Publish the draft on demand:**

```yaml
name: Publish release

on:
  workflow_dispatch:
    inputs:
      version:
        description: "Override the auto-computed version (optional)"
        required: false

permissions:
  contents: write

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: brpaz/draftsman@v1
        with:
          command: publish
          backend: github
          token: ${{ secrets.GITHUB_TOKEN }}
          version: ${{ inputs.version }}
```

`--repo` needs no input — it's read from the `GITHUB_REPOSITORY` environment variable GitHub Actions already sets on every runner.

For Gitea Actions and Forgejo Actions workflow examples (no composite Action exists for those — the binary/Docker image is used directly), see the provider guides linked above.

### CLI

Three commands, all reading the current directory as the git repo:

| Command | What it does |
| --- | --- |
| `draftsman preview` | Computes and prints release notes to stdout — no backend credentials required. |
| `draftsman draft` | Upserts the draft release(s) on the backend with entries computed since the last release. |
| `draftsman publish` | Promotes the draft release to published and tags it. |

```shell
draftsman preview
draftsman draft --backend github --token "$GITHUB_TOKEN" --repo owner/repo
draftsman publish --backend github --token "$GITHUB_TOKEN" --repo owner/repo --version 1.2.0
```

Full flag reference (including env var equivalents, `--package`, and multi-mode behavior): [docs/cli.md](docs/cli.md).

### Configuration

An optional `.draftsman.yml` at the repo root overrides any built-in default — every field is optional:

```yaml
mode: multi # single (default) | multi

categories:
  - type: feat
    section: Features
  - type: fix
    section: Bug Fixes

packages:
  - path: packages/api
    name: API
  - path: packages/web
    name: Web

skip-changelog-trailer: Skip-Changelog
tag-format: "{{package}}/v{{version}}"
```

Full field reference and the built-in default template: [docs/configuration.md](docs/configuration.md).

## 📚 Documentation

Full documentation, including architecture and design decisions, lives under [`docs/`](docs/) and is published as a static site (built with [Zensical](https://zensical.org/)):

- [Installation](docs/installation.md) — prerequisites and every install method in detail.
- [Configuration](docs/configuration.md) — full `.draftsman.yml` reference.
- [CLI reference](docs/cli.md) — every command, flag, and environment variable.
- [Provider setup](docs/providers/) — GitHub, Gitea, Forgejo: tokens, `--base-url`, CI examples.
- [Git Workflows](docs/workflows.md) — trunk-based, GitHub Flow, Git Flow.
- [Development](docs/development/) — environment setup, [architecture](docs/development/architecture.md), and the [ADRs](docs/adr/) behind PR linkage, the continuous-draft model, commit-based entries, and single vs multi release mode.

Coming from [release-drafter](https://github.com/release-drafter/release-drafter)? See the [migration guide](https://brpaz.github.io/draftsman/migrating/from-release-drafter/), or let an agent do the translation — a [Claude Code skill](skills/migrate-from-release-drafter/) is included:

```shell
npx skills add brpaz/draftsman --skill migrate-from-release-drafter --agent claude-code
```

## 🤝 Contributing

All contributions are welcome. Please check [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## 🫶 Support

If you find this project helpful and would like to support its development, there are a few ways you can contribute:

[![Sponsor me on GitHub](https://img.shields.io/badge/Sponsor-%E2%9D%A4-%23db61a2.svg?&logo=github&logoColor=red&&style=for-the-badge&labelColor=white)](https://github.com/sponsors/brpaz)

<a href="https://www.buymeacoffee.com/Z1Bu6asGV" target="_blank"><img src="https://www.buymeacoffee.com/assets/img/custom_images/orange_img.png" alt="Buy Me A Coffee" style="height: auto !important;width: auto !important;" ></a>

## 👱 Contributors

- [Bruno Paz](https://brunopaz.dev) - Creator and maintainer

## ❤️ Acknowledgements

## 📃 License

Distributed under the MIT License. See [LICENSE](LICENSE) file for details.

## 📩 Contact

- 📧 **Email**: [bruno@brunopaz.dev](mailto:bruno@brunopaz.dev)
- 🐞 **Issues**: [GitHub Issues](https://github.com/brpaz/draftsman/issues)
- 🖇️ **Source**: [https://github.com/brpaz/draftsman](https://github.com/brpaz/draftsman)
