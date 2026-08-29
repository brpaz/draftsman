# Continuous draft releases, not stateless on-demand generation

Tools like `git-cliff` are stateless: invoke against a commit range, print notes, done. We instead mirror Release Drafter's model — a live draft release is created/updated on the backend on every push to the default branch, finalized by a human (or `draftsman publish`) later. This was chosen because GitHub, Gitea, and Forgejo release APIs all support a `draft` flag, so the "always-current draft, human publishes" workflow is portable across all three backends without any backend-specific compromise.

## Consequences

Generation and publishing are separate concerns — `draftsman draft` upserts the draft, publishing is a distinct step (backend UI or `draftsman publish`). A purely stateless tool would not need this split, but would also lose the "draft is always current" property that was the actual goal.
