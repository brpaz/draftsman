# 04 — Human-edit backoff

**What to build:** Re-running `draftsman draft` against an already-open release PR whose branch a human has since hand-edited leaves that branch/PR alone instead of overwriting the edit; a branch whose HEAD is still bot-authored keeps being force-updated as before.

**Blocked by:** 03.

**Status:** done

- [x] Before updating an existing release branch, check the branch's HEAD commit author against draftsman's own bot identity (the one ticket 02 commits as)
- [x] HEAD authored by the bot → proceed with the existing force-update behavior from ticket 03
- [x] HEAD authored by anyone else → skip updating that branch/PR entirely for this run, without erroring the whole `draft` invocation (other packages' branches, if any, still get processed normally)
- [x] Tested against fixture branches with a bot-authored HEAD (update proceeds) and a human-authored HEAD (update skipped), independent of any real backend
- [x] `go test ./...` passes
