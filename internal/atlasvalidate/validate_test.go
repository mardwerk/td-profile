package atlasvalidate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, root, name string, value any) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	data := filepath.Join(root, "game-data")
	profileDir := filepath.Join(root, "profile")
	if err := os.MkdirAll(profileDir, 0755); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join("..", "..", "profile")
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "model-contracts" {
			return filepath.SkipDir
		}
		if entry.Name() == "model-contracts.json" {
			return nil
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			return nil
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		destination := filepath.Join(profileDir, relative)
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			return err
		}
		return os.WriteFile(destination, raw, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}

	editDocument(t, profileDir, "manifest.json", func(v map[string]any) {
		documents := v["documents"].(map[string]any)
		delete(documents, "classifications")
		delete(documents, "modelContracts")
	})
	editDocument(t, profileDir, "manifest.json", func(v map[string]any) {
		v["captureMetadata"] = map[string]any{"path": "manifest.json", "base": "parent", "fields": map[string]string{"gameVersion": "gameVersion", "buildId": "steamBuildId", "exporterVersion": "atlasExporterVersion", "helperVersion": "modHelperVersion"}}
	})
	writeFixture(t, profileDir, "numerical-units.json", map[string]any{"units": []any{map[string]any{"id": "seconds", "dimension": "time", "description": "Fixture duration."}}, "bindings": []any{map[string]any{"models": []string{"ThingModel"}, "field": "duration", "unit": "seconds", "meaning": "Fixture duration in seconds."}}})
	writeFixture(t, data, "Things/a.json", map[string]any{"$type": "Example.ThingModel, Assembly-CSharp", "name": "a", "target": "b"})
	writeFixture(t, data, "Things/b.json", map[string]any{"$type": "Example.ThingModel, Assembly-CSharp", "name": "b"})
	writeFixture(t, data, "text.json", map[string]any{"hello": "Hello"})
	writeFixture(t, profileDir, "collections.json", map[string]any{
		"scopes":      []any{},
		"collections": []any{map[string]any{"name": "things", "path": "Things", "idField": "name", "schema": "schemas/game-data/model.schema.json#/$defs/namedModel"}, map[string]any{"name": "text", "path": "text.json", "idField": "@keys", "schema": "schemas/game-data/model.schema.json#/$defs/stringTable", "requireWhen": []any{map[string]any{"collection": "things"}}}},
	})
	writeFixture(t, profileDir, "references.json", map[string]any{"references": []any{map[string]any{"models": []string{"ThingModel"}, "field": "target", "target": "things", "aliases": map[string]string{"legacy-b": "b"}, "externalSymbols": map[string]string{"runtime-only": "This test symbol has no file definition."}}}})
	writeFixture(t, profileDir, "mechanics.json", map[string]any{"typeIdentity": map[string]any{"field": "$type", "encoding": "dotnet"}, "unknownModels": "report", "mechanics": []any{
		map[string]any{"id": "thing", "models": []string{"ThingModel"}, "schema": "schemas/game-data/model.schema.json#/$defs/namedModel", "role": "structure", "fields": map[string]string{"id": "name"}},
		map[string]any{"id": "weapon", "models": []string{"WeaponModel"}, "schema": "schemas/game-data/weapon.schema.json", "role": "structure", "fields": map[string]string{"id": "name", "interval": "rate", "projectile": "projectile", "emission": "emission"}},
		map[string]any{"id": "attack", "models": []string{"AttackModel"}, "schema": "schemas/game-data/attack.schema.json", "role": "structure"},
	}})
	writeFixture(t, profileDir, "rules.json", map[string]any{"rules": []any{}})
	editDocument(t, profileDir, "scoring.json", func(p map[string]any) { p["collection"] = "things"; p["familyField"] = "family" })
	return data, profileDir
}

func editDocument(t *testing.T, root, name string, edit func(map[string]any)) {
	t.Helper()
	value, err := readJSON(filepath.Join(root, name))
	if err != nil {
		t.Fatal(err)
	}
	document := value.(map[string]any)
	edit(document)
	writeFixture(t, root, name, document)
}

func hasError(r Report, code, file, pointer string) bool {
	for _, error := range r.Errors {
		if error.Code == code && (file == "" || error.File == file) && (pointer == "" || error.Pointer == pointer) {
			return true
		}
	}
	return false
}

func TestValidDataAndReferenceConventions(t *testing.T) {
	for _, target := range []string{"b", "legacy-b", "runtime-only"} {
		t.Run(target, func(t *testing.T) {
			data, profileDir := fixture(t)
			writeFixture(t, data, "Things/a.json", map[string]any{"$type": "Example.ThingModel, Assembly-CSharp", "name": "a", "target": target})
			r, status := Validate(data, profileDir, false)
			if status != 0 || !r.Valid || r.FilesChecked != 3 || r.ReferencesChecked != 1 {
				t.Fatalf("unexpected result: %+v, status=%d", r, status)
			}
		})
	}
}

func TestBrokenData(t *testing.T) {
	cases := []struct {
		name, code, file, pointer string
		change                    func(*testing.T, string)
	}{
		{"missing target", "missing_reference", "Things/a.json", "/target", func(t *testing.T, data string) {
			if err := os.Remove(filepath.Join(data, "Things/b.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{"dangling name without file deletion", "missing_reference", "Things/a.json", "/target", func(t *testing.T, data string) {
			writeFixture(t, data, "Things/a.json", map[string]any{"$type": "Example.ThingModel", "name": "a", "target": "missing"})
		}},
		{"nested missing target", "missing_reference", "Things/a.json", "/child/target", func(t *testing.T, data string) {
			writeFixture(t, data, "Things/a.json", map[string]any{"$type": "Example.ThingModel", "name": "a", "child": map[string]any{"$type": "Example.ThingModel", "name": "local", "target": "missing"}})
		}},
		{"alias target removed", "missing_reference", "Things/a.json", "/target", func(t *testing.T, data string) {
			writeFixture(t, data, "Things/a.json", map[string]any{"$type": "Example.ThingModel", "name": "a", "target": "legacy-b"})
			if err := os.Remove(filepath.Join(data, "Things/b.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{"required top-level file", "missing_file", "text.json", "", func(t *testing.T, data string) {
			if err := os.Remove(filepath.Join(data, "text.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{"duplicate identity", "duplicate_identity", "Things/b.json", "", func(t *testing.T, data string) {
			writeFixture(t, data, "Things/b.json", map[string]any{"$type": "Example.ThingModel", "name": "a"})
		}},
		{"invalid nested weapon interval", "schema", "Things/a.json", "/weapon", func(t *testing.T, data string) {
			writeFixture(t, data, "Things/a.json", map[string]any{"$type": "Example.ThingModel", "name": "a", "weapon": map[string]any{"$type": "Example.WeaponModel", "rate": "fast", "projectile": nil, "emission": nil}})
		}},
		{"wrong model type in weapon slot", "schema", "Things/a.json", "/attack", func(t *testing.T, data string) {
			writeFixture(t, data, "Things/a.json", map[string]any{"$type": "Example.ThingModel", "name": "a", "attack": map[string]any{
				"$type": "Example.AttackModel", "range": 40,
				"weapons": []any{map[string]any{"$type": "Example.WrongModel", "rate": 1, "projectile": nil, "emission": nil}},
			}})
		}},
		{"invalid type discriminator", "schema", "Things/a.json", "/$type", func(t *testing.T, data string) {
			writeFixture(t, data, "Things/a.json", map[string]any{"$type": 3, "name": "a"})
		}},
		{"invalid reference type", "reference_type", "Things/a.json", "/target", func(t *testing.T, data string) {
			writeFixture(t, data, "Things/a.json", map[string]any{"$type": "Example.ThingModel", "name": "a", "target": 3})
		}},
		{"invalid reference array", "reference_type", "Things/a.json", "/target/0", func(t *testing.T, data string) {
			writeFixture(t, data, "Things/a.json", map[string]any{"$type": "Example.ThingModel", "name": "a", "target": []any{[]string{"b"}}})
		}},
		{"duplicate JSON key", "json", "Things/a.json", "", func(t *testing.T, data string) {
			if err := os.WriteFile(filepath.Join(data, "Things/a.json"), []byte(`{"$type":"Example.ThingModel","name":"a","name":"b"}`), 0644); err != nil {
				t.Fatal(err)
			}
		}},
		{"trailing JSON value", "json", "Things/a.json", "", func(t *testing.T, data string) {
			if err := os.WriteFile(filepath.Join(data, "Things/a.json"), []byte(`{"$type":"Example.ThingModel","name":"a"} {}`), 0644); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			data, profileDir := fixture(t)
			test.change(t, data)
			r, status := Validate(data, profileDir, false)
			if status != 1 || r.Valid || !hasError(r, test.code, test.file, test.pointer) {
				t.Fatalf("expected %s at %s#%s, got %+v, status=%d", test.code, test.file, test.pointer, r, status)
			}
		})
	}
}

func TestDynamicDataAndRelations(t *testing.T) {
	data, profileDir := fixture(t)
	// Names come from records, so moving a file does not invalidate references.
	if err := os.Rename(filepath.Join(data, "Things/b.json"), filepath.Join(data, "Things/renamed.json")); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, data, "Things/c.json", map[string]any{"$type": "Example.ThingModel", "name": "c", "target": "legacy-b"})
	writeFixture(t, data, "extra.json", map[string]any{"customField": true})
	writeFixture(t, filepath.Dir(data), "manifest.json", map[string]any{"gameVersion": "another-version", "totalFiles": 999})
	r, status := Validate(data, profileDir, true)
	if status != 0 || !r.Valid || r.FilesChecked != 5 || r.Metadata["gameVersion"] != "another-version" || len(r.Relations) != 2 {
		t.Fatalf("dynamic data rejected: %+v, status=%d", r, status)
	}
	if len(r.Backlinks["Things/renamed.json"]) != 2 || r.Relations[1].TargetID != "b" || r.Relations[1].Value != "legacy-b" {
		t.Fatalf("incorrect graph: %+v", r)
	}
	// Removing an unreferenced record is allowed; no baseline inventory is needed.
	if err := os.Remove(filepath.Join(data, "Things/c.json")); err != nil {
		t.Fatal(err)
	}
	r, status = Validate(data, profileDir, false)
	if status != 0 || !r.Valid || r.FilesChecked != 4 || len(r.Relations) != 0 || len(r.Backlinks) != 0 {
		t.Fatalf("different collection size rejected: %+v, status=%d", r, status)
	}
}

func TestFamilyRelations(t *testing.T) {
	data, profileDir := fixture(t)
	editDocument(t, profileDir, "collections.json", func(p map[string]any) {
		p["collections"] = append(p["collections"].([]any), map[string]any{"name": "families", "path": "Things", "idField": "@parent", "schema": "schemas/game-data/model.schema.json#/$defs/namedModel"})
	})
	editDocument(t, profileDir, "references.json", func(p map[string]any) {
		p["references"] = append(p["references"].([]any), map[string]any{"models": []string{"ThingModel"}, "field": "family", "target": "families"})
	})
	writeFixture(t, data, "Things/a.json", map[string]any{"$type": "Example.ThingModel", "name": "a", "family": "Things"})
	r, status := Validate(data, profileDir, true)
	if status != 0 || len(r.Relations) != 1 || len(r.Relations[0].TargetFiles) != 2 || len(r.Backlinks["Things/b.json"]) != 1 {
		t.Fatalf("incorrect family graph: %+v, status=%d", r, status)
	}
}

func TestUnknownModelCoveragePolicy(t *testing.T) {
	data, profileDir := fixture(t)
	writeFixture(t, data, "extra.json", map[string]any{"$type": "Example.UnsupportedModel", "child": map[string]any{"$type": "Example.UnsupportedModel"}})
	r, status := Validate(data, profileDir, false)
	if status != 0 || !r.Valid || r.Coverage.FilesWithSchema != 3 || r.Coverage.FilesWithoutSchema != 1 || r.Coverage.BoundModelInstances != 2 || r.Coverage.UnboundModelInstances != 2 || len(r.Coverage.UnboundModelTypes) != 1 {
		t.Fatalf("inaccurate partial coverage: %+v, status=%d", r, status)
	}
	model := r.Coverage.UnboundModelTypes[0]
	if model.Type != "UnsupportedModel" || model.Instances != 2 || model.Example == nil || *model.Example != (Location{File: "extra.json", Pointer: ""}) {
		t.Fatalf("inaccurate missing-schema example: %+v", model)
	}
	editDocument(t, profileDir, "mechanics.json", func(p map[string]any) { p["unknownModels"] = "error" })
	r, status = Validate(data, profileDir, false)
	if status != 1 || r.IntegrityValid || !hasError(r, "unknown_model", "extra.json", "/$type") || !hasError(r, "unknown_model", "extra.json", "/child/$type") {
		t.Fatalf("complete-coverage requirement ignored: %+v", r)
	}
}

func TestNumericalUnitBindings(t *testing.T) {
	for _, test := range []struct {
		name  string
		value any
		valid bool
	}{
		{"numeric", 1.5, true}, {"zero cache", 0, true}, {"string", "seconds", false}, {"null", nil, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, profileDir := fixture(t)
			writeFixture(t, profileDir, "numerical-units.json", map[string]any{
				"units":    []any{map[string]any{"id": "seconds", "dimension": "time", "description": "Native duration."}},
				"bindings": []any{map[string]any{"models": []string{"ThingModel"}, "field": "duration", "unit": "seconds", "meaning": "Duration in seconds."}},
			})
			writeFixture(t, data, "Things/a.json", map[string]any{"$type": "Example.ThingModel", "name": "a", "duration": test.value})
			r, status := Validate(data, profileDir, false)
			if r.Valid != test.valid || r.Coverage.UnitBindingsChecked != 1 || (status == 0) != test.valid {
				t.Fatalf("unexpected numeric validation: %+v, status=%d", r, status)
			}
			if !test.valid && !hasError(r, "unit_type", "Things/a.json", "/duration") {
				t.Fatalf("missing unit diagnostic: %+v", r.Errors)
			}
		})
	}
}

func TestReportProvenanceAndExternalReference(t *testing.T) {
	data, profileDir := fixture(t)
	writeFixture(t, data, "Things/a.json", map[string]any{"$type": "Example.ThingModel", "name": "a", "target": "runtime-only"})
	writeFixture(t, filepath.Dir(data), "manifest.json", map[string]any{"gameVersion": "test-version", "steamBuildId": "build-123", "modHelperVersion": "helper-version", "atlasExporterVersion": "exporter-version"})
	r, status := Validate(data, profileDir, true)
	if status != 0 || !r.IntegrityValid || !r.RulesValid || r.Profile == nil || len(r.Profile.SHA256) != 64 || r.Checker.Version != Version || len(r.Checker.ExecutableSHA256) != 64 {
		t.Fatalf("missing checker/Profile provenance: %+v", r)
	}
	if r.Metadata["gameVersion"] != "test-version" || r.Metadata["buildId"] != "build-123" || r.Metadata["helperVersion"] != "helper-version" || r.Metadata["exporterVersion"] != "exporter-version" {
		t.Fatalf("missing configured capture metadata: %+v", r.Metadata)
	}
	if r.ExternalReferences != 1 || len(r.Relations) != 1 || !r.Relations[0].External || len(r.Relations[0].TargetFiles) != 0 || len(r.Backlinks) != 0 {
		t.Fatalf("incorrect external relation: %+v", r)
	}
}

func TestDeclaredProgressionIntegration(t *testing.T) {
	data, profileDir := fixture(t)
	writeFixture(t, profileDir, "fixture-member.schema.json", map[string]any{"type": "object", "required": []string{"name", "kind"}})
	writeFixture(t, profileDir, "fixture-root.schema.json", map[string]any{"type": "object", "required": []string{"origin"}, "properties": map[string]any{"origin": map[string]any{"const": true}}})
	editDocument(t, profileDir, "collections.json", func(p map[string]any) {
		p["scopes"] = []any{map[string]any{"name": "ordinary", "collection": "things", "rootSchema": "fixture-root.schema.json", "memberSchema": "fixture-member.schema.json", "edgesField": "buys", "targetField": "next"}}
	})
	writeFixture(t, profileDir, "rules.json", map[string]any{"rules": []any{map[string]any{
		"id": "fixture-progression", "operation": "purchaseProgression", "scope": "ordinary", "tiersField": "levels", "familyField": "kind", "requireCompleteStates": true,
		"limits": map[string]any{"pathCount": 1, "maxTier": 1, "maxPurchasedPaths": 1, "secondaryTierLimit": 1, "maxPathsAboveSecondaryTier": 0},
	}}})
	writeFixture(t, data, "Things/a.json", map[string]any{"$type": "Example.ThingModel", "name": "a", "origin": true, "kind": "ordinary", "levels": []int{0}, "buys": []any{map[string]any{"next": "b"}}})
	writeFixture(t, data, "Things/b.json", map[string]any{"$type": "Example.ThingModel", "name": "b", "kind": "ordinary", "levels": []int{1}, "buys": []any{}})
	writeFixture(t, data, "Things/temporary.json", map[string]any{"$type": "Example.ThingModel", "name": "temporary", "kind": "ordinary", "levels": []int{0}, "buys": []any{}})
	r, status := Validate(data, profileDir, false)
	if status != 0 || len(r.Rules) != 1 {
		t.Fatalf("valid configured progression rejected: %+v, status=%d", r, status)
	}
	result := r.Rules[0]
	if result.ID != "fixture-progression" || result.Roots != 1 || result.RecordsChecked != 2 || result.TransitionsChecked != 1 || result.ExpectedStates != 2 || result.OutOfScopeRecords != 1 || result.Errors != 0 {
		t.Fatalf("incorrect progression coverage: %+v", result)
	}
	editDocument(t, data, "Things/b.json", func(p map[string]any) { p["levels"] = []int{2} })
	r, status = Validate(data, profileDir, false)
	if status != 1 || !r.IntegrityValid || r.RulesValid || !hasError(r, "illegal-build", "Things/b.json", "/levels") {
		t.Fatalf("configured rule was not enforced: %+v, status=%d", r, status)
	}
	for _, diagnostic := range r.Errors {
		if diagnostic.RuleID != "fixture-progression" {
			t.Fatalf("missing rule attribution: %+v", diagnostic)
		}
	}
}
