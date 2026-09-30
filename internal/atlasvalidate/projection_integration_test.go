package atlasvalidate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestTypedCollectionRequiresConfiguredRootDiscriminator(t *testing.T) {
	data, p := fixture(t)
	editDocument(t, p, "collections.json", func(v map[string]any) {
		v["collections"].([]any)[0].(map[string]any)["typed"] = true
	})
	// Generic-looking fields must not mask the configured raw discriminator.
	writeFixture(t, data, "Things/a.json", map[string]any{"kind": "thing", "id": "a", "name": "a"})
	r, status := Validate(data, p, false)
	if status != 1 || !hasError(r, "schema", "Things/a.json", "/$type") {
		t.Fatalf("missing raw discriminator accepted: %+v status=%d", r, status)
	}
	writeFixture(t, data, "Things/a.json", map[string]any{"$type": "Example.ThingModel", "name": "a"})
	// Tables may use these words as ordinary keys without becoming typed models.
	writeFixture(t, data, "text.json", map[string]any{"kind": "Label", "id": "Identifier"})
	r, status = Validate(data, p, false)
	if status != 0 || !r.Valid {
		t.Fatalf("untyped table rejected: %+v status=%d", r, status)
	}
}

func TestRootRequiredFieldsUseCollectionIdentityFallback(t *testing.T) {
	data, p := fixture(t)
	editDocument(t, p, "mechanics.json", func(v map[string]any) {
		binding := v["mechanics"].([]any)[0].(map[string]any)
		delete(binding, "fields")
		binding["requiredFields"] = []string{"id"}
	})
	r, status := Validate(data, p, false)
	if status != 0 || !r.Valid {
		t.Fatalf("root requiredness ignored collection identity: %+v status=%d", r, status)
	}
	// Embedded models cannot borrow their containing collection's identity.
	editDocument(t, data, "Things/a.json", func(v map[string]any) {
		v["child"] = map[string]any{"$type": "Example.ThingModel"}
	})
	r, status = Validate(data, p, false)
	if status != 1 || !hasError(r, "schema", "Things/a.json", "/child") {
		t.Fatalf("nested model borrowed collection identity: %+v status=%d", r, status)
	}
}

func TestOverlappingCollectionsValidateEachIdentityProjection(t *testing.T) {
	data, p := fixture(t)
	if err := os.Remove(filepath.Join(data, "Things/b.json")); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, data, "Things/a.json", map[string]any{"$type": "Example.ThingModel", "name": "a", "alternateName": "wrong"})
	writeFixture(t, p, "identity.schema.json", map[string]any{
		"type": "object", "required": []string{"kind", "id"},
		"properties": map[string]any{"id": map[string]any{"const": "a"}},
	})
	editDocument(t, p, "mechanics.json", func(v map[string]any) {
		delete(v["mechanics"].([]any)[0].(map[string]any), "fields")
	})
	editDocument(t, p, "collections.json", func(v map[string]any) {
		collections := v["collections"].([]any)
		collections[0].(map[string]any)["schema"] = "identity.schema.json"
		v["collections"] = append(collections, map[string]any{
			"name": "alternate", "path": "Things", "idField": "alternateName", "schema": "identity.schema.json",
		})
	})
	r, status := Validate(data, p, false)
	if status != 1 || !hasError(r, "schema", "Things/a.json", "") {
		t.Fatalf("shared schema skipped a different identity projection: %+v status=%d", r, status)
	}
}

func TestUnknownSourceKindCannotPassKnownTowerSchema(t *testing.T) {
	data, p := fixture(t)
	editDocument(t, p, "collections.json", func(v map[string]any) {
		c := v["collections"].([]any)[0].(map[string]any)
		c["schema"] = "schemas/game-data/tower.schema.json"
		c["typed"] = true
	})
	editDocument(t, p, "mechanics.json", func(v map[string]any) {
		v["mechanics"] = []any{map[string]any{
			"id": "tower", "models": []string{"ThingModel"}, "role": "structure",
			"schema": "schemas/game-data/tower.schema.json", "fields": map[string]string{"id": "name", "placementPrice": "cost"},
			"requiredFields": []string{"placementPrice"},
		}}
	})
	writeFixture(t, data, "Things/a.json", map[string]any{"$type": "tower", "id": "a", "name": "a"})
	writeFixture(t, data, "Things/b.json", map[string]any{"$type": "Example.ThingModel", "name": "b", "cost": float64(1)})
	r, status := Validate(data, p, false)
	if status != 1 || !hasError(r, "schema", "Things/a.json", "") || r.Coverage.UnboundModelInstances != 1 {
		t.Fatalf("unknown source kind passed known Tower schema: %+v status=%d", r, status)
	}
}

func TestUnknownSourceKindsStillFitGenericModels(t *testing.T) {
	data, p := fixture(t)
	writeFixture(t, data, "Things/a.json", map[string]any{"$type": "thing", "name": "a"})
	r, status := Validate(data, p, false)
	if status != 0 || !r.Valid || r.Coverage.UnboundModelInstances != 1 || r.Coverage.UnboundModelTypes[0].Type != "thing" {
		t.Fatalf("unknown generic model report semantics changed: %+v status=%d", r, status)
	}
}

func TestNestedCanonicalKindCannotReplaceSourceDiscriminator(t *testing.T) {
	data, p := fixture(t)
	writeFixture(t, data, "Things/a.json", map[string]any{
		"$type": "Example.AttackModel", "name": "a",
		"weapons": []any{map[string]any{"kind": "weapon", "interval": float64(1)}},
	})
	r, status := Validate(data, p, false)
	if status != 1 || !hasError(r, "schema", "Things/a.json", "") {
		t.Fatalf("nested generic kind masked missing source discriminator: %+v status=%d", r, status)
	}
	writeFixture(t, data, "Things/a.json", map[string]any{
		"$type": "Example.AttackModel", "name": "a",
		"weapons": []any{map[string]any{"$type": "Example.WeaponModel", "rate": float64(1)}},
	})
	r, status = Validate(data, p, false)
	if status != 0 || !r.Valid {
		t.Fatalf("restored nested source discriminator rejected: %+v status=%d", r, status)
	}
}

func TestLiteralTableKeysDoNotCreateModelsOrDependencies(t *testing.T) {
	data, p, target := scoreFixture(t)
	editDocument(t, p, "collections.json", func(v map[string]any) {
		v["collections"] = append(v["collections"].([]any), map[string]any{
			"name": "phantomRole", "path": "phantom.json", "idField": "@keys", "schema": "schemas/game-data/model.schema.json#/$defs/stringTable",
			"requireWhen": []any{map[string]any{"models": []string{"purchase"}, "field": "phantom"}},
		})
	})
	baseline, status := ScoreTower(data, p, target, false)
	if status != 0 {
		t.Fatalf("baseline failed: %+v", baseline)
	}
	writeFixture(t, data, "strings/labels.json", map[string]any{
		"kind": "purchase", "id": "Identity", "next": "unrelated", "definition": "missing", "phantom": "Label",
	})
	writeFixture(t, data, "Things/unrelated/unrelated.json", map[string]any{"kind": "unit", "name": "unrelated", "family": "unrelated", "levels": []int{1}})
	r, status := ScoreTower(data, p, target, false)
	if status != 0 || !reflect.DeepEqual(baseline, r) {
		t.Fatalf("table keys activated model checks, roles or dependencies: before=%+v after=%+v status=%d", baseline, r, status)
	}
}

func TestUntypedCatalogTraversesNestedDiscriminatorKey(t *testing.T) {
	data, p, _ := scoreFixture(t)
	writeFixture(t, p, "catalog.schema.json", map[string]any{"type": "object"})
	editDocument(t, p, "collections.json", func(v map[string]any) {
		v["collections"] = append(v["collections"].([]any), map[string]any{
			"name": "catalog", "path": "catalog.json", "idField": "@keys", "schema": "catalog.schema.json",
		})
	})
	writeFixture(t, data, "catalog.json", map[string]any{
		"kind": map[string]any{"kind": "purchase", "name": "nested", "next": "missing"},
	})
	r, status := Validate(data, p, false)
	if status != 1 || !hasError(r, "missing_reference", "catalog.json", "/kind/next") {
		t.Fatalf("catalog skipped a nested typed model: %+v status=%d", r, status)
	}
}

func TestTypedOverlapRetainsModelDetection(t *testing.T) {
	data, p, _ := scoreFixture(t)
	editDocument(t, p, "collections.json", func(v map[string]any) {
		v["collections"] = append(v["collections"].([]any), map[string]any{
			"name": "typedLabels", "path": "strings/labels.json", "idField": "name", "schema": "generic.schema.json", "typed": true,
		})
	})
	writeFixture(t, data, "strings/labels.json", map[string]any{"kind": "purchase", "name": "labels", "next": "missing"})
	r, status := Validate(data, p, false)
	if status != 1 || !hasError(r, "missing_reference", "strings/labels.json", "/next") {
		t.Fatalf("table collection suppressed overlapping typed checks: %+v status=%d", r, status)
	}
}

func TestCatalogModelValuesRequireSourceDiscriminators(t *testing.T) {
	data, p := fixture(t)
	writeFixture(t, p, "catalog.schema.json", map[string]any{
		"type": "object", "additionalProperties": map[string]any{"$ref": "schemas/game-data/model.schema.json#/$defs/model"},
	})
	editDocument(t, p, "collections.json", func(v map[string]any) {
		v["collections"] = append(v["collections"].([]any), map[string]any{
			"name": "catalog", "path": "catalog.json", "idField": "@keys", "schema": "catalog.schema.json",
		})
	})
	writeFixture(t, data, "catalog.json", map[string]any{"row": map[string]any{"kind": "thing"}})
	r, status := Validate(data, p, false)
	if status != 1 || !hasError(r, "schema", "catalog.json", "") {
		t.Fatalf("catalog entry's canonical kind masked missing source discriminator: %+v status=%d", r, status)
	}
	writeFixture(t, data, "catalog.json", map[string]any{"row": map[string]any{"$type": "Example.ThingModel", "name": "row"}})
	r, status = Validate(data, p, false)
	if status != 0 || !r.Valid {
		t.Fatalf("catalog entry with source discriminator rejected: %+v status=%d", r, status)
	}
}
