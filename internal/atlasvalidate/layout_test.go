package atlasvalidate

import (
	"os"
	"path/filepath"
	"testing"
)

func layoutFixture(t *testing.T, collectionPath string, layout any) (string, string) {
	t.Helper()
	data, profileDir := fixture(t)
	if err := os.RemoveAll(data); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(data, 0755); err != nil {
		t.Fatal(err)
	}
	collection := map[string]any{"name": "creatures", "path": collectionPath, "idField": "key", "schema": "layout-record.schema.json"}
	if layout != nil {
		collection["layout"] = layout
	}
	writeFixture(t, profileDir, "layout-record.schema.json", map[string]any{"type": "object"})
	writeFixture(t, profileDir, "collections.json", map[string]any{"collections": []any{collection}, "scopes": []any{}})
	writeFixture(t, profileDir, "references.json", map[string]any{"references": []any{}})
	editDocument(t, profileDir, "scoring.json", func(p map[string]any) { p["collection"] = "creatures" })
	return data, profileDir
}

func TestCollectionLayoutChecksConfiguredFields(t *testing.T) {
	cases := []struct {
		name, path, code string
		change           func(map[string]any)
	}{
		{"grouped record", "Catalog/reef/fin.json", "", nil},
		{"wrong depth", "Catalog/fin.json", "layout_depth", nil},
		{"extra directory", "Catalog/extra/reef/fin.json", "layout_depth", nil},
		{"wrong parent", "Catalog/forest/fin.json", "layout_parent", nil},
		{"wrong stem", "Catalog/reef/tail.json", "layout_stem", nil},
		{"missing parent field", "Catalog/reef/fin.json", "layout_field", func(v map[string]any) { delete(v, "clan") }},
		{"nonstring parent field", "Catalog/reef/fin.json", "layout_field", func(v map[string]any) { v["clan"] = 3 }},
		{"missing stem field", "Catalog/reef/fin.json", "layout_field", func(v map[string]any) { delete(v, "code") }},
		{"nonstring stem field", "Catalog/reef/fin.json", "layout_field", func(v map[string]any) { v["code"] = false }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			data, profileDir := layoutFixture(t, "Catalog", map[string]any{"depth": 2, "parentField": "clan", "stemField": "code"})
			value := map[string]any{"key": "record-identity", "clan": "reef", "code": "fin"}
			if test.change != nil {
				test.change(value)
			}
			writeFixture(t, data, test.path, value)
			r, status := Validate(data, profileDir, false)
			if r.Coverage.LayoutFilesChecked != 1 {
				t.Fatalf("expected one layout check, got %+v", r.Coverage)
			}
			if test.code == "" {
				if status != 0 || !r.Valid {
					t.Fatalf("valid configured layout rejected: %+v, status=%d", r, status)
				}
			} else if status != 1 || r.Valid || !hasError(r, test.code, test.path, "") {
				t.Fatalf("expected %s for %s, got %+v, status=%d", test.code, test.path, r, status)
			}
		})
	}
}

func TestCollectionLayoutDepthConventions(t *testing.T) {
	for _, test := range []struct {
		name, collectionPath, path string
		depth                      int
	}{
		{"singleton", "fin.json", "fin.json", 0},
		{"flat collection", "Catalog", "Catalog/fin.json", 1},
		{"root collection", ".", "fin.json", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, profileDir := layoutFixture(t, test.collectionPath, map[string]any{"depth": test.depth, "stemField": "code"})
			writeFixture(t, data, test.path, map[string]any{"key": "identity", "code": "fin"})
			r, status := Validate(data, profileDir, false)
			if status != 0 || !r.Valid || r.Coverage.LayoutFilesChecked != 1 {
				t.Fatalf("valid depth convention rejected: %+v, status=%d", r, status)
			}
		})
	}
}

func TestCollectionLayoutCoverageCountsDistinctFiles(t *testing.T) {
	data, profileDir := layoutFixture(t, "Catalog", map[string]any{"depth": 2, "parentField": "clan"})
	editDocument(t, profileDir, "collections.json", func(p map[string]any) {
		p["collections"] = append(p["collections"].([]any), map[string]any{"name": "byPath", "path": "Catalog", "idField": "@path", "schema": "layout-record.schema.json", "layout": map[string]any{"depth": 2, "stemField": "code"}})
	})
	writeFixture(t, data, "Catalog/reef/fin.json", map[string]any{"key": "identity", "clan": "reef", "code": "fin"})
	r, status := Validate(data, profileDir, false)
	if status != 0 || !r.Valid || r.Coverage.LayoutFilesChecked != 1 {
		t.Fatalf("secondary collection must not double-count layout coverage: %+v, status=%d", r, status)
	}
	editDocument(t, data, "Catalog/reef/fin.json", func(v map[string]any) { v["code"] = "tail" })
	r, status = Validate(data, profileDir, false)
	if status != 1 || !hasError(r, "layout_stem", "Catalog/reef/fin.json", "") || r.Coverage.LayoutFilesChecked != 1 {
		t.Fatalf("secondary collection must still enforce its own layout: %+v, status=%d", r, status)
	}
}

func TestCollectionWithoutLayoutAllowsFileRenaming(t *testing.T) {
	data, profileDir := layoutFixture(t, "Catalog", nil)
	writeFixture(t, data, "Catalog/reef/fin.json", map[string]any{"key": "identity", "clan": "reef", "code": "fin"})
	if err := os.MkdirAll(filepath.Join(data, "Catalog", "different", "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(data, "Catalog", "reef", "fin.json"), filepath.Join(data, "Catalog", "different", "nested", "renamed.json")); err != nil {
		t.Fatal(err)
	}
	r, status := Validate(data, profileDir, false)
	if status != 0 || !r.Valid || r.Coverage.LayoutFilesChecked != 0 {
		t.Fatalf("absent layout should preserve identity-based file naming: %+v, status=%d", r, status)
	}
}

func TestInvalidCollectionLayoutFailsBeforeData(t *testing.T) {
	for _, test := range []struct {
		name   string
		layout map[string]any
	}{
		{"unknown key", map[string]any{"depth": 2, "typo": true}},
		{"negative depth", map[string]any{"depth": -1}},
		{"fractional depth", map[string]any{"depth": 1.5}},
		{"string depth", map[string]any{"depth": "2"}},
		{"missing depth", map[string]any{"parentField": "clan"}},
		{"empty parent field", map[string]any{"depth": 2, "parentField": ""}},
		{"empty stem field", map[string]any{"depth": 2, "stemField": ""}},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, profileDir := layoutFixture(t, "Catalog", test.layout)
			writeFixture(t, data, "Catalog/reef/fin.json", map[string]any{"key": "identity"})
			r, status := Validate(data, profileDir, false)
			if status != 2 || r.Valid || r.FilesChecked != 0 || !hasError(r, "profile", "", "") {
				t.Fatalf("invalid layout must fail Profile loading: %+v, status=%d", r, status)
			}
		})
	}
}

func TestUnmatchedFilePolicy(t *testing.T) {
	for _, policy := range []string{"", "report", "error", "invalid"} {
		t.Run("policy="+policy, func(t *testing.T) {
			data, profileDir := layoutFixture(t, "Catalog", map[string]any{"depth": 1})
			if policy != "" {
				editDocument(t, profileDir, "collections.json", func(p map[string]any) { p["unmatchedFiles"] = policy })
			}
			writeFixture(t, data, "stray.json", map[string]any{"key": "outside"})
			r, status := Validate(data, profileDir, false)
			if policy == "invalid" {
				if status != 2 || r.Valid || r.FilesChecked != 0 || !hasError(r, "profile", "", "") {
					t.Fatalf("unknown unmatched-file policy accepted: %+v, status=%d", r, status)
				}
				return
			}
			if r.Coverage.FilesWithoutSchema != 1 || r.Coverage.FilesWithSchema != 0 || r.Coverage.LayoutFilesChecked != 0 {
				t.Fatalf("unmatched file coverage is incorrect: %+v", r.Coverage)
			}
			if policy == "error" {
				if status != 1 || r.Valid || !hasError(r, "unmatched_file", "stray.json", "") {
					t.Fatalf("strict policy accepted stray record: %+v, status=%d", r, status)
				}
			} else if status != 0 || !r.Valid {
				t.Fatalf("report policy should allow unmatched records: %+v, status=%d", r, status)
			}
		})
	}
}

func TestStrictUnmatchedPolicyRejectsRecordMovedOutsideCollection(t *testing.T) {
	data, profileDir := layoutFixture(t, "Catalog", map[string]any{"depth": 1, "stemField": "key"})
	editDocument(t, profileDir, "collections.json", func(p map[string]any) { p["unmatchedFiles"] = "error" })
	writeFixture(t, data, "Catalog/fin.json", map[string]any{"key": "fin"})
	r, status := Validate(data, profileDir, false)
	if status != 0 || !r.Valid || r.Coverage.FilesWithSchema != 1 || r.Coverage.LayoutFilesChecked != 1 {
		t.Fatalf("strict policy rejected known path: %+v, status=%d", r, status)
	}
	if err := os.Rename(filepath.Join(data, "Catalog", "fin.json"), filepath.Join(data, "fin.json")); err != nil {
		t.Fatal(err)
	}
	r, status = Validate(data, profileDir, false)
	if status != 1 || r.Valid || !hasError(r, "unmatched_file", "fin.json", "") || r.Coverage.FilesWithoutSchema != 1 || r.Coverage.LayoutFilesChecked != 0 {
		t.Fatalf("moving outside the collection bypassed strict policy: %+v, status=%d", r, status)
	}
}
