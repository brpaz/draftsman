# GitLab

`--backend gitlab` talks to `gitlab.com` by default — pass `--base-url` for a self-hosted instance:

```shell
draftsman draft \
  --backend gitlab \
  --token "$GITLAB_TOKEN" \
  --repo owner/repo
```

```shell
draftsman draft \
  --backend gitlab \
  --base-url https://gitlab.example.com \
  --token "$GITLAB_TOKEN" \
  --repo group/subgroup/project
```

`--repo` accepts a nested group path (anything after the first `/` is passed through as-is), and is required in `owner/repo` form either way — pass the full project path split at the first `/`.

## Draft releases work differently here

GitLab has no draft/hidden release state like GitHub's. draftsman fakes one using GitLab's **Upcoming Release** mechanism: `draft` creates or updates a release with `released_at` set far in the future, which GitLab badges as upcoming; `publish` flips `released_at` to now. Two consequences worth knowing before you rely on this:

- The release is **visible in the releases list the whole time** — just badged "Upcoming Release" — not hidden the way a GitHub draft is.
- The underlying **git tag is created as soon as `draft` first runs**, not deferred to `publish` — GitLab's create-release endpoint requires a `ref` up front to create the tag when it doesn't exist yet.

See [ADR-0005](../adr/0005-gitlab-upcoming-release-as-draft.md) for the full reasoning.

## Token

Settings → Access Tokens (project or group level) → scope **`api`**. A personal access token with `api` scope also works for local runs.

## PR-reference enrichment

GitLab's `ResolvePR` calls the documented `commits/{sha}/merge_requests` endpoint — reliable, so it's used as the live-API fallback whenever a commit's message doesn't contain an extractable reference (e.g. a rebase-merge or direct-push commit). This only runs when `--backend`/`--token`/`--repo` are all supplied. See [ADR-0001](../adr/0001-pr-linkage-strategy.md).

`ResolveAuthor` (linking a commit's author to their account) is not supported on GitLab — its commit API returns only the raw git author name/email, not a linked account, so draftsman falls back to the plain git author name for GitLab.

## GitLab CI

There's no composite Action for GitLab (that's a GitHub Actions concept) — run the Docker image or the binary directly as a job. `CI_PROJECT_PATH` is GitLab CI's own auto-injected `owner/repo`-form variable, so `--repo` needs no explicit input in a GitLab CI pipeline:

```yaml
update-draft-release:
  image: ghcr.io/brpaz/draftsman:latest
  rules:
    - if: $CI_COMMIT_BRANCH == $CI_DEFAULT_BRANCH
  script:
    - draftsman draft --backend gitlab --token "$DRAFTSMAN_TOKEN"
  variables:
    GIT_DEPTH: 0 # full history — draftsman walks commits since the last tag
```

Provision `DRAFTSMAN_TOKEN` as a masked CI/CD variable (Settings → CI/CD → Variables) — a project access token with `api` scope. `$CI_JOB_TOKEN` won't work here: GitLab's Releases API does accept it, but only via a `JOB-TOKEN` header, and draftsman's GitLab adapter only sends `PRIVATE-TOKEN`.
