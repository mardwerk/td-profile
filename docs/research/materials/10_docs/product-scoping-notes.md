# Profile validation research: product scoping

These are dated source observations and proposals. Preserve their recorded review and rejection status. Current local contracts govern acceptance; these notes neither select implementation nor resume paused development.

## Source observations

`td-profile` is a selected public product with an implemented generic Profile Validator, reusable schemas and copyable Profile Template.
Its [README](../../../../README.md) and [Profile contract](../../../PROFILE.md) own its current behavior.


`td-profile` implements the generic Profile Validator.

## Product map

| Product | Owns | Typical interfaces |
| --- | --- | --- |
| [td-profile](../../../../README.md) | Generic Profile Validator, reusable schemas, copyable Profile Template and tested binary/Profile bundles | `validator` CLI and self-contained archives |

## Ownership boundaries

The implemented `td-profile` [Profile contract](../../../PROFILE.md) and [release packaging](../../../RELEASE.md) govern its reusable checks and bundles.
Its [setup evidence](../../../SETUP.md) records verification results. Each release platform must pass its native packaging checks before publication.
Atlas keeps its BTD6 settings, exporter and captures. It now [uses the released td-profile validator](https://github.com/KyleDerZweite/btd6-atlas/blob/main/docs/profile-project.md) and pinned local schema copies; its local checker has been removed.

## Possible extractions

Kyle selected `td-profile` on 2026-09-30 after the [Atlas extraction](https://github.com/KyleDerZweite/btd6-atlas/pull/3).

## Current order

As of 2026-10-11, `td-profile` is implemented as a separate public product.

## Where each kind of document belongs

| Kind of document | Home |
| --- | --- |
| Working research notes: paper notes, candidate drafts, canon research | Planning `30_projects/<workstream>/` |

## Profile targets

Its copyable Profile Template is separate from Tower Generator's former BTD6-based Default Profile.
Atlas retains the exporter, captures and BTD6 settings. It now [uses the released td-profile validator](https://github.com/KyleDerZweite/btd6-atlas/blob/main/docs/profile-project.md) and pinned local schema copies; its local checker has been removed.
Tower Generator's former importer and Default Profile work in #85 is retired history. A new Profile remains later work under the current Tower Generator product scope.
