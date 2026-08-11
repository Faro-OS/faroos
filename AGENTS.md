# FaroOS workspace instructions

- Preserve unrelated user changes in this worktree.
- Use `apply_patch` for source edits and run the relevant Go and Svelte checks before handoff.
- After a successful deployable code change, run `./packaging/auto-stage-if-enabled.sh`. It is a no-op on normal installations; on a maintainer machine with the local update channel enabled, it stages a verified build that the system updater applies automatically without `sudo`.
- Do not publish a GitHub release or create a version tag unless the user explicitly requests it. Stable installations update only from published releases.
