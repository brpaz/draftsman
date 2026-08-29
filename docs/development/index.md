# Development

Environment setup for working on draftsman itself. For the full contribution process — reporting issues, PR checklist, review process — see [CONTRIBUTING.md](https://github.com/brpaz/draftsman/blob/main/CONTRIBUTING.md) in the repo root; this page only covers getting a working local environment.

## Environment

The project uses [Devenv](https://devenv.sh/) (Nix-based) for a self-contained, reproducible environment — same Go version, same linters, same doc-site tooling for every contributor.

```shell
git clone https://github.com/brpaz/draftsman.git
cd draftsman
direnv allow
```

[Direnv](https://direnv.net/) auto-loads the Devenv shell on `cd`. Without it, start the shell manually:

```shell
devenv shell
```

Verify you're in it: `which go` should resolve to a `/nix/store/...` path, not a system Go install.

## Tasks

Common development tasks run through [Taskfile](https://taskfile.dev/):

```shell
task <task-name>
task -l # list every available task
```

| Task | Description |
| --- | --- |
| `build` | Build the project |
| `test` | Run unit tests with coverage |
| `lint` | Run GolangCI-Lint |
| `lint-fix` | Run GolangCI-Lint with auto-fix |
| `gomod` | Download Go modules and tidy |
| `gomarkdoc` | Generate Go package documentation |
| `docs-build` | Build this documentation site (Zensical) |
| `docs-serve` | Serve this documentation site locally |

## Code quality tools

- [GolangCI-Lint](https://golangci-lint.run/) — parallel multi-linter runner.
- [Gotestsum](https://github.com/gotestyourself/gotestsum) — readable test output and summaries.
- [Commitlint](https://keisukeyamashita.github.io/commitlint-rs/) — Conventional Commits enforcement.

## Git hooks

[Lefthook](https://lefthook.io/) installs automatically when the Devenv shell starts:

- `pre-commit` — formatting, linters, tests.
- `pre-push` — commitlint, checking commit messages follow Conventional Commits. Only enforced on push, not on every local commit, so local WIP commits aren't blocked mid-flow.

## Architecture and design decisions

- [Architecture](architecture.md) — how the codebase's layers fit together.
- [Architecture Decision Records](../adr/) — the *why* behind the non-obvious choices: PR linkage strategy, the continuous-draft model, commit-based entries, single vs multi release mode.
