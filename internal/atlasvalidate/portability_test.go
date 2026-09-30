package atlasvalidate

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func schemaContents(t *testing.T, p string) map[string][32]byte {
	t.Helper()
	result := map[string][32]byte{}
	if err := filepath.WalkDir(filepath.Join(p, "schemas"), func(path string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(p, path)
		result[rel] = sha256.Sum256(raw)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return result
}
func TestDifferentGameProfilesReuseIdenticalSchemas(t *testing.T) {
	var reference map[string][32]byte
	variants := []struct{ discriminator, encoding, model, id, family, price string }{
		{"$type", "dotnet", "Example.DefenseUnit, Game", "name", "baseId", "cost"},
		{"entityClass", "literal", "Defender", "identifier", "lineage", "buildCost"},
		{"kind", "literal", "tower", "id", "familyId", "placementPrice"},
	}
	for _, variant := range variants {
		t.Run(variant.discriminator, func(t *testing.T) {
			data, p := fixture(t)
			if err := os.RemoveAll(data); err != nil {
				t.Fatal(err)
			}
			writeFixture(t, data, "Units/unit.json", map[string]any{variant.discriminator: variant.model, variant.id: "unit", variant.family: "family", variant.price: 10.0})
			writeFixture(t, p, "collections.json", map[string]any{"collections": []any{map[string]any{"name": "units", "path": "Units", "idField": variant.id, "schema": "schemas/game-data/tower.schema.json"}}, "scopes": []any{}})
			writeFixture(t, p, "references.json", map[string]any{"references": []any{}})
			model := variant.model
			if variant.encoding == "dotnet" {
				model = typeName(model)
			}
			writeFixture(t, p, "mechanics.json", map[string]any{"typeIdentity": map[string]any{"field": variant.discriminator, "encoding": variant.encoding}, "unknownModels": "error", "mechanics": []any{map[string]any{"id": "tower", "models": []string{model}, "schema": "schemas/game-data/tower.schema.json", "role": "structure", "fields": map[string]string{"id": variant.id, "familyId": variant.family, "placementPrice": variant.price}, "requiredFields": []string{"placementPrice"}}}})
			writeFixture(t, p, "numerical-units.json", map[string]any{"units": []any{map[string]any{"id": "credits", "dimension": "currency", "description": "Native currency."}}, "bindings": []any{map[string]any{"models": []string{model}, "field": variant.price, "unit": "credits", "meaning": "Placement price."}}})
			editDocument(t, p, "scoring.json", func(v map[string]any) { v["collection"] = "units"; v["familyField"] = variant.family })
			if reference == nil {
				reference = schemaContents(t, p)
			} else if !reflect.DeepEqual(reference, schemaContents(t, p)) {
				t.Fatal("game-specific settings changed shared schema files")
			}
			r, status := Validate(data, p, false)
			if status != 0 || !r.Valid || r.Coverage.BoundModelInstances != 1 {
				t.Fatalf("portable source failed: %+v status=%d", r, status)
			}
			scored, status := ScoreTower(data, p, filepath.Join(data, "Units/unit.json"), false)
			if status != 0 || scored.Score.Points != 100 {
				t.Fatalf("complete portable score failed: %+v", scored)
			}
			// Profile-requiredness retains strict checks without requiring BTD6-only flags.
			editDocument(t, data, "Units/unit.json", func(v map[string]any) { delete(v, variant.price) })
			r, status = Validate(data, p, false)
			if status != 1 || !hasError(r, "schema", "Units/unit.json", "") {
				t.Fatalf("binding-required source field skipped: %+v", r)
			}
		})
	}
}
func TestSourceSelectorsRejectUnknownNamesAndCycles(t *testing.T) {
	for _, selectors := range []map[string]any{
		{"one": map[string]any{"all": []string{"missing"}}},
		{"one": map[string]any{"all": []string{"two"}}, "two": map[string]any{"all": []string{"one"}}},
	} {
		data, p := fixture(t)
		editDocument(t, p, "rules.json", func(v map[string]any) { v["selectors"] = selectors })
		r, status := Validate(data, p, false)
		if status != 2 || r.FilesChecked != 0 || !hasError(r, "profile", "rules.json", "") {
			t.Fatalf("invalid selector references accepted: %+v", r)
		}
	}
}
func TestSharedSchemasContainNoBTDSourceBindings(t *testing.T) {
	root := filepath.Join("..", "..", "profile", "schemas")
	markers := []string{"$type", "BloonModel", "TowerModel", "HeroModel", "IsBaseTower", "isSubTower", "isParagon", "towerSet", "baseId", "LocsKey", "FrontierLegends"}
	if err := filepath.WalkDir(root, func(path string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, marker := range markers {
			if strings.Contains(string(raw), marker) {
				t.Errorf("source binding %q leaked into shared schema %s", marker, path)
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
