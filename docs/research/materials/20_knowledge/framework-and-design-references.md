# Profile validation research: framework and design references

These observations come from the September 26, 2026 source review. Metadata-only checks did not establish paper findings; reproduced software examples establish only the stated version and scenario. No present-day check or acceptance is implied.

These are dated source observations and proposals. Preserve their recorded review and rejection status. Current local contracts govern acceptance; these notes neither select implementation nor resume paused development.

### Verified findings

- **Confirmed**: The [JSON Schema reference](https://json-schema.org/understanding-json-schema/reference) documents type-specific keywords, `required` properties (object page), boolean composition with `allOf`, `anyOf`, `oneOf` and `not` (on the [combining](https://json-schema.org/understanding-json-schema/reference/combining) subpage), and `if`/`then`/`else` [conditionals](https://json-schema.org/understanding-json-schema/reference/conditionals).
- **Confirmed**: Zod 4 exports `z.toJSONSchema`, and some types have no JSON Schema form.
  Running zod 4.6.5 (npm `latest`) produced a draft 2020-12 object schema with `required`, and threw by default for transforms, `Date`, `BigInt` and `Map` ("Transforms cannot be represented in JSON Schema").
  With `unrepresentable: "any"` it emits `{}` instead.
  The cited zod.dev page itself was not read.

### Proposals and guidance (not evidence)

- **Proposal**: The "Boundary for this project" column in FS gives sound usage guidance but is not a source finding.
  It says to keep semantic checks outside structural validation, keep local validation authoritative, test provider schema compatibility, and treat CEL as a formula candidate rather than a dependency.

### Sources checked

| Source | Cited in | Check (2026-09-26) | Status | Note |
| --- | --- | --- | --- | --- |
| JSON Schema reference | FS, GR | Direct fetch of 6 pages | Confirmed | Types, `required`, composition, conditionals all present |
| Zod JSON Schema guide (zod.dev) | FS, GR | npm tarball zod 4.6.5, ran code | Confirmed | Claim confirmed from the package; the page itself is blocked |
| "Project already uses Zod 4" | FS | unit-generator history | Contradicted | Used zod 4.6.5 at the inspected revision; removed by the inspected revision (2026-09-26) |
| CEL overview (cel.dev) | FS, GR | cel-spec clone, cel-go README | Confirmed | cel.dev blocked; claim confirmed in the spec and cel-go docs |

Source identifiers: FS means Framework source notes; GR means Generator references; GDR means the game-design research collection; KR means the source collection index. These names identify cited inputs, not current product contracts.
