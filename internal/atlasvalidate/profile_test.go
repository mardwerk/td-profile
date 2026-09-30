package atlasvalidate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInvalidProfileDocuments(t *testing.T) {
	cases := []struct {
		name, file, want string
		edit             func(map[string]any)
	}{
		{"unknown manifest property", "manifest.json", "", func(p map[string]any) { p["typo"] = true }},
		{"unsupported profile version", "manifest.json", "", func(p map[string]any) { p["formatVersion"] = 900 }},
		{"missing required document", "manifest.json", "", func(p map[string]any) { delete(p["documents"].(map[string]any), "mechanics") }},
		{"unknown collections property", "collections.json", "", func(p map[string]any) { p["typo"] = true }},
		{"duplicate collection", "collections.json", "duplicate collection", func(p map[string]any) {
			p["collections"] = append(p["collections"].([]any), p["collections"].([]any)[0])
		}},
		{"unknown identity selector", "collections.json", "unknown identity selector", func(p map[string]any) { p["collections"].([]any)[0].(map[string]any)["idField"] = "@unknown" }},
		{"missing collection schema", "collections.json", "missing.schema.json", func(p map[string]any) { p["collections"].([]any)[0].(map[string]any)["schema"] = "missing.schema.json" }},
		{"missing schema definition", "collections.json", "", func(p map[string]any) {
			p["collections"].([]any)[0].(map[string]any)["schema"] = "schemas/game-data/model.schema.json#/$defs/missing"
		}},
		{"invalid scope collection", "collections.json", "unknown collection", func(p map[string]any) {
			p["scopes"] = []any{map[string]any{"name": "ordinary", "collection": "missing", "rootSchema": "schemas/game-data/model.schema.json", "memberSchema": "schemas/game-data/model.schema.json", "edgesField": "upgrades", "targetField": "tower"}}
		}},
		{"unknown reference collection", "references.json", "unknown reference target", func(p map[string]any) { p["references"].([]any)[0].(map[string]any)["target"] = "missing" }},
		{"unexplained external symbol", "references.json", "", func(p map[string]any) {
			p["references"].([]any)[0].(map[string]any)["externalSymbols"] = map[string]any{"runtime-only": ""}
		}},
		{"duplicate mechanic identity", "mechanics.json", "duplicate mechanic", func(p map[string]any) { p["mechanics"] = append(p["mechanics"].([]any), p["mechanics"].([]any)[0]) }},
		{"duplicate mechanic model binding", "mechanics.json", "more than one mechanic binding", func(p map[string]any) { p["mechanics"].([]any)[1].(map[string]any)["models"] = []string{"ThingModel"} }},
		{"unknown mechanic policy", "mechanics.json", "", func(p map[string]any) { p["unknownModels"] = "ignore" }},
		{"unknown rule operation", "rules.json", "", func(p map[string]any) {
			p["rules"] = []any{map[string]any{"id": "mystery", "operation": "unknown", "scope": "ordinary", "tiersField": "levels", "familyField": "kind", "requireCompleteStates": true, "limits": map[string]any{"pathCount": 1, "maxTier": 1, "maxPurchasedPaths": 1, "secondaryTierLimit": 1, "maxPathsAboveSecondaryTier": 0}}}
		}},
		{"unknown unit", "numerical-units.json", "unknown numerical unit", func(p map[string]any) { p["bindings"].([]any)[0].(map[string]any)["unit"] = "missing" }},
		{"duplicate unit", "numerical-units.json", "duplicate unit", func(p map[string]any) { p["units"] = append(p["units"].([]any), p["units"].([]any)[0]) }},
		{"duplicate unit binding", "numerical-units.json", "duplicate unit binding", func(p map[string]any) { p["bindings"] = append(p["bindings"].([]any), p["bindings"].([]any)[0]) }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			data, profileDir := fixture(t)
			editDocument(t, profileDir, test.file, test.edit)
			r, status := Validate(data, profileDir, false)
			if status != 2 || r.Valid || !hasError(r, "profile", "", "") || r.FilesChecked != 0 {
				t.Fatalf("invalid Profile must fail before inspecting data: %+v, status=%d", r, status)
			}
			if test.want != "" && !strings.Contains(r.Errors[0].Message, test.want) {
				t.Fatalf("wanted %q, got %+v", test.want, r.Errors)
			}
		})
	}
}

func TestSplitSchemaResolution(t *testing.T) {
	data, profileDir := fixture(t)
	// Collection schemas may resolve a fragment through another local file.
	writeFixture(t, profileDir, "fixture/entry.schema.json", map[string]any{"$ref": "defs.schema.json#/$defs/thing"})
	writeFixture(t, profileDir, "fixture/defs.schema.json", map[string]any{"$defs": map[string]any{"thing": map[string]any{"allOf": []any{map[string]any{"$ref": "../schemas/game-data/model.schema.json#/$defs/namedModel"}, map[string]any{"properties": map[string]any{"name": map[string]any{"enum": []string{"a", "b"}}}}}}}})
	editDocument(t, profileDir, "collections.json", func(p map[string]any) {
		p["collections"].([]any)[0].(map[string]any)["schema"] = "fixture/entry.schema.json"
	})
	r, status := Validate(data, profileDir, false)
	if status != 0 {
		t.Fatalf("local schemas rejected: %+v", r.Errors)
	}
	writeFixture(t, data, "Things/c.json", map[string]any{"$type": "Example.ThingModel", "name": "c"})
	r, status = Validate(data, profileDir, false)
	if status != 1 || !hasError(r, "schema", "Things/c.json", "") {
		t.Fatalf("nested definition was not enforced: %+v", r)
	}
}

func TestProfileDependenciesStayLocal(t *testing.T) {
	for _, ref := range []string{"https://example.invalid/schema.json", "../../../outside.schema.json", "file:///tmp/outside.schema.json"} {
		t.Run(ref, func(t *testing.T) {
			_, profileDir := fixture(t)
			writeFixture(t, filepath.Dir(profileDir), "outside.schema.json", map[string]any{"type": "object"})
			writeFixture(t, profileDir, "schemas/game-data/weapon.schema.json", map[string]any{"$ref": ref})
			if _, _, err := loadProfile(profileDir); err == nil {
				t.Fatal("Profile accepted an external dependency")
			}
		})
	}
	t.Run("escaping symlink", func(t *testing.T) {
		_, profileDir := fixture(t)
		outside := filepath.Join(t.TempDir(), "external.json")
		writeFixture(t, filepath.Dir(outside), filepath.Base(outside), map[string]any{"type": "object"})
		path := filepath.Join(profileDir, "schemas/game-data/weapon.schema.json")
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, path); err != nil {
			t.Fatal(err)
		}
		if _, _, err := loadProfile(profileDir); err == nil {
			t.Fatal("Profile accepted an escaping symlink")
		}
	})
}

func TestProfileDigest(t *testing.T) {
	_, firstDir := fixture(t)
	_, secondDir := fixture(t)
	first, _, err := loadProfile(firstDir)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := loadProfile(secondDir)
	if err != nil {
		t.Fatal(err)
	}
	if first.Identity.SHA256 != second.Identity.SHA256 || first.Identity.Files == 0 {
		t.Fatal("identical Profile contents must have the same digest in different directories")
	}
	writeFixture(t, secondDir, "unused.json", map[string]any{"unused": true})
	unused, _, err := loadProfile(secondDir)
	if err != nil {
		t.Fatal(err)
	}
	if first.Identity != unused.Identity {
		t.Fatal("unreferenced file altered dependency identity")
	}
	editDocument(t, secondDir, "schemas/game-data/model.schema.json", func(p map[string]any) { p["description"] = "Changed dependency content" })
	changed, _, err := loadProfile(secondDir)
	if err != nil {
		t.Fatal(err)
	}
	if changed.Identity.SHA256 == first.Identity.SHA256 {
		t.Fatal("changing a referenced schema did not change the digest")
	}
}

func TestMechanicProposalSchema(t *testing.T) {
	_, profileDir := fixture(t)
	p, schemas, err := loadProfile(profileDir)
	if err != nil {
		t.Fatal(err)
	}
	schema := schemas[p.Manifest.ProposalSchema]
	proposal := map[string]any{"mechanic": map[string]any{"id": "new-effect", "models": []any{"NewModel"}, "schema": "new.schema.json", "role": "effect"}, "reason": "Explain a reviewed raw model."}
	if err := schema.Validate(proposal); err != nil {
		t.Fatalf("valid proposal rejected: %v", err)
	}
	delete(proposal, "reason")
	if err := schema.Validate(proposal); err == nil {
		t.Fatal("proposal without a reason accepted")
	}
	proposal["reason"] = "Review this mechanic."
	proposal["mechanic"].(map[string]any)["role"] = "invalid"
	if err := schema.Validate(proposal); err == nil {
		t.Fatal("proposal with invalid mechanic accepted")
	}
	// Compiling the proposal also requires its transitive dependencies.
	writeFixture(t, profileDir, "schemas/profile/mechanic-proposal.schema.json", map[string]any{"$ref": "missing.schema.json"})
	if _, _, err := loadProfile(profileDir); err == nil {
		t.Fatal("missing proposal dependency accepted")
	}
}

func TestMalformedProfileJSON(t *testing.T) {
	for _, raw := range []string{`{"rules":[],"rules":[]}`, `{"rules":[]} {}`, `{"rules":`} {
		t.Run(raw, func(t *testing.T) {
			data, profileDir := fixture(t)
			if err := os.WriteFile(filepath.Join(profileDir, "rules.json"), []byte(raw), 0644); err != nil {
				t.Fatal(err)
			}
			r, status := Validate(data, profileDir, false)
			if status != 2 || !hasError(r, "profile", "", "") || r.FilesChecked != 0 {
				t.Fatalf("malformed Profile accepted: %+v, status=%d", r, status)
			}
		})
	}
}

func TestScopeMemberSchemaIsRequiredAndResolved(t *testing.T) {
	for _, member := range []string{"", "missing.schema.json"} {
		t.Run(member, func(t *testing.T) {
			data, profileDir := fixture(t)
			scope := map[string]any{"name": "ordinary", "collection": "things", "rootSchema": "schemas/game-data/model.schema.json", "edgesField": "buys", "targetField": "next"}
			if member != "" {
				scope["memberSchema"] = member
			}
			editDocument(t, profileDir, "collections.json", func(p map[string]any) { p["scopes"] = []any{scope} })
			r, status := Validate(data, profileDir, false)
			if status != 2 || r.FilesChecked != 0 || !hasError(r, "profile", "", "") {
				t.Fatalf("invalid membership selector accepted: %+v, status=%d", r, status)
			}
		})
	}
}

func TestTowerClassificationFieldsCannotEvadeValidation(t *testing.T) {
	p, schemas, err := loadProfile(filepath.Join("..", "..", "profile"))
	if err != nil {
		t.Fatal(err)
	}
	validate := func(tower map[string]any) error {
		view := p.project(tower).(map[string]any)
		if err := p.requiredModelFields("tower", view); err != nil {
			return err
		}
		return schemas[p.ModelSchemas["tower"]].Validate(view)
	}
	// The starter Profile explicitly requires its native base-state flag.
	tower := map[string]any{"kind": "tower", "id": "test", "familyId": "test", "placementPrice": 0.0, "range": 0.0, "buildTiers": []any{0.0}, "upgradeOptions": []any{}, "appliedUpgrades": []any{}, "behaviors": []any{}, "isBase": true}
	if err := validate(tower); err != nil {
		t.Fatalf("valid minimum Tower rejected: %v", err)
	}
	for _, field := range []string{"isBase"} {
		t.Run(field, func(t *testing.T) {
			original := tower[field]
			delete(tower, field)
			if err := validate(tower); err == nil {
				t.Fatalf("missing %s accepted", field)
			}
			for _, malformed := range []any{nil, "false", 0.0} {
				tower[field] = malformed
				if err := validate(tower); err == nil {
					t.Fatalf("malformed %s=%v accepted", field, malformed)
				}
			}
			tower[field] = original
		})
	}
}

func TestCollectionAndRequiredPathsAreNormalized(t *testing.T) {
	tests := []struct {
		name, collectionPath, requiredPath string
	}{
		{"leading dot", "./Things", "./text.json"},
		{"trailing slash", "Things/", "text.json/"},
		{"interior parent", "unused/../Things", "unused/../text.json"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data, profileDir := fixture(t)
			editDocument(t, profileDir, "collections.json", func(p map[string]any) {
				p["collections"].([]any)[0].(map[string]any)["path"] = test.collectionPath
				p["collections"].([]any)[1].(map[string]any)["path"] = test.requiredPath
			})
			r, status := Validate(data, profileDir, false)
			if status != 0 || !r.Valid || r.Coverage.FilesWithSchema != 3 || r.ReferencesChecked != 1 {
				t.Fatalf("equivalent local paths lost collection or required-file checks: %+v, status=%d", r, status)
			}

			// An unbound model must still receive its collection's schema and identity checks.
			writeFixture(t, data, "Things/a.json", map[string]any{"$type": "Example.UnboundModel"})
			r, status = Validate(data, profileDir, false)
			if status != 1 || !hasError(r, "schema", "Things/a.json", "") || !hasError(r, "identity", "Things/a.json", "/name") {
				t.Fatalf("normalized collection path bypassed validation: %+v, status=%d", r, status)
			}
			if err := os.Remove(filepath.Join(data, "text.json")); err != nil {
				t.Fatal(err)
			}
			r, status = Validate(data, profileDir, false)
			if status != 1 || !hasError(r, "missing_file", "text.json", "") {
				t.Fatalf("normalized required-file path lost missing-file check: %+v, status=%d", r, status)
			}
		})
	}
}

func TestRootCollectionIncludesNestedRecords(t *testing.T) {
	data, profileDir := fixture(t)
	editDocument(t, profileDir, "collections.json", func(p map[string]any) {
		p["collections"] = []any{map[string]any{"name": "things", "path": ".", "idField": "@path", "schema": "schemas/game-data/model.schema.json#/$defs/namedModel"}}
	})
	writeFixture(t, data, "text.json", map[string]any{"$type": "Example.TextModel", "name": "text"})
	writeFixture(t, data, "Things/a.json", map[string]any{"$type": "Example.ThingModel", "name": "a", "target": "Things/b.json"})
	r, status := Validate(data, profileDir, true)
	if status != 0 || !r.Valid || r.Coverage.FilesWithSchema != 3 || len(r.Relations) != 1 {
		t.Fatalf("root collection did not include root and nested files: %+v, status=%d", r, status)
	}
	if r.Relations[0].TargetID != "Things/b.json" || len(r.Backlinks["Things/b.json"]) != 1 {
		t.Fatalf("root collection lost relative path identities: %+v", r)
	}
}
