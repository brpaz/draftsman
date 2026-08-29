# Forgejo

`--backend forgejo` requires `--base-url` — Forgejo is always self-hosted (e.g. a private instance, or [codeberg.org](https://codeberg.org)).

```shell
draftsman draft \
  --backend forgejo \
  --base-url https://codeberg.org \
  --token "$FORGEJO_TOKEN" \
  --repo owner/repo
```

Pass the instance root URL — no trailing slash or `/api/v1` suffix needed, draftsman adds it. The adapter was verified directly against a live instance (codeberg.org), not just assumed identical to Gitea's API.

## Token

Settings → Applications → **Generate New Token**, scoped to `write:repository`. Same granular-scope model as Gitea, since Forgejo is Gitea-derived.

## PR-reference enrichment

Forgejo's `ResolvePR` deliberately returns "not supported" — the equivalent API endpoint has been reported to return the wrong PR in some cases, and draftsman treats an incorrect link as worse than a missing one. PR references still get attached when extractable from commit-message text: Forgejo's squash-merge appends a `Reviewed-on: .../pulls/N` trailer, same as Gitea. See [ADR-0001](../adr/0001-pr-linkage-strategy.md).

## Forgejo Actions

There's no composite Action for Forgejo — run the Docker image or the binary directly as a step. Provision your own token as a repo/org secret (there's no `secrets.GITHUB_TOKEN` equivalent).

```yaml
name: Update draft release

on:
  push:
    branches: [main]

jobs:
  draft:
    runs-on: docker # requires a registered Forgejo Actions runner
    container: ghcr.io/brpaz/draftsman:latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - run: |
          draftsman draft \
            --backend forgejo \
            --base-url ${{ vars.FORGEJO_URL }} \
            --token "$DRAFTSMAN_TOKEN" \
            --repo ${{ github.repository }}
        env:
          DRAFTSMAN_TOKEN: ${{ secrets.DRAFTSMAN_TOKEN }}
```

`${{ github.repository }}` works the same way on Forgejo Actions as it does on GitHub Actions — the runner sets it in `owner/repo` form.
