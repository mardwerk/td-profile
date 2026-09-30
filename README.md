# td-profile

A Profile Validator and reusable schemas for tower-defense game data. Each game supplies a Profile directory that describes its collections, supported models, references and rules. The validator checks separate game-data without rewriting it.

`profile/` is a copyable Profile Template. `game-data/` starts empty and passes because its requirements apply when records exist. This establishes an empty data contract, not a playable game. A small synthetic game in `examples/minimal-game/` demonstrates Tower states and purchases.

## Use

Download a native archive from [Releases](https://github.com/mardwerk/td-profile/releases) and extract it. Run from the extracted directory:

```sh
./atlas-validator --profile profile --game-data game-data --format text
./atlas-validator --profile profile --game-data examples/minimal-game/game-data --format text
./atlas-validator score-tower --profile profile --game-data examples/minimal-game/game-data --tower Towers/Bolt/Bolt-0.json --format text
```

Windows uses `atlas-validator.exe`. To build from a source checkout, use Go 1.23 or later:

```sh
go build -o bin/atlas-validator ./cmd/atlas-validator
bin/atlas-validator --profile profile --game-data game-data --format text
bin/atlas-validator --profile profile --game-data examples/minimal-game/game-data --format text
bin/atlas-validator score-tower --profile profile --game-data examples/minimal-game/game-data --tower Towers/Bolt/Bolt-0.json --format text
```

The empty directory should report zero files. The example should pass and its Tower should score 100/100. The score measures the declared checks, not gameplay balance or simulation. Missing required states, upgrades or references fail. Unrelated files do not affect the selected Tower's score.

## Structure

| Path | Responsibility |
| --- | --- |
| `profile/` | Copyable settings and self-contained schemas. |
| `profile/schemas/` | Reusable Tower, mechanics, mechanic-proposal and supporting schemas. |
| `game-data/` | Empty starting directory. |
| `examples/minimal-game/` | Synthetic records separate from the Profile. |
| `internal/atlasvalidate/` | Generic checking operations and tests. |
| `cmd/atlas-validator/` | Small command entry point. |
| `scripts/` | Release packaging. |

The starter settings use native `kind`, `id` and `familyId` fields and one upgrade path through tiers 0, 1 and 2. These are editable starting choices. A different game can change source bindings, paths, units, limits and score weights without changing the schemas. A new unsupported rule operation still requires checker code.

Shared schemas check declared fields and permit additional fields. Unknown model types fail. Games that need strict raw field shapes can enable source contracts. A passing score reports the coverage the selected Profile declares.

See [the Profile contract](docs/PROFILE.md), [release packaging](docs/RELEASE.md), [setup evidence](docs/SETUP.md) and [agent instructions](AGENTS.md). Product contracts and contributor instructions work without private Planning access.

The checker and schemas were extracted from [btd6-atlas](https://github.com/KyleDerZweite/btd6-atlas). Atlas retains its BTD6 Profile, exporter and captures. This repository includes no BTD6 captures or derived model catalog. See [the source notice](NOTICE.md) and [MIT license](LICENSE).
