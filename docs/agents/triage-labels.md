# Triage roles

Use the existing repository labels. Skill roles do not introduce another lifecycle. Readiness, work selection and contract acceptance are separate decisions.

| Skill role | Repository treatment |
| --- | --- |
| `needs-triage` | Record unresolved scope or ownership in the issue; preserve existing labels. |
| `needs-info` | Record the missing information and who must supply it; preserve the work state. |
| `ready-for-agent` | Record the accepted outcome, settled responsibilities, interfaces, dependency directions, prerequisites and acceptance evidence. Preserve the work state. |
| `ready-for-human` | Record the required human action and why it needs a person; preserve the work state. |
| `wontfix` | Distinguish explicit rejection, duplication and invalid reports. Preserve retained deferred or excluded proposals; close only when the recorded disposition warrants it. |

Use existing `bug` and `enhancement` categories when appropriate. Use `duplicate`, `invalid` and `wontfix` only when their recorded disposition applies. Retaining a proposal is not a rejection. Record deferral and exclusion in the issue until local lifecycle labels are adopted.

Do not create the upstream default role labels or infer `status:active` from readiness. Read [issue-tracker.md](issue-tracker.md) before listing work and [RULES.md](../../RULES.md) before changing terminology. Preserve the invoked triage skill's AI attribution requirement.

Before assigning or migrating labels, read [local label rules](../ISSUE_LABELS.md).
