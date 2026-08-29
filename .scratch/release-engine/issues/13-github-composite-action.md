# 13 — GitHub composite Action

**What to build:** A reusable GitHub composite Action (`action.yml`) that downloads the pinned Goreleaser-built binary for the runner's OS/arch and exposes inputs mapping to CLI flags — `command` (`draft`/`preview`/`publish`), `config-path`, `token`, `backend`, `package`, `version` — so a workflow author can drop in one `uses:` step instead of hand-writing install/invoke steps. Marketplace-publishable (public repo, unique name, Developer Agreement, explicit `shell:` on every run step — confirmed requirements).

**Blocked by:** 07, 09

- [x] `action.yml` at repo root, composite (not Docker), with the inputs listed above mapped to the corresponding CLI flags
- [x] Binary download step pins to the release matching the Action's own tagged version (not "latest", via `github.action_repository`/`github.action_ref`), verified checksum (`checksums.txt` asset, `sha256sum -c`) — also fixed `.goreleaser.yaml`'s stale `main: cmd/main.go` (real path is `cmd/releaser/main.go`), added `project_name: releaser` and a fixed `checksum.name_template`, and dropped the Windows zip override in favor of `tar.gz` everywhere so the Action needs only one extraction path
- [ ] `command: draft`, `command: preview`, and `command: publish` all invoke correctly through the Action in a real workflow run — **not verified**: this sandbox has no pushed repo/release to run a live Actions workflow against; needs a real run after the first tagged release exists
- [x] README/usage example added showing the Action wired into a push-to-default-branch workflow (draft) and a manual/dispatch workflow (publish)
- [x] Every `run:` step declares an explicit `shell:` per Marketplace composite-action requirements (both steps use `shell: bash`; verified with `shellcheck`, no warnings)
