# Provider setup

draftsman talks to GitHub, Gitea, and Forgejo through one common `Backend` interface (`UpsertDraft`, `Publish`, `ResolvePR`) — the `draft`/`publish` CLI commands behave identically regardless of which one you point `--backend` at. What differs per provider is the token you need and whether `--base-url` is required:

| Provider | `--backend` | `--base-url` | PR-reference API fallback |
| --- | --- | --- | --- |
| [GitHub](github.md) | `github` | not needed (fixed `api.github.com`) | yes |
| [Gitea](gitea.md) | `gitea` | required | no endpoint available |
| [Forgejo](forgejo.md) | `forgejo` | required | not attempted (unreliable upstream) |

The PR-reference API fallback column is about *enrichment* only (attaching a linked PR number to a changelog entry) — it has no bearing on core `draft`/`publish` functionality, which works identically on all three. See [ADR-0001](../adr/0001-pr-linkage-strategy.md) for why Gitea and Forgejo don't get an API fallback: commit-message text extraction (squash-merge trailers) still works on both, just without the live-lookup safety net GitHub gets.
