# Profile contract

A Profile combines schemas with settings for one game's content. Its files stay separate from game-data and resolve without network access. The manifest identifies the Profile, revision, checker interface and required local documents.

Schemas define reusable field shapes. Settings define collection paths, source identities, model bindings, required fields, references, progression limits, units and score weights. The checker creates a temporary schema view and preserves the supplied records.

The requested formats are separate files:

- `profile/schemas/game-data/tower.schema.json`
- `profile/schemas/profile/mechanics.schema.json`
- `profile/schemas/profile/mechanic-proposal.schema.json`

The mechanics catalog declares supported model types and their shared schemas. A mechanic proposal remains separate from accepted mechanics. Declaring a model does not supply a runtime implementation of its behavior.

The Profile Template uses native generic fields. Tower files follow `Towers/<familyId>/<id>.json`, with one directory per family and one file per state. Upgrade records live in a separate collection. Progression settings require the starter's one path through tiers 0, 1 and 2. A game can replace these limits and source paths while keeping the schemas.

Empty collections are valid when no present consumer requires them. Once a Tower family exists, its applicable progression and outgoing references must resolve. Deleting a required state or upgrade fails. Removing an optional unreferenced record can pass. There is no inventory of filenames or game-data content hashes.

Shared schemas check their declared fields and permit extra fields. The optional source-contract format declares required raw fields, value shapes and exact nested model types; it can reject undeclared fields and validate dictionary values without listing their keys. The starter does not include a capture-derived source catalog.

`score-tower` requires `--profile`, `--game-data` and `--tower`. It checks the selected family, outgoing dependencies and applicable requirements. Unrelated failures do not enter the score. Profile weights award partial credit. Incomplete model coverage prevents 100, and a Profile can make unknown model types errors.

Reports record the checker build, executable identity, Profile revision and dependency digest. They distinguish structural source checks from canonical schema checks. These counts can overlap. A score measures compliance with declared checks, not gameplay behavior, balance or fitness for a particular runtime.
