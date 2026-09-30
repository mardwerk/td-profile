# Project setup

Kyle selected the extraction on 2026-09-30 after approving Atlas PR #3. The implementation plan was reviewed against Atlas's separate-project proposal and Mardwerk's repository ownership guidance.

`td-profile` owns the generic checker, reusable schemas and copyable Profile Template. Atlas keeps BTD6 configuration, raw captures and the exporter. Tower Generator and game consumers have separate adoption decisions; creating this repository does not migrate them.

Keep each Profile self-contained with local schemas. Start with pinned schema copies rather than runtime inheritance or submodules. Game-specific source names and limits belong in settings. Required references and progression establish completeness without enumerating captured filenames or hashing game-data.

The starter uses native generic fields and a small explicit progression. Empty data passes; a synthetic example demonstrates meaningful checks. The shared schema layer permits additional fields. Optional exact source contracts can make a game's raw model structure stricter.

Release archives contain the executable beside its matching Profile, an empty game-data directory, source notices and machine-readable release identity. Keep binaries out of Git. Version the bundle, checker and Profile independently, and record the tested pair.

Before public publication, run tests, race checks, vet, native builds and a release archive smoke test outside the checkout. Verify the empty directory, the example, a partial Tower score and a broken dependency. Verify the extracted checker against Atlas's unchanged accepted capture. Record actual results below before publication.

## Verification

Tests, race checks, vet and native builds passed. All 25 reusable schemas match the merged Atlas source byte-for-byte. The command and reported checker name use `validator`. Synthetic test fixtures adapt to the new Template without relying on BTD6 settings.

The empty directory passes with zero files. The synthetic game checks five files, eight references and 21 canonical model instances. Its Tower scores 100/100. Regression tests reject missing upgrades, missing states, wrong values and unknown mechanics; those failures produce partial scores. Unrelated malformed data does not change the selected Tower's report.

The Linux amd64 archive runs outside the checkout, with its Profile, source notices, documentation and example included. Its game-data directory is actually empty. The archive smoke test validates the empty directory and example, checks a score of 100 and rejects a removed Profile dependency. The Profile dependency digest is `981def0d205941af5535501f93b19abfa73254bb1331f4db1f42647975dae5a7`.

Atlas compatibility checks cover 10,053 files and 482,492 model instances, with no unbound instances. Integrity passes. The sole error is the accepted capture's existing duplicate Boomerang purchase. Dart scores 100. The accepted capture and exporter remain unchanged.

Linux runtime checks passed locally. The first release workflow also passed native Linux and macOS packaging, but Windows exposed a copied schema-loader bug. Checker `5.0.1` converts native drive paths to correctly escaped file URLs and back before checking containment. Regression tests preserve rejection of foreign hosts and paths outside the Profile.

Release `v1.0.1` requires all three native packaging jobs to pass before publication. The failed `v1.0.0` tag is retained without downloadable releases. A cross-build alone does not establish that an archive runs on its target.
