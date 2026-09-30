package atlasvalidate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const fixtureSourceType = "Example.ThingModel, Assembly-CSharp"

func sourceContractIntegrationFixture(t *testing.T) (string, string) {
	t.Helper()
	data, p := fixture(t)
	if err := os.Remove(filepath.Join(data, "Things/b.json")); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, data, "Things/a.json", map[string]any{
		"$type": fixtureSourceType, "name": "a", "family": "a", "power": 1.5,
		"child": map[string]any{"$type": "Example.ChildModel, Assembly-CSharp", "enabled": true},
	})
	editDocument(t, p, "manifest.json", func(v map[string]any) {
		v["documents"].(map[string]any)["modelContracts"] = map[string]any{"file": "model-contracts.json", "schema": "schemas/profile/model-contracts.schema.json"}
	})
	writeFixture(t, p, "model-contracts.json", map[string]any{"requireContracts": true, "contracts": integrationContracts()})
	return data, p
}

func integrationContracts() []modelContract {
	return []modelContract{
		{SourceType: fixtureSourceType, Fields: []modelContractField{
			{Field: "name", Shape: modelShape{Type: "string"}},
			{Field: "family", Shape: modelShape{Type: "string"}},
			{Field: "power", Shape: modelShape{Type: "number"}},
			{Field: "child", Shape: modelShape{Type: "model", Models: []string{"Example.ChildModel, Assembly-CSharp"}}},
		}},
		{SourceType: "Example.ChildModel, Assembly-CSharp", Fields: []modelContractField{{Field: "enabled", Shape: modelShape{Type: "boolean"}}}},
		{SourceType: "Example.OtherModel, Assembly-CSharp", Fields: []modelContractField{}},
	}
}

func TestSourceContractIntegrationStructuralCoverageCanBeComplete(t *testing.T) {
	data, p := sourceContractIntegrationFixture(t)
	editDocument(t, p, "mechanics.json", func(v map[string]any) {
		v["unknownModels"] = "error"
		for _, raw := range v["mechanics"].([]any) {
			binding := raw.(map[string]any)
			if binding["id"] == "thing" {
				binding["models"] = []string{"UnusedThingModel"}
			}
		}
	})
	report, status := ScoreTower(data, p, filepath.Join(data, "Things/a.json"), false)
	if status != 0 || !report.Valid || report.Score == nil || report.Score.Points != 100 || !report.Score.Complete {
		t.Fatalf("complete structural score failed: status=%d report=%+v score=%+v", status, report, report.Score)
	}
	if report.Coverage.CanonicalSchemaModelInstances != 0 || report.Coverage.StructuralContractModelInstances != 2 || report.Coverage.BoundModelInstances != 2 || report.Coverage.UnboundModelInstances != 0 {
		t.Fatalf("coverage should distinguish structural contracts from canonical schemas: %+v", report.Coverage)
	}
	if report.ModelContracts == nil || report.ModelContracts.Types != 3 || !report.ModelContracts.Required {
		t.Fatalf("missing source contract report: %+v", report.ModelContracts)
	}
}

func TestSourceContractIntegrationDataFailuresProducePartialScores(t *testing.T) {
	tests := []struct {
		name, pointer, message string
		change                 func(map[string]any)
	}{
		{"missing field", "/power", "required source field", func(v map[string]any) { delete(v, "power") }},
		{"wrong primitive", "/power", "expected number", func(v map[string]any) { v["power"] = "1.5" }},
		{"unexpected field", "/unrecognized", "unexpected source field", func(v map[string]any) { v["unrecognized"] = true }},
		{"incompatible known nested model", "/child/$type", "not allowed here", func(v map[string]any) { v["child"] = map[string]any{"$type": "Example.OtherModel, Assembly-CSharp"} }},
		{"invalid nested source field", "/child/enabled", "expected boolean", func(v map[string]any) { v["child"].(map[string]any)["enabled"] = "true" }},
		{"same short kind different exact type", "/$type", "no source contract", func(v map[string]any) { v["$type"] = "Another.ThingModel, Assembly-CSharp" }},
		{"empty normalized model type", "/$type", "", func(v map[string]any) { v["$type"] = "." }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, p := sourceContractIntegrationFixture(t)
			editDocument(t, data, "Things/a.json", tt.change)
			report, status := ScoreTower(data, p, filepath.Join(data, "Things/a.json"), false)
			if status != 1 || report.Valid || report.Score == nil || report.Score.Complete || report.Score.Points <= 0 || report.Score.Points >= 100 {
				t.Fatalf("invalid source must earn a partial score: status=%d report=%+v score=%+v", status, report, report.Score)
			}
			found := false
			for _, d := range report.Errors {
				if d.File == "Things/a.json" && d.Pointer == tt.pointer && strings.Contains(d.Message, tt.message) {
					found = true
				}
			}
			if !found {
				t.Fatalf("missing diagnostic at %s with %q: %+v", tt.pointer, tt.message, report.Errors)
			}
			if tt.name == "same short kind different exact type" && (report.Coverage.CanonicalSchemaModelInstances != 1 || report.Coverage.UnboundModelInstances != 1) {
				t.Fatalf("canonical short-name match must not satisfy required exact source contract: %+v", report.Coverage)
			}
		})
	}
}

func TestSourceContractIntegrationIncludesContributeToDigest(t *testing.T) {
	data, p := sourceContractIntegrationFixture(t)
	writeFixture(t, p, "model-contracts.json", map[string]any{"requireContracts": true, "includes": []string{"contracts/left.json", "contracts/right.json"}})
	writeFixture(t, p, "contracts/left.json", map[string]any{"includes": []string{"shared.json"}})
	writeFixture(t, p, "contracts/right.json", map[string]any{"includes": []string{"shared.json"}})
	writeFixture(t, p, "contracts/shared.json", map[string]any{"contracts": integrationContracts()})
	first, status := Validate(data, p, false)
	if status != 0 || !first.Valid || first.ModelContracts == nil || first.ModelContracts.Types != 3 {
		t.Fatalf("shared include should load only once: %+v status=%d", first, status)
	}
	contracts := integrationContracts()
	contracts[0].Fields[0].Shape.Enum = []string{"a"}
	writeFixture(t, p, "contracts/shared.json", map[string]any{"contracts": contracts})
	after, status := Validate(data, p, false)
	if status != 0 || !after.Valid || first.Profile.SHA256 == after.Profile.SHA256 || first.Profile.Files != after.Profile.Files {
		t.Fatalf("nested included contract must affect profile digest: before=%+v after=%+v status=%d errors=%+v", first.Profile, after.Profile, status, after.Errors)
	}
}

func TestSourceContractIntegrationInvalidDocumentsFailBeforeData(t *testing.T) {
	tests := []struct {
		name, file, message string
		change              func(*testing.T, string)
	}{
		{"missing include", "missing.json", "", func(t *testing.T, p string) {
			writeFixture(t, p, "model-contracts.json", map[string]any{"includes": []string{"missing.json"}})
		}},
		{"cyclic include", "model-contracts.json", "include cycle", func(t *testing.T, p string) {
			writeFixture(t, p, "model-contracts.json", map[string]any{"includes": []string{"child.json"}})
			writeFixture(t, p, "child.json", map[string]any{"includes": []string{"model-contracts.json"}})
		}},
		{"nonlocal include", "model-contracts.json", "local relative path", func(t *testing.T, p string) {
			writeFixture(t, p, "model-contracts.json", map[string]any{"includes": []string{"../outside.json"}})
		}},
		{"unknown nested source type", "model-contracts.json", "unknown nested model type", func(t *testing.T, p string) {
			contracts := integrationContracts()
			contracts[0].Fields[3].Shape.Models = []string{"Absent"}
			writeFixture(t, p, "model-contracts.json", map[string]any{"contracts": contracts})
		}},
		{"duplicate source type", "model-contracts.json", "duplicate source contract", func(t *testing.T, p string) {
			contracts := integrationContracts()
			contracts = append(contracts, contracts[0])
			writeFixture(t, p, "model-contracts.json", map[string]any{"contracts": contracts})
		}},
		{"duplicate source field", "model-contracts.json", "duplicate field", func(t *testing.T, p string) {
			contracts := integrationContracts()
			contracts[0].Fields = append(contracts[0].Fields, contracts[0].Fields[0])
			writeFixture(t, p, "model-contracts.json", map[string]any{"contracts": contracts})
		}},
		{"invalid included schema", "child.json", "", func(t *testing.T, p string) {
			writeFixture(t, p, "model-contracts.json", map[string]any{"includes": []string{"child.json"}})
			writeFixture(t, p, "child.json", map[string]any{"contracts": []any{map[string]any{"sourceType": "Example", "fields": []any{}, "invented": true}}})
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, p := sourceContractIntegrationFixture(t)
			tt.change(t, p)
			report, status := Validate(data, p, false)
			if status != 2 || report.FilesChecked != 0 || !hasError(report, "profile", tt.file, "") {
				t.Fatalf("invalid source contract setup must fail before data: status=%d report=%+v", status, report)
			}
			if len(report.Errors) != 1 || !strings.Contains(report.Errors[0].Message, tt.message) {
				t.Fatalf("expected %q: %+v", tt.message, report.Errors)
			}
		})
	}
}
