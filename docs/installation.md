# Installation

## Prerequisites

- `git` available on `PATH`. draftsman shells out to the local `git` binary to read commit history, changed files, and tags — no network access to your repository is required for `preview`.
- A backend API token, for `draft`/`publish` only. `preview` needs no credentials unless you also want GitHub PR-reference enrichment (see [Provider setup](providers/index.md)).

## Nix

A [flake](https://github.com/brpaz/draftsman/blob/main/flake.nix) is published at the repo root, exposing `packages.default` and `apps.default` for `x86_64-linux`, `aarch64-linux`, `x86_64-darwin`, and `aarch64-darwin`.

Run it directly, without installing anything:

```shell
nix run github:brpaz/draftsman -- --help
```

Or install it into your user profile:

```shell
nix profile install github:brpaz/draftsman
```

Or add it as a flake input to your own project:

```nix
{
  inputs.draftsman.url = "github:brpaz/draftsman";

  outputs = { self, nixpkgs, draftsman, ... }: {
    # e.g. inside a devShell:
    # packages = [ draftsman.packages.${system}.default ];
  };
}
```

## Go

Requires Go 1.25+ (matching the version pinned in [`go.mod`](https://github.com/brpaz/draftsman/blob/main/go.mod)):

```shell
go install github.com/brpaz/draftsman/cmd/draftsman@latest
```

This builds from source and installs to `$GOBIN` (or `$GOPATH/bin`). The resulting binary reports `0.0.0-dev` for its version, since `go install` doesn't set the build-time `ldflags` the release pipeline uses — functionality is unaffected.

## Docker

Images are published to the GitHub Container Registry on every push to `main` (`:latest`) and every tagged release (`:vX.Y.Z`), for `linux/amd64` and `linux/arm64`:

```shell
docker run --rm \
  -v "$(pwd):/repo" -w /repo \
  ghcr.io/brpaz/draftsman:latest preview
```

`draft`/`publish` need the token passed through as an environment variable rather than baked into the image:

```shell
docker run --rm \
  -v "$(pwd):/repo" -w /repo \
  -e DRAFTSMAN_TOKEN="$GITHUB_TOKEN" \
  -e DRAFTSMAN_BACKEND=github \
  -e DRAFTSMAN_REPO=owner/repo \
  ghcr.io/brpaz/draftsman:latest draft
```

The image's `ENTRYPOINT` is the `draftsman` binary itself, so arguments after the image name are passed straight to it — no `draftsman` prefix needed inside the container.

## Prebuilt binaries

Every [release](https://github.com/brpaz/draftsman/releases) publishes a `tar.gz` archive per OS/arch (Linux, macOS, Windows — amd64 and arm64) plus a single `checksums.txt` covering all of them:

```shell
curl -sSfL -o draftsman.tar.gz \
  https://github.com/brpaz/draftsman/releases/latest/download/draftsman_Linux_x86_64.tar.gz
curl -sSfL -o checksums.txt \
  https://github.com/brpaz/draftsman/releases/latest/download/checksums.txt

grep " draftsman.tar.gz\$" checksums.txt | sha256sum -c -
tar -xzf draftsman.tar.gz
./draftsman --version
```

This is also how the [GitHub composite Action](providers/github.md#github-action) installs draftsman under the hood — no separate setup step needed when you're already using the Action.

## From source

```shell
git clone https://github.com/brpaz/draftsman.git
cd draftsman
go build -o draftsman ./cmd/draftsman
```

See [CONTRIBUTING.md](https://github.com/brpaz/draftsman/blob/main/CONTRIBUTING.md) for the full development environment setup (Devenv/Nix-based, with Taskfile tasks for lint/test/build).
