# Commit-based changelog entries, not PR-based

Release Drafter and similar tools build entries from merged PR metadata (title, labels), which only works for repos that merge every change through a PR. We instead generate one Entry per commit, parsed as Conventional Commits, so the tool works identically for direct-push and rebase-merge workflows and doesn't depend on PR APIs (which differ across GitHub/Gitea/Forgejo) for its primary data path. PR data is optional enrichment, attached when resolvable (see ADR-0001), never required for an Entry to exist.

## Consequences

Changelog quality depends on reasonably linear, well-formed commit history — squash-merge or rebase-merge discipline is assumed. A stray unformatted merge commit or non-Conventional-Commit message produces a dropped or noisy entry by design, rather than a silently guessed one.
