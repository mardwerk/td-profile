package atlasvalidate

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func templateExample(t *testing.T) (string, string, string) {
	t.Helper()
	data := t.TempDir()
	source := filepath.Join("..", "..", "examples", "minimal-game", "game-data")
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(data, relative)
		if entry.IsDir() {
			return os.MkdirAll(destination, 0755)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, raw, 0644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return data, filepath.Join("..", "..", "profile"), filepath.Join(data, "Towers", "Bolt", "Bolt-0.json")
}

func TestTemplateEmptyGameDataPasses(t *testing.T) {
	report, status := Validate(filepath.Join("..", "..", "game-data"), filepath.Join("..", "..", "profile"), false)
	if status != 0 || !report.Valid || report.FilesChecked != 0 {
		t.Fatalf("empty template failed: %+v", report)
	}
}

func TestTemplateExampleCompleteScore(t *testing.T) {
	data, profile, target := templateExample(t)
	report, status := Validate(data, profile, false)
	if status != 0 || !report.Valid || len(report.Rules) != 1 || report.Rules[0].ExpectedStates != 3 {
		t.Fatalf("example failed: %+v", report)
	}
	report, status = ScoreTower(data, profile, target, false)
	if status != 0 || report.Score == nil || report.Score.Points != 100 || !report.Score.Complete || report.Coverage.UnboundModelInstances != 0 || report.Coverage.StructuralContractModelInstances != 0 {
		t.Fatalf("complete canonical score failed: %+v", report)
	}
}

func TestTemplateExampleRejectsMissingDependenciesAndInvalidModels(t *testing.T) {
	cases := []struct {
		name, code string
		change     func(*testing.T, string)
	}{
		{"missing upgrade", "missing_reference", func(t *testing.T, data string) {
			if err := os.Remove(filepath.Join(data, "Upgrades", "Power-1.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{"missing state", "missing_reference", func(t *testing.T, data string) {
			if err := os.Remove(filepath.Join(data, "Towers", "Bolt", "Bolt-1.json")); err != nil {
				t.Fatal(err)
			}
		}},
		{"wrong source value", "schema", func(t *testing.T, data string) {
			editDocument(t, data, "Towers/Bolt/Bolt-0.json", func(v map[string]any) { v["placementPrice"] = "cheap" })
		}},
		{"unknown mechanic", "unknown_model", func(t *testing.T, data string) {
			editDocument(t, data, "Towers/Bolt/Bolt-0.json", func(v map[string]any) {
				v["behaviors"] = append(v["behaviors"].([]any), map[string]any{"kind": "unrecognized"})
			})
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			data, profile, target := templateExample(t)
			test.change(t, data)
			report, status := Validate(data, profile, false)
			if status != 1 || report.Valid || !hasError(report, test.code, "", "") {
				t.Fatalf("invalid example passed: %+v", report)
			}
			score, status := ScoreTower(data, profile, target, false)
			if status != 1 || score.Score == nil || score.Score.Points >= 100 || score.Score.Complete {
				t.Fatalf("invalid example scored complete: %+v", score)
			}
		})
	}
}

func TestTemplateScoreIgnoresUnrelatedMalformedContent(t *testing.T) {
	data, profile, target := templateExample(t)
	baseline, status := ScoreTower(data, profile, target, false)
	if status != 0 {
		t.Fatalf("baseline failed: %+v", baseline)
	}
	if err := os.MkdirAll(filepath.Join(data, "Towers", "Other"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "Towers", "Other", "Other.json"), []byte(`{"kind":`), 0644); err != nil {
		t.Fatal(err)
	}
	report, status := ScoreTower(data, profile, target, false)
	if status != 0 || !reflect.DeepEqual(baseline, report) {
		t.Fatalf("unrelated malformed content changed score: %+v", report)
	}
	report, status = Validate(data, profile, false)
	if status != 1 || !hasError(report, "json", "Towers/Other/Other.json", "") {
		t.Fatalf("whole-data validation missed malformed JSON: %+v", report)
	}
}
