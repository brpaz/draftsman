# Gitea

`--backend gitea` requires `--base-url` — Gitea is always self-hosted, so there's no fixed default the way there is for GitHub.

```shell
draftsman draft \
  --backend gitea \
  --base-url https://gitea.example.com \
  --token "$GITEA_TOKEN" \
  --repo owner/repo
```

Pass the instance root URL — no trailing slash or `/api/v1` suffix needed, draftsman adds it.

## Token

Settings → Applications → **Generate New Token**, scoped to `write:repository` (Gitea 1.20+ granular token scopes) — this covers both reading release state and creating/updating draft releases. On older Gitea versions without granular scopes, a token with repository write access is required.

## PR-reference enrichment

Gitea's API has no endpoint equivalent to GitHub's `commits/{sha}/pulls`, so draftsman doesn't attempt one — `ResolvePR` returns "not supported" rather than guessing. PR references still get attached when extractable from commit-message text: Gitea's squash-merge appends a `Reviewed-on: .../pulls/N` trailer, which draftsman's text extraction recognizes. See [ADR-0001](../adr/0001-pr-linkage-strategy.md).

## Gitea Actions

There's no composite Action for Gitea (Gitea Actions doesn't share GitHub's Marketplace) — run the Docker image or the binary directly as a step. `secrets.GITHUB_TOKEN` isn't a thing on Gitea; provision your own token as a repo/org secret.

```yaml
name: Update draft release

on:
  push:
    branches: [main]

jobs:
  draft:
    runs-on: ubuntu-latest # requires a registered Gitea Actions runner
    container: ghcr.io/brpaz/draftsman:latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - run: |
          draftsman draft \
            --backend gitea \
            --base-url ${{ vars.GITEA_URL }} \
            --token "$DRAFTSMAN_TOKEN" \
            --repo ${{ github.repository }}
        env:
          DRAFTSMAN_TOKEN: ${{ secrets.DRAFTSMAN_TOKEN }}
```

`${{ github.repository }}` works the same way on Gitea Actions as it does on GitHub Actions — the runner sets it in `owner/repo` form.
