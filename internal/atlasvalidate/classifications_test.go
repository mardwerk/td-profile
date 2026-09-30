package atlasvalidate

import (
	"testing"
)

func classificationFixture(t *testing.T) (string, string, string) {
	t.Helper()
	data, profileDir, target := scoreFixture(t)
	editDocument(t, profileDir, "manifest.json", func(v map[string]any) {
		v["documents"].(map[string]any)["classifications"] = map[string]any{"file": "classifications.json", "schema": "schemas/profile/classifications.schema.json"}
	})
	writeFixture(t, profileDir, "classifications.json", map[string]any{"groups": []any{map[string]any{"collection": "things", "types": []any{
		map[string]any{"id": "permanent", "selectorSchema": "generic.schema.json", "rule": "progression", "membership": "checked", "validation": "progression"},
		map[string]any{"id": "form", "selectorSchema": "generic.schema.json", "rule": "progression", "membership": "unchecked", "validation": "schemaAndReferences", "reason": "Alternate record outside permanent purchases."},
	}}}})
	return data, profileDir, target
}
func TestClassificationsShareRecordSchemaAndRespectGraphMembership(t *testing.T) {
	data, p, target := classificationFixture(t)
	writeFixture(t, data, "Things/family/form.json", map[string]any{"kind": "unit", "name": "form", "family": "family"})
	r, status := ScoreTower(data, p, target, false)
	if status != 0 || len(r.RecordTypes) != 2 || r.RecordTypes[0].Records != 3 || r.RecordTypes[1].Records != 1 || r.RecordTypes[1].Validation != "schemaAndReferences" {
		t.Fatalf("graph classification: %+v status=%d", r, status)
	}
	// Every returned category states its actual coverage, without inventing a ladder for forms.
	if r.RecordTypes[0].Rule != "progression" || r.RecordTypes[1].Reason == "" {
		t.Fatalf("classification lost declared coverage: %+v", r.RecordTypes)
	}
	writeFixture(t, data, "Things/unrelated/unrelated.json", map[string]any{"kind": "unit", "name": "unrelated", "family": "unrelated"})
	after, status := ScoreTower(data, p, target, false)
	if status != 0 || after.FilesChecked != r.FilesChecked || after.Score.Points != r.Score.Points || after.RecordTypes[1].Records != 1 {
		t.Fatalf("unrelated classification leaked into score: %+v", after)
	}
}
func TestUnknownAndAmbiguousRecordTypesFail(t *testing.T) {
	for _, ambiguous := range []bool{false, true} {
		d, p, target := classificationFixture(t)
		editDocument(t, p, "classifications.json", func(v map[string]any) {
			group := v["groups"].([]any)[0].(map[string]any)
			if ambiguous {
				group["types"] = append(group["types"].([]any), map[string]any{"id": "duplicate", "selectorSchema": "generic.schema.json", "rule": "progression", "membership": "checked", "validation": "progression"})
			} else {
				group["types"] = group["types"].([]any)[1:]
			}
		})
		r, status := ScoreTower(d, p, target, false)
		code := "unknown_record_type"
		if ambiguous {
			code = "ambiguous_record_type"
		}
		if status != 1 || r.IntegrityValid || !hasError(r, code, "Things/family/base.json", "") || r.Score.Points >= 100 {
			t.Fatalf("unaccounted type accepted: %+v status=%d", r, status)
		}
	}
}
func TestClassificationConfigurationIsValidatedBeforeData(t *testing.T) {
	tests := []func(map[string]any){
		func(v map[string]any) { v["groups"].([]any)[0].(map[string]any)["collection"] = "missing" },
		func(v map[string]any) { v["groups"] = append(v["groups"].([]any), v["groups"].([]any)[0]) },
		func(v map[string]any) {
			types := v["groups"].([]any)[0].(map[string]any)["types"].([]any)
			types[0].(map[string]any)["rule"] = "missing"
		},
		func(v map[string]any) {
			types := v["groups"].([]any)[0].(map[string]any)["types"].([]any)
			delete(types[0].(map[string]any), "membership")
		},
		func(v map[string]any) {
			types := v["groups"].([]any)[0].(map[string]any)["types"].([]any)
			types[0].(map[string]any)["selectorSchema"] = "missing.schema.json"
		},
		func(v map[string]any) {
			types := v["groups"].([]any)[0].(map[string]any)["types"].([]any)
			types[1].(map[string]any)["id"] = types[0].(map[string]any)["id"]
		},
		func(v map[string]any) {
			types := v["groups"].([]any)[0].(map[string]any)["types"].([]any)
			delete(types[1].(map[string]any), "reason")
		},
	}
	for i, edit := range tests {
		d, p, _ := classificationFixture(t)
		editDocument(t, p, "classifications.json", edit)
		r, status := Validate(d, p, false)
		if status != 2 || r.FilesChecked != 0 || !hasError(r, "profile", "", "") {
			t.Fatalf("case %d invalid classification accepted: %+v status=%d", i, r, status)
		}
	}
}
func TestClassificationCanDescribeAnotherCollection(t *testing.T) {
	d, p, _ := classificationFixture(t)
	editDocument(t, p, "classifications.json", func(v map[string]any) {
		v["groups"] = []any{map[string]any{"collection": "purchases", "types": []any{map[string]any{"id": "purchaseDefinition", "selectorSchema": "generic.schema.json", "validation": "schemaAndReferences", "reason": "Referenced reusable purchase record."}}}}
	})
	r, status := Validate(d, p, false)
	if status != 0 || len(r.RecordTypes) != 1 || r.RecordTypes[0].Collection != "purchases" || r.RecordTypes[0].Records != 1 {
		t.Fatalf("classification is Tower-specific: %+v status=%d", r, status)
	}

}
