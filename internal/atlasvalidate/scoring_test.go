package atlasvalidate

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func scoreFixture(t *testing.T) (string, string, string) {
	t.Helper()
	data, profileDir := fixture(t)
	if err := os.RemoveAll(data); err != nil {
		t.Fatal(err)
	}
	schema := map[string]any{"type": "object", "required": []string{"name", "kind"}, "properties": map[string]any{"name": map[string]any{"type": "string", "minLength": 1}, "kind": map[string]any{"type": "string", "minLength": 1}}}
	writeFixture(t, profileDir, "generic.schema.json", schema)
	writeFixture(t, profileDir, "root.schema.json", map[string]any{"required": []string{"origin"}, "properties": map[string]any{"origin": map[string]any{"const": true}}})
	writeFixture(t, profileDir, "member.schema.json", map[string]any{"required": []string{"levels"}})
	writeFixture(t, profileDir, "collections.json", map[string]any{
		"unmatchedFiles": "error",
		"collections": []any{
			map[string]any{"name": "things", "path": "Things", "idField": "name", "schema": "generic.schema.json", "layout": map[string]any{"depth": 2, "parentField": "family", "stemField": "name"}},
			map[string]any{"name": "purchases", "path": "Purchases", "idField": "name", "schema": "generic.schema.json", "layout": map[string]any{"depth": 1}},
			map[string]any{"name": "localization", "path": "strings/labels.json", "idField": "@keys", "schema": "schemas/game-data/model.schema.json#/$defs/stringTable", "requireWhen": []any{map[string]any{"collection": "things"}}},
			map[string]any{"name": "extraMode", "path": "extra.json", "idField": "@stem", "schema": "generic.schema.json", "enabled": false, "requireWhen": []any{map[string]any{"collection": "things"}}},
		},
		"scopes": []any{map[string]any{"name": "ordinary", "collection": "things", "rootSchema": "root.schema.json", "memberSchema": "member.schema.json", "edgesField": "buys", "targetField": "next"}},
	})
	writeFixture(t, profileDir, "mechanics.json", map[string]any{"typeIdentity": map[string]any{"field": "kind", "encoding": "literal"}, "unknownModels": "report", "mechanics": []any{map[string]any{"id": "structure", "models": []string{"unit", "purchase"}, "schema": "generic.schema.json", "role": "structure"}}})
	writeFixture(t, profileDir, "references.json", map[string]any{"references": []any{
		map[string]any{"models": []string{"purchase"}, "field": "next", "target": "things", "aliases": map[string]string{"old-first": "first"}},
		map[string]any{"models": []string{"purchase"}, "field": "definition", "target": "purchases"},
	}})
	writeFixture(t, profileDir, "numerical-units.json", map[string]any{"units": []any{map[string]any{"id": "credits", "dimension": "currency", "description": "Game currency."}}, "bindings": []any{map[string]any{"models": []string{"unit"}, "field": "cost", "unit": "credits", "meaning": "Purchase cost."}}})
	writeFixture(t, profileDir, "rules.json", map[string]any{"rules": []any{map[string]any{"id": "progression", "operation": "purchaseProgression", "scope": "ordinary", "tiersField": "levels", "familyField": "family", "requireCompleteStates": true, "limits": map[string]any{"pathCount": 1, "maxTier": 2, "maxPurchasedPaths": 1, "secondaryTierLimit": 2, "maxPathsAboveSecondaryTier": 0}}}})
	for i, name := range []string{"base", "first", "second"} {
		buys := []any{}
		if i < 2 {
			next := []string{"old-first", "second"}[i]
			buys = append(buys, map[string]any{"kind": "purchase", "name": "purchase", "next": next, "definition": "upgrade"})
		}
		writeFixture(t, data, "Things/family/"+name+".json", map[string]any{"kind": "unit", "name": name, "family": "family", "origin": i == 0, "levels": []int{i}, "buys": buys, "cost": float64(i)})
	}
	writeFixture(t, data, "Purchases/upgrade.json", map[string]any{"kind": "purchase", "name": "upgrade", "next": "base"}) // dependency cycle
	writeFixture(t, data, "strings/labels.json", map[string]any{"label": "A unit"})
	return data, profileDir, filepath.Join(data, "Things/family/base.json")
}
func TestScoreIsolationAndCompleteCoverage(t *testing.T) {
	data, p, target := scoreFixture(t)
	baseline, status := ScoreTower(data, p, target, true)
	if status != 0 || baseline.Score == nil || baseline.Score.Points != 100 || !baseline.Score.Complete || baseline.FilesChecked != 5 || len(baseline.Backlinks["Purchases/upgrade.json"]) != 2 {
		t.Fatalf("complete score: %+v status=%d", baseline, status)
	}
	writeFixture(t, data, "Things/other/other.json", map[string]any{"kind": "unsupported", "name": "other", "family": "other", "levels": []int{1}, "buys": []any{}})
	if err := os.WriteFile(filepath.Join(data, "Things/other/broken.json"), []byte(`{`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "extra.json"), []byte(`{`), 0644); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, filepath.Dir(data), "manifest.json", map[string]any{"irrelevant": true})
	after, status := ScoreTower(data, p, target, true)
	if status != 0 || !reflect.DeepEqual(baseline, after) {
		t.Fatalf("unrelated data contaminated score: before=%+v after=%+v status=%d", baseline, after, status)
	}
	if err := os.RemoveAll(filepath.Join(data, "Things/other")); err != nil {
		t.Fatal(err)
	}
	after, status = ScoreTower(data, p, target, true)
	if status != 0 || !reflect.DeepEqual(baseline, after) {
		t.Fatal("unrelated deletion changed score")
	}
}
func TestScoreFailuresAndWeights(t *testing.T) {
	tests := []struct {
		name, code string
		change     func(*testing.T, string, string)
	}{
		{"missing state", "missing_reference", func(t *testing.T, d, p string) {
			if err := os.Remove(filepath.Join(d, "Things/family/first.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{"missing root", "missing_reference", func(t *testing.T, d, p string) {
			if err := os.Remove(filepath.Join(d, "Things/family/base.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{"missing purchase", "missing_reference", func(t *testing.T, d, p string) {
			if err := os.Remove(filepath.Join(d, "Purchases/upgrade.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{"missing role", "missing_file", func(t *testing.T, d, p string) {
			if err := os.Remove(filepath.Join(d, "strings/labels.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{"malformed family file", "json", func(t *testing.T, d, p string) {
			if err := os.WriteFile(filepath.Join(d, "Things/family/first.json"), []byte(`{`), 0644); err != nil {
				t.Fatal(err)
			}
		}},
		{"unknown mechanic", "", func(t *testing.T, d, p string) {
			editDocument(t, d, "Things/family/first.json", func(v map[string]any) { v["effect"] = map[string]any{"kind": "unknown"} })
		}},
		{"invalid unit", "unit_type", func(t *testing.T, d, p string) {
			editDocument(t, d, "Things/family/first.json", func(v map[string]any) { v["cost"] = "free" })
		}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			d, p, _ := scoreFixture(t)
			target := filepath.Join(d, "Things/family/second.json")
			test.change(t, d, p)
			report, status := ScoreTower(d, p, target, false)
			if report.Score == nil || report.Score.Points <= 0 || report.Score.Points >= 100 || report.Score.Complete || (test.code != "" && (status != 1 || !hasError(report, test.code, "", ""))) {
				t.Fatalf("failure score: %+v status=%d", report, status)
			}
			previous := report.Score.Points
			editDocument(t, p, "scoring.json", func(v map[string]any) { v["weights"].(map[string]any)["units"] = 100.0 })
			weighted, _ := ScoreTower(d, p, target, false)
			if test.name == "invalid unit" && weighted.Score.Points >= previous {
				t.Fatal("weight did not change score")
			}
		})
	}
}
func TestScoreCannotRoundIncompleteCoverageTo100(t *testing.T) {
	p := profile{Scoring: scoringDocument{Weights: map[string]float64{"schema": 1, "layout": 1, "references": 1, "rules": 1, "units": 1, "mechanics": 1}}}
	r := Report{Coverage: Coverage{BoundModelInstances: 1000000, UnboundModelInstances: 1}}
	score := scoreReport(p, &r)
	if score.Points >= 100 || score.Complete {
		t.Fatalf("unknown coverage rounded to 100: %+v", score)
	}
}
func TestConditionalRolesAndDisabledData(t *testing.T) {
	d, p, _ := scoreFixture(t)
	if err := os.RemoveAll(filepath.Join(d, "Things")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(d, "Purchases")); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(filepath.Join(d, "strings")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "extra.json"), []byte(`{`), 0644); err != nil {
		t.Fatal(err)
	}
	r, status := Validate(d, p, false)
	if status != 0 || r.FilesChecked != 0 || r.Coverage.FilesSkipped != 1 {
		t.Fatalf("empty unused collections or disabled feature failed: %+v status=%d", r, status)
	}
	writeFixture(t, d, "Things/one/one.json", map[string]any{"kind": "unit", "name": "one", "family": "one"})
	r, status = Validate(d, p, false)
	if status != 1 || !hasError(r, "missing_file", "strings/labels.json", "") {
		t.Fatalf("consumer did not require relocated role: %+v", r)
	}
}
func TestGenericCaptureBindings(t *testing.T) {
	d, p := fixture(t)
	editDocument(t, p, "manifest.json", func(v map[string]any) {
		v["captureMetadata"] = map[string]any{"base": "gameData", "path": "capture/info.json", "fields": map[string]any{"revision": "revisionNumber"}}
	})
	writeFixture(t, d, "capture/info.json", map[string]any{"revisionNumber": 7, "unbound": "unused"})
	r, status := Validate(d, p, false)
	if status != 0 || len(r.Metadata) != 1 || r.Metadata["revision"] != float64(7) {
		t.Fatalf("generic metadata: %+v", r)
	}
}
func TestScoreCLI(t *testing.T) {
	d, p, target := scoreFixture(t)
	args := []string{"score-tower", "--profile", p, "--game-data", d, "--tower", target}
	var out, stderr bytes.Buffer
	if status := RunCLI(args, &out, &stderr); status != 0 {
		t.Fatalf("score CLI: %d %s %s", status, out.String(), stderr.String())
	}
	var r Report
	if err := json.Unmarshal(out.Bytes(), &r); err != nil || r.Score == nil || r.Score.Points != 100 {
		t.Fatalf("score output: %s", out.String())
	}
	out.Reset()
	if status := RunCLI(append(args, "--format", "text"), &out, &stderr); status != 0 || !strings.Contains(out.String(), "score=100.00/100") {
		t.Fatalf("text output: %s", out.String())
	}
	for _, missing := range []string{"--profile", "--game-data", "--tower"} {
		var trimmed []string
		for i := 0; i < len(args); i++ {
			if args[i] == missing {
				i++
				continue
			}
			trimmed = append(trimmed, args[i])
		}
		if status := RunCLI(trimmed, &out, &stderr); status != 2 {
			t.Fatalf("missing %s returned %d", missing, status)
		}
	}
	if _, status := ScoreTower(d, p, filepath.Join(t.TempDir(), "outside.json"), false); status != 2 {
		t.Fatal("outside target accepted")
	}
}

func TestNoOrdinaryMembersDoesNotRequireOrdinaryRoot(t *testing.T) {
	p, schemas, records := progressionFixture(t)
	for _, matches := range records["units"] {
		for _, rec := range matches {
			delete(rec.Value, "kind")
			rec.Value["origin"] = false
		}
	}
	var report Report
	checkRules(p, schemas, records, &report)
	if len(report.Errors) > 0 {
		t.Fatalf("out-of-scope-only collection failed: %+v", report.Errors)
	}
}
func TestRequiredTableCannotBeEmpty(t *testing.T) {
	d, p, target := scoreFixture(t)
	writeFixture(t, d, "strings/labels.json", map[string]any{})
	r, status := ScoreTower(d, p, target, false)
	if status != 1 || !hasError(r, "empty_required_role", "strings/labels.json", "") || r.Score.Points >= 100 {
		t.Fatalf("empty consumed table accepted: %+v status=%d", r, status)
	}
}
func TestInvalidNewProfileSettings(t *testing.T) {
	tests := []struct {
		file string
		edit func(map[string]any)
	}{
		{"scoring.json", func(v map[string]any) { v["weights"].(map[string]any)["schema"] = 0 }},
		{"scoring.json", func(v map[string]any) { v["weights"].(map[string]any)["unknown"] = 1 }},
		{"scoring.json", func(v map[string]any) { v["collection"] = "unknown" }},
		{"mechanics.json", func(v map[string]any) { v["typeIdentity"].(map[string]any)["encoding"] = "unknown" }},
		{"mechanics.json", func(v map[string]any) { v["typeIdentity"].(map[string]any)["field"] = "" }},
		{"manifest.json", func(v map[string]any) { v["captureMetadata"].(map[string]any)["path"] = "../outside.json" }},
		{"collections.json", func(v map[string]any) {
			v["collections"].([]any)[0].(map[string]any)["requireWhen"] = []any{map[string]any{"collection": "missing"}}
		}},
		{"collections.json", func(v map[string]any) {
			v["collections"].([]any)[0].(map[string]any)["requireWhen"] = []any{map[string]any{}}
		}},
		{"collections.json", func(v map[string]any) {
			v["collections"].([]any)[0].(map[string]any)["requireWhen"] = []any{map[string]any{"models": []any{"unit"}}}
		}},
		{"collections.json", func(v map[string]any) {
			v["collections"].([]any)[0].(map[string]any)["requireWhen"] = []any{map[string]any{"collection": "things", "models": []any{"unit"}, "field": "value"}}
		}},
		{"collections.json", func(v map[string]any) {
			v["collections"].([]any)[0].(map[string]any)["requireWhen"] = []any{map[string]any{"collection": "things", "selectorSchema": "missing.schema.json"}}
		}},
	}
	for _, test := range tests {
		t.Run(test.file, func(t *testing.T) {
			d, p, target := scoreFixture(t)
			editDocument(t, p, test.file, test.edit)
			r, status := ScoreTower(d, p, target, false)
			if status != 2 || r.FilesChecked != 0 || r.Score != nil || !hasError(r, "profile", "", "") {
				t.Fatalf("invalid profile accepted: %+v status=%d", r, status)
			}
		})
	}
}
func TestCollectionAndModelRoleSelectors(t *testing.T) {
	d, p, target := scoreFixture(t)
	writeFixture(t, p, "selected.schema.json", map[string]any{"required": []string{"advanced"}, "properties": map[string]any{"advanced": map[string]any{"const": true}}})
	editDocument(t, p, "collections.json", func(v map[string]any) {
		collections := v["collections"].([]any)
		collections = append(collections, map[string]any{"name": "advanced", "path": "advanced.json", "idField": "@stem", "schema": "generic.schema.json", "requireWhen": []any{map[string]any{"collection": "things", "selectorSchema": "selected.schema.json"}}})
		collections = append(collections, map[string]any{"name": "assets", "path": "assets.json", "idField": "@keys", "schema": "schemas/game-data/model.schema.json#/$defs/stringTable", "requireWhen": []any{map[string]any{"models": []any{"unit"}, "field": "asset"}}})
		v["collections"] = collections
	})
	r, status := ScoreTower(d, p, target, false)
	if status != 0 || r.Score.Points != 100 {
		t.Fatalf("inapplicable role required: %+v", r)
	}
	editDocument(t, d, "Things/family/first.json", func(v map[string]any) { v["advanced"] = true; v["asset"] = "asset-id" })
	r, status = ScoreTower(d, p, target, false)
	if status != 1 || !hasError(r, "missing_file", "advanced.json", "") || !hasError(r, "missing_file", "assets.json", "") {
		t.Fatalf("conditional roles absent: %+v", r)
	}
	writeFixture(t, d, "advanced.json", map[string]any{"name": "advanced", "kind": "purchase"})
	writeFixture(t, d, "assets.json", map[string]any{"asset-id": "asset-path"})
	r, status = ScoreTower(d, p, target, false)
	if status != 0 || r.Score.Points != 100 || r.FilesChecked != 7 {
		t.Fatalf("required roles not selected or validated: %+v status=%d", r, status)
	}
}
