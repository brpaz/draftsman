# 03 — Package resolution + cross-package duplication

**What to build:** A `packages` config section (path-prefix → package name) sections `preview` output per Package — each Package's Entries shown separately, derived purely from `git diff --name-only` per commit, no backend API involved. A commit whose changed files span 2+ configured path-prefixes duplicates its Entry into each affected Package's section, verbatim.

**Blocked by:** 02

- [ ] `packages` config field: ordered list of path-prefix → package-name mappings
- [ ] Package resolution reads each commit's changed file paths and maps them to the configured prefixes — a commit with no changed file under any configured prefix is unmapped (repo-root/implicit-package handling is out of scope for this ticket; single-package repos with no `packages` config keep ticket-01/02 behavior)
- [ ] A commit touching files under two or more configured prefixes produces one Entry per affected Package (duplicated content, not split or merged)
- [ ] `preview` output groups by Package first, then by category within each Package
- [ ] `Plan()` tests cover: single-package-prefix repo, multi-package repo with non-overlapping commits, and a cross-cutting commit touching 2+ packages — asserted via real git fixtures with realistic monorepo path layouts
