# 05 — GitHub: `CreateRelease` + `publish` under `release-strategy: pr`

**What to build:** Running `draftsman publish` under `release-strategy: pr` against a repo where a release branch has already been merged to the default branch reads the bumped version file and the `CHANGELOG.md` top entry, then creates an already-published GitHub release directly — no pre-existing Draft Release object involved, unlike `release-strategy: draft`'s publish path.

**Blocked by:** 01 (independent of 02/03/04 — this ticket never opens a PR or pushes a branch; it operates on a fixture repo where the merge has already happened).

- [ ] `backend.Backend` gains `CreateRelease(ctx, tag, name, body) error` — creates an already-published release directly, no promote-a-draft step. Implemented for the GitHub adapter first, other adapters get a not-yet-implemented stub
- [ ] `internal/commands/publish`'s `Action`, under `release-strategy: pr`, reads the version back from the package's version file (auto-detected or overridden path, same convention as ticket 01) instead of using `Engine.Plan`'s suggested version
- [ ] Reads the release body verbatim from the `CHANGELOG.md`'s newest entry for that package instead of re-rendering via `Engine.Plan`
- [ ] Computes the tag via the existing `tag-format` template, same as `release-strategy: draft`'s publish path
- [ ] Calls the new `Backend.CreateRelease` instead of `Backend.Publish`
- [ ] `release-strategy: draft`'s existing publish path (`Backend.Publish` promoting an existing draft) is unchanged
- [ ] GitHub adapter's `CreateRelease` is tested via `httptest` fixtures, same pattern as existing adapter coverage
- [ ] `publish` under `release-strategy: pr` is tested against a fixture repo with a pre-existing merged bump commit, via a fake in-memory `Backend`, asserting the correct version/tag/body and the right `Backend` call
- [ ] `go test ./...` passes
