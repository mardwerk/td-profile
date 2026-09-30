package atlasvalidate

import (
	"errors"
	"math"
	"os"
	"path/filepath"
)

type ScoreCategory struct {
	Category string  `json:"category"`
	Label    string  `json:"label"`
	Weight   float64 `json:"weight"`
	Checked  int     `json:"checked"`
	Passed   int     `json:"passed"`
}
type TowerScore struct {
	Tower      string          `json:"tower"`
	Points     float64         `json:"points"`
	Maximum    int             `json:"maximum"`
	Complete   bool            `json:"complete"`
	Categories []ScoreCategory `json:"categories"`
}

// ScoreTower checks a Tower family and its outgoing dependencies against a Profile.
// A score measures contract compliance, not balance or simulated behavior.
func ScoreTower(dataDirectory, profileDirectory, tower string, includeRelations bool) (Report, int) {
	return validate(dataDirectory, profileDirectory, includeRelations, tower)
}
func (r *Report) attempt(category, key string) {
	if r.checks == nil {
		r.checks = map[string]map[string]bool{}
	}
	if r.checks[category] == nil {
		r.checks[category] = map[string]bool{}
	}
	if _, exists := r.checks[category][key]; !exists {
		r.checks[category][key] = true
	}
}
func scoreReport(p profile, r *Report) *TowerScore {
	failed := map[string]map[string]bool{}
	fail := func(category, key string) {
		r.attempt(category, key)
		if failed[category] == nil {
			failed[category] = map[string]bool{}
		}
		failed[category][key] = true
	}
	for category, checks := range r.checks {
		for key, passed := range checks {
			if !passed {
				fail(category, key)
			}
		}
	}
	for _, diagnostic := range r.Errors {
		category := "schema"
		key := diagnostic.File
		if diagnostic.RuleID != "" {
			category = "rules"
			key = diagnostic.RuleID
		} else {
			switch diagnostic.Code {
			case "missing_file", "empty_required_role", "layout_depth", "layout_parent", "layout_stem", "layout_field":
				category = "layout"
			case "missing_reference", "reference_type":
				category = "references"
			case "unit_type":
				category = "units"
			case "unknown_model":
				continue // Coverage counts each model instance below.
			}
		}
		fail(category, key)
	}
	for _, rule := range r.Rules {
		if rule.Roots > 0 || rule.RecordsChecked > 0 || rule.Errors > 0 {
			r.attempt("rules", rule.ID)
		}
	}
	score := &TowerScore{Maximum: 100, Complete: true, Categories: []ScoreCategory{}}
	total, earned := 0.0, 0.0
	maxWeight := 0.0
	for _, weight := range p.Scoring.Weights {
		if weight > maxWeight {
			maxWeight = weight
		}
	}
	for _, name := range []string{"schema", "layout", "references", "rules", "units", "mechanics"} {
		checked := len(r.checks[name])
		passed := checked - len(failed[name])
		if name == "mechanics" {
			checked = r.Coverage.BoundModelInstances + r.Coverage.UnboundModelInstances
			passed = r.Coverage.BoundModelInstances
			// No recognized discriminator cannot establish complete mechanic coverage.
			if checked == 0 {
				checked = 1
				passed = 0
			}
		}
		label := name
		if name == "mechanics" {
			label = "model-schema coverage"
		}
		category := ScoreCategory{Category: name, Label: label, Weight: p.Scoring.Weights[name], Checked: checked, Passed: passed}
		score.Categories = append(score.Categories, category)
		if checked == 0 {
			continue
		}
		weight := category.Weight / maxWeight
		total += weight
		earned += weight * float64(passed) / float64(checked)
		if passed != checked {
			score.Complete = false
		}
	}
	if total > 0 {
		score.Points = math.Round(10000*earned/total) / 100
	}
	if !score.Complete && score.Points >= 100 {
		score.Points = 99.99
	}
	return score
}
func readCaptureMetadata(directory string, p profile, r *Report) {
	binding := p.Manifest.CaptureMetadata
	if binding == nil {
		return
	}
	base := directory
	if binding.Base == "parent" {
		base = filepath.Dir(filepath.Clean(directory))
	}
	path := filepath.Join(base, filepath.FromSlash(binding.Path))
	value, err := readJSON(path)
	if err != nil { // Metadata is optional, but a supplied invalid document is diagnosed.
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		r.add(binding.Path, "", "json", err.Error())
		return
	}
	object, ok := value.(map[string]any)
	if !ok {
		r.add(binding.Path, "", "schema", "capture metadata must be a JSON object")
		return
	}
	r.Metadata = map[string]any{}
	for label, field := range binding.Fields {
		if value, ok := object[field]; ok {
			r.Metadata[label] = value
		}
	}
}
