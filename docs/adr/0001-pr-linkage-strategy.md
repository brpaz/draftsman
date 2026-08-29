# PR linkage via commit-message text, not backend commit→PR APIs

Entries are generated per-commit (Conventional Commits), not per-PR, so a commit's originating PR must be resolved separately for enrichment (title override, labels, author). GitHub's `commits/{sha}/pulls` API is reliable, but Gitea has no equivalent endpoint and Forgejo's is reported to return the wrong PR in some cases. We instead extract the PR number from commit message text where the merge strategy already embeds it — GitHub squash-merge appends `(#N)`, Gitea/Forgejo squash-merge appends a `Reviewed-on: .../pulls/N` trailer — and only fall back to a live API lookup on GitHub, where it's known reliable. Gitea gets no fallback (no endpoint); Forgejo gets no fallback (unreliable) — commits with no extractable text reference and no reliable API simply ship without a PR link rather than risk attaching the wrong one.

## Consequences

Rebase-merge and direct-push commits, and any Gitea/Forgejo commit without a resolvable text reference, will have no PR link in the changelog. This is accepted: a missing link is preferable to an incorrect one.
