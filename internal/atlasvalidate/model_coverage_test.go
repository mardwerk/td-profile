package atlasvalidate

import "testing"

func TestMissingSchemaExamplesUseStableObjectLocations(t *testing.T) {
	data, profileDir := fixture(t)
	unknown := map[string]any{"$type": "Example.UnsupportedModel"}
	writeFixture(t, data, "Things/a.json", map[string]any{
		"$type": "Example.ThingModel", "name": "a",
		"z":   unknown,
		"a/b": []any{map[string]any{"~nested": unknown}},
	})
	writeFixture(t, data, "extra.json", unknown)
	r, status := Validate(data, profileDir, false)
	if status != 0 || len(r.Coverage.UnboundModelTypes) != 1 {
		t.Fatalf("unexpected validation result: %+v, status=%d", r, status)
	}
	model := r.Coverage.UnboundModelTypes[0]
	want := Location{File: "Things/a.json", Pointer: "/a~1b/0/~0nested"}
	if model.Type != "UnsupportedModel" || model.Instances != 3 || model.Example == nil || *model.Example != want {
		t.Fatalf("missing-schema example = %+v, want %+v", model, want)
	}
}
