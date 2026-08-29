# GitLab: faking a draft via Upcoming Release, not a real draft state

GitLab's Releases API has no draft/hidden release concept like GitHub's — every release that exists is visible and listed. The closest mechanism is "Upcoming Release": a release whose `released_at` is set to a future date is flagged `upcoming_release: true` and badged as upcoming in the UI, with no separate `publish` action to promote it. We use this to approximate draftsman's draft→publish model: `UpsertDraft` creates or updates a release with `released_at` far in the future (`9999-01-01T00:00:00Z`); `Publish` PATCHes `released_at` to the current time, clearing the flag. `UpsertDraft` refuses to modify a release whose `upcoming_release` is already `false`, same guard the other adapters use for an already-published release.

This is an approximation, not an equivalent, in two ways: the release is listed (not hidden) the whole time, just badged; and GitLab's create-release endpoint requires `ref` up front to create the tag when it doesn't exist yet, so the underlying git tag is created as soon as the "draft" exists — GitHub/Gitea/Forgejo all defer tag creation to `Publish`, since their drafts have no tag at all until then.

## Consequences

A GitLab "draft" is visible in the releases list from the moment `draft` first runs, tagged immediately, and only its badge changes at `publish` time. This is a meaningfully different visibility story than GitHub/Gitea/Forgejo and should be called out to users choosing `--backend gitlab`, not silently glossed over as identical behavior.
