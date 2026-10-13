# Issue tracker

Use GitHub Issues in [mardwerk/td-profile](https://github.com/mardwerk/td-profile/issues). Use `gh` with explicit `--repo mardwerk/td-profile`. Read the full body, comments, labels, milestone and relevant local contracts before changing an issue. Preserve recorded owner decisions. Vocabulary proposals and excluded product proposals stay in their owning repository.

For an ordinary request for open issues, use:

```sh
gh issue list --repo mardwerk/td-profile --state open --search '-label:status:out-of-scope' --limit 1000
```

Remove the exclusion when explicitly asked for all open issues, out-of-scope proposals or parked proposals. Preserve other open states unless the request narrows them. For complete inventories, paginate the API rather than assuming one bounded list is exhaustive.

Use [triage-labels.md](triage-labels.md) for skill roles and [RULES.md](../../RULES.md) for local rules. A work label does not accept a contract or select implementation. Use temporary body files with `--body-file` for multiline issues, comments and PRs.

PRs as a request surface: no.

Publishing to the tracker means creating or updating the owning GitHub issue. Product contracts and implementation remain in the owning repository. Do not create local tracker directories or a separate out-of-scope knowledge base.

Before assigning or migrating labels, read [local label rules](../ISSUE_LABELS.md).
