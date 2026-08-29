# GitHub

`--backend github` talks to `api.github.com` — `--base-url` is never needed.

## Token

**In a GitHub Actions workflow**, the default `secrets.GITHUB_TOKEN` is sufficient — grant the job `permissions: contents: write` and pass it straight through:

```yaml
permissions:
  contents: write

steps:
  - uses: brpaz/draftsman@v1
    with:
      command: draft
      backend: github
      token: ${{ secrets.GITHUB_TOKEN }}
```

**Outside Actions** (local runs, other CI systems), use a Personal Access Token:

- Fine-grained PAT: **Contents** → Read and write (draft/publish), plus **Pull requests** → Read if you want PR-reference enrichment on `preview`.
- Classic PAT: `repo` scope.

## PR-reference enrichment

GitHub is the only backend with a live API fallback: when a commit's message doesn't contain an extractable PR reference (e.g. a rebase-merge or direct-push commit), draftsman calls `commits/{sha}/pulls` to resolve it. This only runs when `--backend`/`--token`/`--repo` are all supplied — `preview` works fine without them, just without this enrichment step. See [ADR-0001](../adr/0001-pr-linkage-strategy.md).

## GitHub Action

The composite Action at [`action.yml`](https://github.com/brpaz/draftsman/blob/main/action.yml) wraps the CLI — it downloads the matching prebuilt binary for the runner's OS/arch from the Action's own tagged release, verifies it against `checksums.txt`, and runs it. No separate install step needed.

| Input | Required | Notes |
| --- | --- | --- |
| `command` | yes | `draft`, `preview`, or `publish`. |
| `backend` | for draft/publish | `github`, `gitea`, or `forgejo`. |
| `token` | for draft/publish | Backend API token. |
| `config-path` | no | Defaults to draftsman's own resolution (`.draftsman.yml`, or built-ins). |
| `package` | no | Multi mode only. |
| `version` | no | `publish` only — overrides the computed suggestion. |

`--repo` needs no input at all — the Action relies on `GITHUB_REPOSITORY`, which every GitHub Actions runner sets automatically.

**Continuous draft on every push:**

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

**Publish on demand:**

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

## Without the Action (Docker)

```yaml
jobs:
  draft:
    runs-on: ubuntu-latest
    container: ghcr.io/brpaz/draftsman:latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - run: draftsman draft --backend github --token "$DRAFTSMAN_TOKEN" --repo "$GITHUB_REPOSITORY"
        env:
          DRAFTSMAN_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```
