# 01 — Walking skeleton: `preview` on a single-package repo

**What to build:** Running `releaser preview` inside a real git repository parses every commit as a Conventional Commit, buckets each into a changelog section (built-in default type→section mapping — `feat`→Features, `fix`→Bug Fixes, everything else unmapped→Other), renders them through a built-in default template, and prints the result to stdout. No `.releaser.yml` required — everything is a hardcoded default at this stage. No backend credentials involved.

**Blocked by:** None — can start immediately.

- [ ] `internal/git` reads the current repo's commit log (author, message, changed file paths, SHA) without shelling assumptions beyond `git` being on `PATH`
- [ ] `internal/commit` parses a commit message into type, optional scope, breaking-change flag (`!` or `BREAKING CHANGE:` footer), subject, and raw footers
- [ ] `internal/engine.Plan` produces one Entry per commit and groups Entries by section using the built-in default category mapping
- [ ] A built-in default template renders the grouped Entries into changelog body text
- [ ] `internal/commands/preview`'s `Action` calls `Plan` and writes the rendered output to `cmd.Writer` instead of returning "not yet implemented"
- [ ] Running `releaser preview` in a repo with a small real commit history (mix of `feat`/`fix`/unmapped types) produces correctly sectioned, human-readable output
- [ ] `Plan()` is tested against real temporary git repo fixtures (real commits, no mocked git), per the confirmed test-seam design
