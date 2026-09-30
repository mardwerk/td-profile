package atlasvalidate

import (
	"reflect"
	"sort"
	"testing"
)

func TestKeyIdentitiesExcludeOnlyConfiguredKeys(t *testing.T) {
	value := map[string]any{"$type": "metadata", "$label": "Label", "kind": "Kind", "id": "Identity"}
	for _, test := range []struct {
		name     string
		excluded []string
		want     []string
	}{
		{"default", nil, []string{"$label", "$type", "id", "kind"}},
		{"one metadata key", []string{"$type"}, []string{"$label", "id", "kind"}},
		{"explicit label exclusion", []string{"$type", "$label"}, []string{"id", "kind"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			ids := recordIDs(collection{IDField: "@keys", ExcludedKeys: test.excluded}, "table.json", value)
			sort.Strings(ids)
			if !reflect.DeepEqual(ids, test.want) {
				t.Fatalf("identities = %v, want %v", ids, test.want)
			}
		})
	}
}

func TestRequiredTableIgnoresOnlyExcludedKeys(t *testing.T) {
	d, p, target := scoreFixture(t)
	setExclusions := func(keys []string) {
		editDocument(t, p, "collections.json", func(v map[string]any) {
			for _, value := range v["collections"].([]any) {
				c := value.(map[string]any)
				if c["name"] == "localization" {
					c["excludedKeys"] = keys
				}
			}
		})
	}
	setExclusions([]string{"metadata"})
	writeFixture(t, d, "strings/labels.json", map[string]any{"metadata": "Table metadata"})
	r, status := ScoreTower(d, p, target, false)
	if status != 1 || !hasError(r, "empty_required_role", "strings/labels.json", "") {
		t.Fatalf("metadata-only table counted as populated: %+v status=%d", r, status)
	}
	writeFixture(t, d, "strings/labels.json", map[string]any{"metadata": "Table metadata", "$label": "An ordinary label"})
	r, status = ScoreTower(d, p, target, false)
	if status != 0 || !r.Valid || r.Score.Points != 100 {
		t.Fatalf("unexcluded dollar-prefixed key did not populate table: %+v status=%d", r, status)
	}
	setExclusions([]string{"metadata", "$label"})
	r, status = ScoreTower(d, p, target, false)
	if status != 1 || !hasError(r, "empty_required_role", "strings/labels.json", "") {
		t.Fatalf("explicit label exclusion was ignored: %+v status=%d", r, status)
	}
}
