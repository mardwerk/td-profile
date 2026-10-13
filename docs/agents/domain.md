# Domain documentation

This repository has one context. Read [GLOSSARY.md](../../GLOSSARY.md), [RULES.md](../../RULES.md), relevant local contracts and applicable decisions in `docs/adr/` before domain exploration or terminology changes. Missing ADRs do not block exploration. Create them only for durable choices with meaningful alternatives and costly reversal.

Use Accepted terms with their recorded definitions and aliases. Pending and out-of-scope rows are proposals. Resolve new or changed meanings with Kyle before marking them Accepted. Product contracts govern behavior and acceptance.

Before establishing a term, search the accessible repository glossaries for its spelling, aliases and words describing its intended concept. Use a literal, case-insensitive search with explicit glossary paths, including this repository:

```sh
rg -n -i -F -- "$term" /path/to/repo/GLOSSARY.md /path/to/another/repo/GLOSSARY.md
```

Repeat for known aliases and concept descriptions. Exit 0 means a text match, 1 means no text match, and 2 means a search error. Inspect the matching definitions and their status. Text matching does not determine semantic equivalence or approve a pending proposal. An equivalent Accepted definition may be copied locally without a new terminology decision; keep its spelling, definition and accepted aliases unchanged. Ask Kyle when no Accepted equivalent exists, meanings differ, definitions conflict or relevant glossaries are unavailable. A search error is incomplete coverage. Record the search coverage and decision in the owning issue or PR.

Keep accepted terms and definitions complete here. Vocabulary lookup creates no runtime dependency or authority for another repository. Flag contradictions with an accepted contract or ADR before dependent implementation.
