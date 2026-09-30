# Agent instructions

This repository owns the Profile Validator, reusable schemas and a copyable Profile Template. Game repositories own their settings, content and runtime behavior.

Keep schemas reusable across games. Put source fields, model names, paths, requirements, progression limits and score weights in Profile settings. Preserve supplied game-data. Do not add a capture inventory or game-data hashes for completeness.

Keep the command entry point small. Put reusable checking operations in `internal/atlasvalidate/`. Unknown operations and required model types must fail with useful diagnostics. A score measures declared data compliance, not gameplay balance or simulation.

`profile/` is the starter contract. `game-data/` stays empty apart from `.gitkeep`; synthetic examples belong in `examples/`. Required relationships and progression apply when their consumers exist. Every Profile must resolve its schemas and configuration locally.

Run `go test ./...`, `go vet ./...` and the build for code changes. Verify the empty directory and synthetic example, and test a meaningful invalid case when changing validation. Run `git diff --check` before committing.

Keep `README.md` and local contracts usable without private repository access. Use short, literal prose. Work on focused branches and link issues and pull requests. Do not commit compiled binaries or game captures. Preserve source attribution and license notices.

Maintainers with Planning access also follow [Mardwerk's shared instructions](https://github.com/mardwerk/planning/blob/main/AGENTS.md). Public contributors need only this repository's instructions.
