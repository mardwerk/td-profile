# Profile Template

Copy this `profile/` directory and the empty `game-data/` directory into a game repository. Keep the schema files together. They resolve locally and do not require the validator repository at runtime.

The manifest declares interface version 5 and Template revision 1.0.0. Set its `id`, `revision` and `game` for your game. Change the JSON settings to match your source fields, model names, paths, requirements, progression limits and score weights. Keep reusable schemas unchanged.

The default discriminator is `kind`, with literal model names. The known types are `tower`, `upgrade`, `purchase`, `attack`, `weapon`, `projectile`, `damage`, `ability`, `enemy` and `map`. Each mechanic maps canonical fields to fields with the same name. `mechanics.json` lists its required fields. Unknown model types fail.

Tower states live in `Towers/<familyId>/<id>.json`. Upgrade definitions live in `Upgrades/<id>.json`. Optional map and enemy records live in `Maps/<id>.json` and `Enemies/<id>.json`. Purchases, attacks, weapons, projectiles, damage and abilities are embedded models. A purchase references its target Tower and Upgrade definition. Applied upgrades and an ability's source upgrade reference Upgrade definitions.

A base Tower starts a family with one upgrade path and tiers 0 through 2. The progression rule requires all three reachable states and their purchases when a base exists. Empty game-data passes because there are no consumers or progression roots. Change the limits and selectors for your game.

The Template validates canonical shapes, required mapped fields, layouts, references, numerical units and configured progression. Canonical schemas allow additional fields. The Template does not declare strict source contracts or claim to check every source field. Add a `modelContracts` document to the manifest to check exact source field shapes. The reusable `schemas/profile/model-contracts.schema.json` defines that document. Every listed source field is required. Coverage distinguishes canonical schema checks from source contract checks.

The score measures declared data compliance. It does not measure gameplay balance or simulate combat. An isolated Tower score follows its family and declared dependencies; unrelated malformed files do not affect it. Validate all game-data separately when checking the whole corpus.

Run from the repository root:

```sh
go run ./cmd/atlas-validator --data game-data --profile profile
go run ./cmd/atlas-validator --data examples/minimal-game/game-data --profile profile
go run ./cmd/atlas-validator score-tower --game-data examples/minimal-game/game-data --profile profile --tower Towers/Bolt/Bolt-0.json
```
