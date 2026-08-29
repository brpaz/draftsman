# Configuration

An optional `.draftsman.yml` at the repo root overrides the built-in defaults. Every field is optional — an absent file, or a file that sets nothing, behaves identically to the built-in defaults below. Point at a different path with `--config` (see [CLI reference](cli.md)).

A full, commented example with every field set is at [`.draftsman.example.yml`](https://github.com/brpaz/draftsman/blob/main/.draftsman.example.yml) in the repo root. A [JSON Schema](schema/config.schema.json) is also published — point your editor at it for validation and autocomplete:

```yaml
# yaml-language-server: $schema=https://brpaz.github.io/draftsman/schema/config.schema.json
```

## `mode`

```yaml
mode: single # default
```

- `single` — one Draft Release, one repo-wide version. `packages`, if set, only sections entries within that single release; it does not split versions or tags.
- `multi` — one Draft Release *per Package*, each with an independent version and tag.

See [ADR-0004](adr/0004-single-vs-multi-release-mode.md) for why this is an explicit choice rather than an implicit "everything is one Package" default. Switching `single → multi` later doesn't cleanly re-partition existing tags into per-package sequences — decide this early.

## `categories`

```yaml
categories:
  - type: feat
    section: Features
  - type: fix
    section: Bug Fixes
  - type: perf
    section: Performance
  - type: docs
    section: Documentation
```

Maps a Conventional Commit `type` to a changelog section name. Order in the list is the display order of sections in the rendered output. Commit types with no matching entry fall into an "Other" section.

Setting `categories` at all is a **full replace**, not a merge with the built-in default — list every type/section pair you want, in the order you want. This is deliberate: a partial merge can't express reordering unambiguously when only some types are overridden.

### Matching by scope too

An optional `scope` field narrows a category to a specific Conventional Commit scope, e.g. `fix(security): ...` or `chore(deps): ...`. A commit is matched against `categories` **in order, first match wins** — so a scope-specific rule only takes precedence over a broader type-only rule for the same type when it's listed *before* it:

```yaml
categories:
  - type: fix
    scope: security
    section: 🔐 Security
  - type: fix
    section: 🐛 Bug Fixes
  - type: chore
    scope: deps
    section: 🧩 Dependency Updates
  - type: chore
    section: ⚠️ Maintenance
```

With this config, `fix(security): patch auth bypass` lands in **Security**; any other `fix(...)` (or `fix:` with no scope) falls through to **Bug Fixes**. A `scope` of `""` (the default when omitted) matches any scope for that type — it's how the plain `type`-only rules above still catch everything the scope-specific rules don't. Listing the type-only `fix` rule *first* would make the `security`-scoped rule unreachable — ordering, not specificity, decides precedence.

**Built-in default:**

```yaml
categories:
  - type: feat
    section: Features
  - type: fix
    section: Bug Fixes
```

## `packages`

```yaml
packages:
  - path: packages/api
    name: API
  - path: packages/web
    name: Web
```

Maps a path prefix to a monorepo Package name. A commit is attributed to every Package whose `path` prefixes one of the commit's changed files — a commit touching files under two or more configured prefixes produces entries in both.

In `single` mode, this only sections entries within the one Draft Release. In `multi` mode, each Package gets its own Draft Release, version, and tag — see [`mode`](#mode) above.

Setting `packages` at all is a **full replace** of the (empty) default, same reasoning as `categories`.

## `skip-changelog-trailer`

```yaml
skip-changelog-trailer: Skip-Changelog # default
```

A commit whose message footer contains this trailer key is excluded from the changelog entirely — useful for `chore:`/internal commits that shouldn't appear in release notes. Example commit footer:

```
fix: correct off-by-one in pagination

Skip-Changelog: true
```

### `[skip changelog]` tag

For a single-line commit, a footer trailer needs a multi-line message. A `[skip changelog]` tag anywhere in the commit message — subject or body — does the same thing, `[skip ci]`-style:

```shell
git commit -m "chore: bump lockfile [skip changelog]"
```

Case-insensitive, and always recognized — unlike `skip-changelog-trailer`, this literal tag isn't configurable.

## `tag-format`

```yaml
tag-format: "v{{version}}" # default
```

A template string used both to locate the previous release tag (range-finding) and to render the tag for a new release. `{{version}}` is replaced with the computed SemVer string.

In `multi` mode, `{{package}}` is also available and typically required to keep each Package's tags distinct:

```yaml
tag-format: "{{package}}/v{{version}}"
```

In `single` mode, `{{package}}` is accepted in the template but always resolves to an empty string — there's only one Package's worth of tags to render.

## `template`

Overrides the built-in Go [`text/template`](https://pkg.go.dev/text/template) used to render the changelog body — see that page for the templating syntax itself (`{{if}}`, `{{range}}`, `{{with}}`, pipelines, etc.); the table below only covers the data draftsman feeds into it. The built-in default (shown below) is Markdown, with an `{{if .SuggestedVersion}}` heading guard that keeps single-package output identical to before Packages existed:

```gotemplate
{{if .SuggestedVersion}}# {{.SuggestedVersion}}

{{end}}{{range .Packages}}{{if .Name}}# {{.Name}}{{if .SuggestedVersion}} ({{.SuggestedVersion}}){{end}}
{{end}}{{range .Sections}}## {{.Name}}
{{range .Entries}}- {{.Description}}{{if .PR}} ({{if .PR.Link}}[#{{.PR.Number}}]({{.PR.Link}}){{else}}#{{.PR.Number}}{{end}}){{end}} by {{if .AuthorRef}}[@{{.AuthorRef.Login}}]({{.AuthorRef.ProfileURL}}){{else}}{{.Author}}{{end}} ({{if .CommitURL}}[{{.ShortSHA}}]({{.CommitURL}}){{else}}{{.ShortSHA}}{{end}})
{{end}}
{{end}}{{end}}
```

### Available placeholders

The root template value is a `Plan`. `{{.Packages}}`, `{{.Sections}}`, and `{{.Entries}}` are slices — range over them to reach the fields below.

| Placeholder | Type | Meaning |
| --- | --- | --- |
| `{{.SuggestedVersion}}` | string | Computed next version (single mode). Empty in multi mode — use each Package's own `.SuggestedVersion` instead. |
| `{{.PreviousVersion}}` | string | The version the range was computed from (single mode). Empty if no prior matching tag was found. |
| `{{.Packages}}` | `[]PackagePlan` | Multi mode only — empty in single mode. |
| `{{.Packages}}[].Name` | string | Package display name, from `packages[].name` in config. |
| `{{.Packages}}[].SuggestedVersion` | string | Computed next version for this Package. |
| `{{.Packages}}[].PreviousVersion` | string | The version this Package's range was computed from. |
| `{{.Packages}}[].Sections` | `[]Section` | This Package's changelog sections. |
| `{{.Sections}}` | `[]Section` | Single mode: top-level sections. Also reachable per-Package via `{{.Packages}}[].Sections` in multi mode. |
| `{{.Sections}}[].Name` | string | Section heading, from `categories[].section` in config (or the built-in "Other" bucket). |
| `{{.Sections}}[].Entries` | `[]Entry` | Changelog entries in this section. |
| `{{.Entries}}[].Description` | string | The commit's Conventional Commit subject line. |
| `{{.Entries}}[].Type` | string | The commit's Conventional Commit type, e.g. `feat`, `fix`. |
| `{{.Entries}}[].Scope` | string | The commit's Conventional Commit scope, if any (empty string otherwise). |
| `{{.Entries}}[].SHA` | string | Full commit SHA. |
| `{{.Entries}}[].ShortSHA` | string | SHA truncated to git's conventional 7-character abbreviation. |
| `{{.Entries}}[].Author` | string | The commit's plain git author name (`git log`'s `%an`) — always populated, independent of `AuthorRef`. |
| `{{.Entries}}[].AuthorRef` | `*AuthorReference` | The author's linked backend account, resolved via a live API call. `nil` when no `Backend` is configured, the backend doesn't support the lookup, or the commit's author has no linked account — guard with `{{if .AuthorRef}}` and fall back to `Author` (see the built-in default template's pattern). Currently only resolved on GitHub. |
| `{{.Entries}}[].AuthorRef.Login` | string | The linked account's username. |
| `{{.Entries}}[].AuthorRef.ProfileURL` | string | The linked account's profile URL. |
| `{{.Entries}}[].PR` | `*PRReference` | `nil` when no PR was resolved — guard with `{{if .PR}}`. |
| `{{.Entries}}[].PR.Number` | int | Resolved PR number. |
| `{{.Entries}}[].PR.Link` | string | Resolved PR URL. Empty when the number came from GitHub's `(#N)` squash-merge title extraction, which carries no URL — guard with `{{if .PR.Link}}` before linking it (see the built-in default template's `{{if .PR.Link}}...{{else}}#{{.PR.Number}}{{end}}` pattern). Populated for Gitea/Forgejo's `Reviewed-on:` trailer extraction and GitHub's API fallback. |
| `{{.Entries}}[].CommitURL` | string | Link to the commit on the backend's web UI. Empty when no `--backend`/`--repo` was configured (e.g. `preview` without those flags) — guard with `{{if .CommitURL}}` (see the built-in default template's pattern). |

## Full example

```yaml
mode: multi

categories:
  - type: feat
    section: Features
  - type: fix
    section: Bug Fixes
  - type: perf
    section: Performance
  - type: docs
    section: Documentation

packages:
  - path: packages/api
    name: API
  - path: packages/web
    name: Web

skip-changelog-trailer: Skip-Changelog
tag-format: "{{package}}/v{{version}}"
```
