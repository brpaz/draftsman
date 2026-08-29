# Two release modes — single vs multi, config-selected

Monorepos need independent per-package versions and tags (`api-v1.0.0`, `shared-v0.3.1`); single-package repos need one repo-wide version. We considered making every repo implicitly multi-package (a non-monorepo just has one Package, prefix `.`) to avoid a mode switch, but that forces monorepo concepts (per-package tag patterns, N draft releases) onto the common single-repo case where they add nothing. Instead `.draftsman.yml` declares `mode: single | multi` explicitly: `single` keeps one Draft Release with a repo-wide version and uses Packages only to section entries within it; `multi` maintains one Draft Release per Package with independent versions and tags.

## Consequences

Switching a repo from `single` to `multi` later re-partitions version history per package — existing single tags don't map cleanly onto per-package tag sequences. This is a decision best made early.
