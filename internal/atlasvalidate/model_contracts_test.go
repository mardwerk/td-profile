package atlasvalidate

import (
	"strings"
	"testing"
)

func contractFixture(t *testing.T) modelContractIndex {
	t.Helper()
	zero := 0
	doc := modelContractsDocument{RequireContracts: true, Contracts: []modelContract{
		{SourceType: "game.Attack, Assembly", Fields: []modelContractField{
			{Field: "rate/~", Shape: modelShape{Type: "number"}},
			{Field: "target", Shape: modelShape{AnyOf: []modelShape{{Type: "null"}, {Type: "model", Models: []string{"game.Target, Assembly"}}}}},
			{Field: "flags", Shape: modelShape{Type: "object", AdditionalFields: &modelShape{Type: "boolean"}}},
			{Field: "points", Shape: modelShape{Type: "array", Items: &modelShape{Type: "integer"}}},
			{Field: "empty", Shape: modelShape{Type: "array", MaxItems: &zero}},
			{Field: "special", Shape: modelShape{AnyOf: []modelShape{{Type: "number"}, {Type: "string", Enum: []string{"NaN"}}}}},
		}},
		{SourceType: "game.Target, Assembly", Fields: []modelContractField{{Field: "priority", Shape: modelShape{Type: "integer"}}}},
	}}
	index, err := compileModelContracts(doc, typeIdentity{Field: "class", Encoding: "literal"})
	if err != nil {
		t.Fatal(err)
	}
	return index
}

func validContractRecord() map[string]any {
	return map[string]any{"class": "game.Attack, Assembly", "rate/~": 0.5, "target": map[string]any{"class": "game.Target, Assembly", "priority": float64(1)}, "flags": map[string]any{"camo": true}, "points": []any{float64(1), float64(2)}, "empty": []any{}, "special": "NaN"}
}

func TestSourceContractStrictFieldChecks(t *testing.T) {
	index := contractFixture(t)
	tests := []struct {
		name, pointer, message string
		change                 func(map[string]any)
	}{
		{"valid", "", "", func(map[string]any) {}},
		{"missing field", "/root/rate~1~0", "required source field", func(v map[string]any) { delete(v, "rate/~") }},
		{"unexpected field", "/root/new", "unexpected source field", func(v map[string]any) { v["new"] = true }},
		{"numeric type", "/root/rate~1~0", "expected number", func(v map[string]any) { v["rate/~"] = "0.5" }},
		{"wrong nested class", "/root/target", "not allowed here", func(v map[string]any) { v["target"] = map[string]any{"class": "game.Attack, Assembly"} }},
		{"missing nested discriminator", "/root/target", "nested model requires", func(v map[string]any) { v["target"] = map[string]any{} }},
		{"nullable nested model", "", "", func(v map[string]any) { v["target"] = nil }},
		{"dictionary value", "/root/flags/camo", "expected boolean", func(v map[string]any) { v["flags"] = map[string]any{"camo": "true"} }},
		{"fractional integer", "/root/points/1", "expected integer", func(v map[string]any) { v["points"] = []any{float64(1), 1.5} }},
		{"observed empty array", "/root/empty", "maximum is 0", func(v map[string]any) { v["empty"] = []any{nil} }},
		{"arbitrary numeric string", "/root/special", "no allowed shape matched", func(v map[string]any) { v["special"] = "hello" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := validContractRecord()
			tt.change(value)
			var report Report
			if !index.check(value, "tower.json", "/root", &report) {
				t.Fatal("known exact source type should be bound")
			}
			if tt.message == "" {
				if len(report.Errors) != 0 {
					t.Fatalf("unexpected errors: %+v", report.Errors)
				}
				return
			}
			if len(report.Errors) != 1 {
				t.Fatalf("expected one error, got %+v", report.Errors)
			}
			got := report.Errors[0]
			if got.Pointer != tt.pointer || got.Code != "model_contract" || got.File != "tower.json" || !strings.Contains(got.Message, tt.message) {
				t.Fatalf("unexpected diagnostic: %+v", got)
			}
		})
	}
}

func TestSourceContractExactDiscriminatorAndIndependentChildChecks(t *testing.T) {
	index := contractFixture(t)
	for _, kind := range []any{"other.Attack, Assembly", "game.Attack, OtherAssembly", float64(3), "", nil} {
		var report Report
		if index.check(map[string]any{"class": kind}, "tower.json", "", &report) {
			t.Fatalf("unexpected match for %v", kind)
		}
		if len(report.Errors) != 1 || report.Errors[0].Pointer != "/class" {
			t.Fatalf("missing exact-type diagnostic: %+v", report.Errors)
		}
	}
	value := validContractRecord()
	child := value["target"].(map[string]any)
	child["priority"] = "bad"
	var report Report
	index.check(value, "tower.json", "", &report)
	if len(report.Errors) != 0 {
		t.Fatal("parent should defer child contract to regular model walk")
	}
	index.check(child, "tower.json", "/target", &report)
	if len(report.Errors) != 1 || report.Errors[0].Pointer != "/target/priority" {
		t.Fatalf("child contract missing: %+v", report.Errors)
	}
	index.Required = false
	report = Report{}
	if index.check(map[string]any{"class": "new"}, "tower.json", "", &report) || len(report.Errors) != 0 {
		t.Fatal("optional source coverage should not fail unknown types")
	}
}

func TestSourceContractRejectsInvalidConfiguration(t *testing.T) {
	zero, negative := 0, -1
	tests := []struct {
		name  string
		shape modelShape
		want  string
	}{
		{"unknown shape", modelShape{Type: "anything"}, "unknown shape type"},
		{"unknown model", modelShape{Type: "model", Models: []string{"Missing"}}, "unknown nested model"},
		{"no model list", modelShape{Type: "model"}, "requires allowed source types"},
		{"duplicate model", modelShape{Type: "model", Models: []string{"Root", "Root"}}, "duplicate nested model"},
		{"duplicate field", modelShape{Type: "object", Fields: []modelContractField{{Field: "x", Shape: modelShape{Type: "null"}}, {Field: "x", Shape: modelShape{Type: "null"}}}}, "duplicate field"},
		{"empty union", modelShape{AnyOf: []modelShape{}}, "at least two alternatives"},
		{"ambiguous union", modelShape{Type: "string", AnyOf: []modelShape{{Type: "null"}, {Type: "string"}}}, "cannot be combined"},
		{"untyped array", modelShape{Type: "array"}, "array requires items"},
		{"negative maximum", modelShape{Type: "array", MaxItems: &negative}, "must be nonnegative"},
		{"wrong field keyword", modelShape{Type: "number", Fields: []modelContractField{}}, "require object"},
		{"wrong array keyword", modelShape{Type: "string", MaxItems: &zero}, "require array"},
		{"wrong model keyword", modelShape{Type: "null", Models: []string{}}, "requires model"},
		{"wrong enum keyword", modelShape{Type: "number", Enum: []string{"NaN"}}, "requires string"},
		{"empty enum", modelShape{Type: "string", Enum: []string{}}, "must not be empty"},
		{"duplicate enum", modelShape{Type: "string", Enum: []string{"x", "x"}}, "duplicate enum"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := compileModelContracts(modelContractsDocument{Contracts: []modelContract{{SourceType: "Root", Fields: []modelContractField{{Field: "payload", Shape: tt.shape}}}}}, typeIdentity{Field: "class"})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("wanted %q, got %v", tt.want, err)
			}
		})
	}
	for _, contracts := range [][]modelContract{
		{{SourceType: ""}},
		{{SourceType: "Root"}, {SourceType: "Root"}},
		{{SourceType: "Root", Fields: []modelContractField{{Field: "class", Shape: modelShape{Type: "string"}}}}},
	} {
		if _, err := compileModelContracts(modelContractsDocument{Contracts: contracts}, typeIdentity{Field: "class"}); err == nil {
			t.Fatalf("accepted invalid contracts %+v", contracts)
		}
	}
}

func TestSourceContractAllowsRecursiveModelsButBoundsInlineNesting(t *testing.T) {
	recursive := modelContract{SourceType: "Node", Fields: []modelContractField{{Field: "children", Shape: modelShape{Type: "array", Items: &modelShape{Type: "model", Models: []string{"Node"}}}}}}
	if _, err := compileModelContracts(modelContractsDocument{Contracts: []modelContract{recursive}}, typeIdentity{Field: "class"}); err != nil {
		t.Fatal(err)
	}
	shape := modelShape{Type: "null"}
	for i := 0; i < maxModelShapeDepth+1; i++ {
		child := shape
		shape = modelShape{Type: "array", Items: &child}
	}
	_, err := compileModelContracts(modelContractsDocument{Contracts: []modelContract{{SourceType: "Root", Fields: []modelContractField{{Field: "payload", Shape: shape}}}}}, typeIdentity{Field: "class"})
	if err == nil || !strings.Contains(err.Error(), "nesting exceeds") {
		t.Fatalf("expected nesting limit, got %v", err)
	}
}

func TestSourceContractDictionaryDiscriminatorAndNestedObject(t *testing.T) {
	index, err := compileModelContracts(modelContractsDocument{RequireContracts: true, Contracts: []modelContract{
		{SourceType: "Dictionary", Fields: []modelContractField{}, AdditionalFields: &modelShape{Type: "object", Fields: []modelContractField{{Field: "enabled", Shape: modelShape{Type: "boolean"}}}}},
	}}, typeIdentity{Field: "entity/type"})
	if err != nil {
		t.Fatal(err)
	}
	value := map[string]any{"entity/type": "Dictionary", "arbitrary/key": map[string]any{"enabled": true}}
	var report Report
	if !index.check(value, "map.json", "", &report) || len(report.Errors) > 0 {
		t.Fatalf("discriminator should be excluded from dictionary values: %+v", report.Errors)
	}
	value["arbitrary/key"] = map[string]any{"other": true}
	index.check(value, "map.json", "", &report)
	if len(report.Errors) != 2 || report.Errors[0].Pointer != "/arbitrary~1key/enabled" || report.Errors[1].Pointer != "/arbitrary~1key/other" {
		t.Fatalf("nested object must enforce missing and unknown fields: %+v", report.Errors)
	}
}
