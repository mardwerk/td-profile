# Issue labels

This repository owns its label rules. Keep the installed GitHub labels and their meanings until an explicit local migration is agreed.

Use `bug`, `enhancement` and `documentation` for the corresponding change types. Multiple applicable topics, such as `accessibility`, may accompany a type. Use `duplicate`, `invalid` or `wontfix` only when the recorded disposition supports it. `help wanted` and `good first issue` describe contribution opportunities, not acceptance or selection. Read the installed label descriptions for other local labels.

The intended state, review and priority model below does not install missing labels. Until local lifecycle labels are adopted, record those decisions in the issue body. Do not infer a state from a legacy label alone.

## Label rules

State, topic, type, review, origin and importance describe separate questions. Applying a label does not accept a contract or select work for the owner.

- Use one recorded state per open issue. Normal completion uses native closure without a terminal label. Remove the open state on closure; add at most one terminal reason when the recorded disposition supports it.
- Use topics for the responsibilities involved. Multiple topics are allowed when they apply to the same task. Split independent tasks into separate issues.
- Type is optional, with at most one primary change type.
- Review, Origin and Importance are optional. Only applied labels need a recorded reason. An absent importance label means unassessed.
- Importance is priority, independent of state. High means high priority, medium means ordinary priority, and low means lower priority. Set it from an owner decision or owner-requested assessment. Record blocking dependencies separately in the issue.
- Use `review:decision` for a named pending owner decision, `review:audit` for a fresh audit and `review:closure` for a proposed closure. Multiple review labels require distinct pending tasks. A closure proposal does not close, reject or change the issue's state.
- Use origins only for explicit source or product relationships. They do not create implementation dependencies.
- A milestone records a repository outcome and observable completion criteria. GitHub permits at most one milestone per issue. Assignment does not select work or accept a contract; use linked dependencies for other outcomes.
- Retained excluded proposals stay in their owning repository. Review may apply to them, but ordinary open-issue and owner-review lists exclude `status:out-of-scope`.

## Changes and migration

This repository owns these instructions and its GitHub label definitions. Keep them complete locally. Another repository's private standard is not required reading.

Preserve recorded decisions when migrating labels. Rename or map a label only when its existing meaning and issue evidence support the replacement. Leave ambiguous assignments unchanged and ask Kyle. A label named `wontfix` alone does not prove owner rejection; inactivity does not prove a pause. Cleanup cannot itself accept, reject, close, pause or resume work. Remove a legacy label only after all uses have been resolved.

Use six hexadecimal digits for colors. Reuse a color when a label has the same name and meaning as an existing label in another repository. Different meanings may share colors. Local additions require an explained recurring need and owner agreement; a color has no acceptance meaning.

Before changing an issue, check its owner decisions, current state, applicable topics, optional type, reasons for any applied review, origin or importance labels, and milestone relevance. Missing optional labels are not a checklist failure.
