# Project setup

Kyle selected the extraction on 2026-09-30 after approving Atlas PR #3. The implementation plan was reviewed against Atlas's separate-project proposal and Mardwerk's repository ownership guidance.

`td-profile` owns the generic checker, reusable schemas and copyable Profile Template. Atlas keeps BTD6 configuration, raw captures and the exporter. Tower Generator and game consumers have separate adoption decisions; creating this repository does not migrate them.

Keep each Profile self-contained with local schemas. Start with pinned schema copies rather than runtime inheritance or submodules. Game-specific source names and limits belong in settings. Required references and progression establish completeness without enumerating captured filenames or hashing game-data.

The starter uses native generic fields and a small explicit progression. Empty data passes; a synthetic example demonstrates meaningful checks. The shared schema layer permits additional fields. Optional exact source contracts can make a game's raw model structure stricter.

Release archives contain the executable beside its matching Profile, an empty game-data directory, source notices and machine-readable release identity. Keep binaries out of Git. Version the bundle, checker and Profile independently, and record the tested pair.

Before public publication, run tests, race checks, vet, native builds and a release archive smoke test outside the checkout. Verify the empty directory, the example, a partial Tower score and a broken dependency. Verify the extracted checker against Atlas's unchanged accepted capture. Record actual results below before publication.
