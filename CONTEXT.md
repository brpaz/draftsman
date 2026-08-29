# Draftsman

A CLI tool that generates release notes from Conventional Commits, publishing/maintaining them as a live draft release across multiple git backends (GitHub, GitLab, Gitea, Forgejo), monorepo-aware.

## Language

**Entry**:
One changelog line, derived from a single commit parsed as a Conventional Commit. The unit of generation — not the PR, not the release as a whole.
_Avoid_: Change, line item

**Draft Release**:
The live, continuously-updated release notes maintained on the backend (as a draft release object) as commits land on the default branch, finalized when the release is published/tagged. In `single` mode there is exactly one Draft Release per repo; in `multi` mode there is one per Package, each with its own version/tag. GitLab has no draft/hidden release state, so its adapter fakes one via the "Upcoming Release" mechanism (a release with a future `released_at`) — see [ADR-0005](docs/adr/0005-gitlab-upcoming-release-as-draft.md); the underlying git tag is created immediately there, unlike GitHub/Gitea/Forgejo where it's deferred to publish.
_Avoid_: Draft PR, changelog draft

**Release Mode**:
A repo-wide config choice between `single` (one Draft Release, repo-wide version; Packages only section entries within it) and `multi` (one Draft Release per Package, independent versions and tags).
_Avoid_: Versioning mode (that's a consequence of this choice, not the choice itself)

**Backend**:
A git hosting platform this tool integrates with to read repository data and publish releases: GitHub, GitLab, Gitea, or Forgejo. Each backend is implemented as an adapter behind a common interface.
_Avoid_: Provider, platform (when referring to the adapter target specifically)

**PR Reference**:
The pull/merge request number associated with a commit, resolved either by extracting it from the commit message text (squash-merge trailers) or, on GitHub only, via a live API lookup. Used to enrich an Entry, never required for one to exist.
_Avoid_: PR link, PR number (when referring to the resolution process rather than the raw number)

**Package**:
A monorepo subdivision with its own changelog, determined by mapping a commit's changed file paths to a configured path-prefix in `draftsman`'s config. Derived purely from `git diff`, independent of any backend API.
_Avoid_: Module, workspace, project (when referring to the changelog-scoping unit specifically)
