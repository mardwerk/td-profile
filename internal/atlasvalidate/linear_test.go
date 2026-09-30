package atlasvalidate

import (
	"encoding/json"
	"math"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

func linearFixture(t *testing.T) (profile, schemaIndex, recordIndex) {
	t.Helper()
	p, schemas, records := progressionFixture(t)
	p.Rules[0] = rule{ID: "linear", Operation: "linearProgression", Scope: "purchases", FamilyField: "kind", LevelField: "rank", Levels: levelRange{First: 7, Last: 9}, RequireCompleteStates: true}
	for i, id := range []string{"base", "first", "second"} {
		records["units"][id][0].Value["rank"] = float64(i + 7)
	}
	return p, schemas, records
}

func TestLinearUsesConfiguredFieldsAndLevels(t *testing.T) {
	p, s, r := linearFixture(t)
	if err := validateRules(p); err != nil {
		t.Fatal(err)
	}
	var report Report
	checkRules(p, s, r, &report)
	if len(report.Errors) != 0 {
		t.Fatalf("valid linear progression: %+v", report.Errors)
	}
	got := report.Rules[0]
	if got.Roots != 1 || got.RecordsChecked != 3 || got.TransitionsChecked != 2 || got.ExpectedStates != 3 || got.OutOfScopeRecords != 0 {
		t.Fatalf("unexpected coverage: %+v", got)
	}
}

func TestLinearBrokenGraphs(t *testing.T) {
	tests := []struct {
		name, code string
		change     func(recordIndex)
	}{
		{"missing root", "missing-root", func(r recordIndex) { delete(r["units"], "base") }},
		{"missing level", "missing-level", func(r recordIndex) { delete(r["units"], "first") }},
		{"missing target", "purchase-target", func(r recordIndex) { delete(r["units"], "second") }},
		{"duplicate root", "duplicate-progression-root", func(r recordIndex) { r["units"]["first"][0].Value["origin"] = true }},
		{"duplicate level", "duplicate-level", func(r recordIndex) { r["units"]["second"][0].Value["rank"] = float64(8) }},
		{"invalid root level", "invalid-root-level", func(r recordIndex) { r["units"]["base"][0].Value["rank"] = float64(8) }},
		{"fractional level", "illegal-level", func(r recordIndex) { r["units"]["first"][0].Value["rank"] = 8.5 }},
		{"out of range", "illegal-level", func(r recordIndex) { r["units"]["second"][0].Value["rank"] = float64(10) }},
		{"missing level field", "illegal-level", func(r recordIndex) { delete(r["units"]["first"][0].Value, "rank") }},
		{"missing family", "rule-family", func(r recordIndex) { r["units"]["first"][0].Value["kind"] = "" }},
		{"wrong family", "purchase-family", func(r recordIndex) { r["units"]["first"][0].Value["kind"] = "other" }},
		{"skip level", "purchase-transition", func(r recordIndex) { r["units"]["base"][0].Value["buys"] = []any{map[string]any{"next": "second"}} }},
		{"cycle", "purchase-transition", func(r recordIndex) { r["units"]["second"][0].Value["buys"] = []any{map[string]any{"next": "base"}} }},
		{"missing edge", "missing-purchase", func(r recordIndex) { r["units"]["first"][0].Value["buys"] = []any{} }},
		{"missing edge list", "purchase-list", func(r recordIndex) { delete(r["units"]["first"][0].Value, "buys") }},
		{"malformed edge", "purchase-target", func(r recordIndex) { r["units"]["first"][0].Value["buys"] = []any{"second"} }},
		{"duplicate edge", "duplicate-purchase", func(r recordIndex) {
			r["units"]["base"][0].Value["buys"] = []any{map[string]any{"next": "first"}, map[string]any{"next": "first"}}
		}},
		{"branch", "linear-transition-count", func(r recordIndex) {
			r["units"]["base"][0].Value["buys"] = []any{map[string]any{"next": "first"}, map[string]any{"next": "second"}}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, s, r := linearFixture(t)
			tt.change(r)
			var report Report
			checkRules(p, s, r, &report)
			if !hasRuleCode(report, tt.code) {
				t.Fatalf("want %s, got %+v", tt.code, report.Errors)
			}
			if report.Rules[0].Errors != len(report.Errors) {
				t.Fatalf("error accounting: %+v", report.Rules[0])
			}
		})
	}
}

func TestLinearChecksDisconnectedMembersAndAllowsEmptyInput(t *testing.T) {
	p, s, r := linearFixture(t)
	r["units"]["first"][0].Value["buys"] = []any{}
	r["units"]["second"][0].Value["rank"] = -1.0
	var report Report
	checkRules(p, s, r, &report)
	if !hasRuleCode(report, "illegal-level") || report.Rules[0].RecordsChecked != 3 {
		t.Fatalf("disconnected member was not checked: %+v", report)
	}
	report = Report{}
	checkRules(p, s, recordIndex{}, &report)
	if len(report.Errors) > 0 || report.Rules[0].RecordsChecked != 0 {
		t.Fatalf("unused progression must allow no members: %+v", report)
	}
}

func TestLinearOptionalCompletenessStillValidatesPresentMembers(t *testing.T) {
	p, s, r := linearFixture(t)
	p.Rules[0].RequireCompleteStates = false
	delete(r["units"], "base")
	r["units"]["first"][0].Value["buys"] = []any{}
	var report Report
	checkRules(p, s, r, &report)
	if len(report.Errors) > 0 || report.Rules[0].RecordsChecked != 2 {
		t.Fatalf("optional completeness should check only present records: %+v", report)
	}
}

func TestLinearRejectsTargetOutsideMemberSelector(t *testing.T) {
	p, s, r := linearFixture(t)
	var schema jsonschema.Schema
	if err := json.Unmarshal([]byte(`{"type":"object","required":["kind"],"not":{"required":["temporary"]}}`), &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	s["member"] = resolved
	r["units"]["first"][0].Value["temporary"] = true
	var report Report
	checkRules(p, s, r, &report)
	if !hasRuleCode(report, "purchase-scope") || report.Rules[0].OutOfScopeRecords != 1 {
		t.Fatalf("alternative form must not fill canonical level: %+v", report)
	}
}

func TestLinearUsesTypedLiteralAliases(t *testing.T) {
	p, s, r := linearFixture(t)
	p.TypeIdentity = typeIdentity{Field: "recordType", Encoding: "literal"}
	p.References = []referenceRule{{Models: []string{"advance"}, Field: "next", Target: "units", Aliases: map[string]string{"alias": "first"}}}
	edge := r["units"]["base"][0].Value["buys"].([]any)[0].(map[string]any)
	edge["recordType"], edge["next"] = "advance", "alias"
	var report Report
	checkRules(p, s, r, &report)
	if len(report.Errors) > 0 {
		t.Fatalf("typed literal alias rejected: %+v", report.Errors)
	}
	edge["recordType"] = "other"
	report = Report{}
	checkRules(p, s, r, &report)
	if !hasRuleCode(report, "purchase-target") {
		t.Fatalf("alias must be model-specific: %+v", report.Errors)
	}
}

func TestLinearConfigurationBounds(t *testing.T) {
	for _, bounds := range []levelRange{{First: -1, Last: 2}, {First: 3, Last: 2}, {First: 0, Last: maxProgressionStates}, {First: 0, Last: int(^uint(0) >> 1)}} {
		p, _, _ := linearFixture(t)
		p.Rules[0].Levels = bounds
		if err := validateRules(p); err == nil {
			t.Fatalf("accepted invalid bounds %+v", bounds)
		}
	}
	p, _, _ := linearFixture(t)
	p.Rules[0].LevelField = ""
	if err := validateRules(p); err == nil || !strings.Contains(err.Error(), "levelField") {
		t.Fatalf("missing level field: %v", err)
	}
	for _, value := range []any{math.NaN(), math.Inf(1), "8", nil} {
		if _, err := readLevel(value, levelRange{First: 7, Last: 9}); err == nil {
			t.Fatalf("accepted invalid level %v", value)
		}
	}
}

func TestRuleSchemaSeparatesOperationArguments(t *testing.T) {
	loader, err := newProfileLoader("../../profile")
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := loader.resolve("schemas/profile/rules.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	base := `{"rules":[{"id":"linear","operation":"linearProgression","scope":"levels","familyField":"family","levelField":"rank","levels":{"first":1,"last":3},"requireCompleteStates":true%s}]}`
	for _, extra := range []string{"", `,"tiersField":"tiers"`, `,"limits":{"pathCount":1,"maxTier":3,"maxPurchasedPaths":1,"secondaryTierLimit":3,"maxPathsAboveSecondaryTier":0}`, `,"unexpected":true`} {
		var value any
		if err := json.Unmarshal([]byte(strings.Replace(base, "%s", extra, 1)), &value); err != nil {
			t.Fatal(err)
		}
		err := resolved.Validate(value)
		if (extra == "") != (err == nil) {
			t.Fatalf("extra %q: %v", extra, err)
		}
	}
}

func TestLinearCompleteFamilyCannotHideOrphanFamily(t *testing.T) {
	p, s, r := linearFixture(t)
	r["units"]["orphan"] = []record{{File: "orphan.json", Value: map[string]any{"id": "orphan", "kind": "other", "rank": float64(9), "buys": []any{}}}}
	var report Report
	checkRules(p, s, r, &report)
	if !hasRuleCode(report, "missing-root") || report.Rules[0].Roots != 1 || report.Rules[0].RecordsChecked != 4 {
		t.Fatalf("complete family hid orphan: %+v", report)
	}
}

func TestLinearSingleLevelProgression(t *testing.T) {
	p, s, r := linearFixture(t)
	p.Rules[0].Levels.Last = p.Rules[0].Levels.First
	delete(r["units"], "first")
	delete(r["units"], "second")
	r["units"]["base"][0].Value["buys"] = []any{}
	var report Report
	checkRules(p, s, r, &report)
	if len(report.Errors) > 0 || report.Rules[0].ExpectedStates != 1 {
		t.Fatalf("single level must be a valid terminal root: %+v", report)
	}
}

func TestLinearRejectsRootOutsideMemberSelector(t *testing.T) {
	p, s, r := linearFixture(t)
	var schema jsonschema.Schema
	if err := json.Unmarshal([]byte(`{"type":"object","required":["kind"],"properties":{"origin":{"const":false}}}`), &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	s["member"] = resolved
	var report Report
	checkRules(p, s, r, &report)
	if !hasRuleCode(report, "root-outside-members") {
		t.Fatalf("inconsistent selectors accepted: %+v", report.Errors)
	}
}

func TestLinearLevelIterationAtRepresentableLimit(t *testing.T) {
	p, s, r := linearFixture(t)
	last := int(^uint(0) >> 1)
	exactLast := uint64(1<<53 - 1)
	if uint64(last) > exactLast {
		last = int(exactLast)
	}
	p.Rules[0].Levels = levelRange{First: last - 2, Last: last}
	for i, id := range []string{"base", "first", "second"} {
		r["units"][id][0].Value["rank"] = float64(last - 2 + i)
	}
	if err := validateRules(p); err != nil {
		t.Fatal(err)
	}
	var report Report
	checkRules(p, s, r, &report)
	if len(report.Errors) != 0 || report.Rules[0].ExpectedStates != 3 {
		t.Fatalf("valid bounded progression near numeric limit rejected: %+v", report)
	}
}

func TestProgressionRejectsAmbiguousOrUnknownSelectorBindings(t *testing.T) {
	for _, tt := range []struct {
		name, want string
		change     func(*scope)
	}{
		{"root collision", "exactly one", func(s *scope) { s.RootSelector = "configured-root" }},
		{"member collision", "exactly one", func(s *scope) { s.MemberSelector = "configured-member" }},
		{"unknown root", "unknown root selector", func(s *scope) { s.RootSchema, s.RootSelector = "", "missing" }},
		{"unknown member", "unknown member selector", func(s *scope) { s.MemberSchema, s.MemberSelector = "", "missing" }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			p, _, _ := linearFixture(t)
			tt.change(&p.Scopes[0])
			if err := validateRules(p); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want %q, got %v", tt.want, err)
			}
		})
	}
}

func TestProgressionsUseConfiguredSourceSelectors(t *testing.T) {
	for _, operation := range []string{"purchaseProgression", "linearProgression"} {
		t.Run(operation, func(t *testing.T) {
			p, _, r := progressionFixture(t)
			if operation == "linearProgression" {
				p, _, r = linearFixture(t)
			}
			p.Selectors = map[string]sourceSelector{
				"canonical": {Fields: map[string][]any{"kind": {"unit"}}},
				"initial":   {All: []string{"canonical"}, Fields: map[string][]any{"origin": {true}}},
			}
			p.Scopes[0].RootSchema, p.Scopes[0].MemberSchema = "", ""
			p.Scopes[0].RootSelector, p.Scopes[0].MemberSelector = "initial", "canonical"
			if err := validateRules(p); err != nil {
				t.Fatal(err)
			}
			var report Report
			// No source schemas are available: every binding comes from settings.
			checkRules(p, nil, r, &report)
			if len(report.Errors) != 0 || report.Rules[0].RecordsChecked != 3 {
				t.Fatalf("configured selectors failed: %+v", report)
			}
		})
	}
}
