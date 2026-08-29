# CLI reference

All three commands read the current working directory as the git repository — there's no `--repo-path`-style flag; `cd` into the repo (or `-w` into it, in Docker) before running.

## Global flags

| Flag | Env var | Default | Meaning |
| --- | --- | --- | --- |
| `--config` | — | `.draftsman.yml` | Path to the config file. A missing file at the *default* path silently falls back to built-in defaults; an explicitly-passed `--config` path that's missing is an error. |
| `--backend` | `DRAFTSMAN_BACKEND` | — | `github`, `gitea`, or `forgejo`. Required for `draft`/`publish`; optional for `preview` (enables PR enrichment only). |
| `--token` | `DRAFTSMAN_TOKEN` | — | Backend API token. Same required/optional split as `--backend`. |
| `--repo` | `DRAFTSMAN_REPO`, `GITHUB_REPOSITORY` | — | Target repository as `owner/repo`. `GITHUB_REPOSITORY` is GitHub Actions' own auto-injected env var, so no explicit input is needed in a GitHub Actions workflow. |
| `--base-url` | `DRAFTSMAN_BASE_URL` | — | Backend instance API base URL. Ignored by GitHub (fixed `api.github.com`); **required** for `gitea`/`forgejo`. |
| `--package` | — | — | Scope the operation to a single Package (multi mode only). |

## `draftsman preview`

Computes and prints release notes to stdout. Never touches the backend for writes — `--backend`/`--token`/`--repo` are all optional here and, when supplied, are used *only* for best-effort GitHub PR-reference enrichment (see [ADR-0001](adr/0001-pr-linkage-strategy.md)).

```shell
draftsman preview
draftsman preview --backend github --token "$GITHUB_TOKEN" --repo owner/repo
```

## `draftsman draft`

Upserts the draft release(s) on the backend with entries computed since the last release. `--backend`, `--token`, and `--repo` are required.

```shell
draftsman draft --backend github --token "$GITHUB_TOKEN" --repo owner/repo
```

- **Single mode:** one Draft Release is upserted. If there's nothing to release (no commits since the last release warrant one), it prints a message and exits 0 — this is not an error.
- **Multi mode:** one Draft Release is upserted per Package that has pending entries. `--package` scopes this to a single Package; omitted, every pending Package is processed in one invocation. Passing `--package` for a Package with nothing pending is an error (there's nothing to scope to); omitting `--package` when nothing anywhere is pending just prints a message, same as single mode.

## `draftsman publish`

Promotes the draft release to published and tags it. `--backend`, `--token`, and `--repo` are required.

```shell
draftsman publish --backend github --token "$GITHUB_TOKEN" --repo owner/repo
draftsman publish --backend github --token "$GITHUB_TOKEN" --repo owner/repo --version 2.0.0
```

| Flag | Meaning |
| --- | --- |
| `--version` | Override the auto-computed version instead of accepting the suggestion. In multi mode, requires `--package` too (otherwise it's ambiguous which Package's tag the override applies to). |

- **Single mode:** publishes using `--version` if given, otherwise the computed suggestion. Errors if neither is available (nothing computed and no override given).
- **Multi mode, `--package` given:** publishes that one Package, using `--version` if given, otherwise its computed suggestion. Errors if the Package has nothing to publish.
- **Multi mode, `--package` omitted:** publishes every Package that currently has a pending suggested version, in one invocation. If none do, prints a message and exits 0.

## Version computation

The suggested version bump is the highest-severity bump among all Entries in range:

| Commit shape | Bump |
| --- | --- |
| Breaking change (`!` after type/scope, or a `BREAKING CHANGE:` footer) | major |
| `feat` | minor |
| anything else that produces an Entry (`fix`, unmapped types, etc.) | patch |

The range is bounded by the latest tag matching `tag-format` (see [Configuration](configuration.md#tag-format)) reachable from `HEAD`; with no matching tag, the full history is used.

## Exit codes

Non-zero on any error (missing required flags, unresolvable backend, git/API failures). "Nothing to release/publish" is **not** an error — it's a normal, expected outcome printed to stdout with exit code 0, so these commands are safe to run unconditionally on every push without failing a CI job when there's simply nothing new.
