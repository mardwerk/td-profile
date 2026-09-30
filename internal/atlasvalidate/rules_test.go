package atlasvalidate

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

// The fixture declares its own names, one path and two purchases. It verifies
// that rule evaluation does not depend on BTD6 field names or progression limits.
func progressionFixture(t *testing.T) (profile, schemaIndex, recordIndex) {
	t.Helper()
	var schema jsonschema.Schema
	if err := json.Unmarshal([]byte(`{"type":"object","required":["origin"],"properties":{"origin":{"const":true}}}`), &schema); err != nil {
		t.Fatal(err)
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	var memberSchema jsonschema.Schema
	if err := json.Unmarshal([]byte(`{"type":"object","required":["kind"]}`), &memberSchema); err != nil {
		t.Fatal(err)
	}
	memberResolved, err := memberSchema.Resolve(nil)
	if err != nil {
		t.Fatal(err)
	}
	p := profile{
		TypeIdentity: typeIdentity{Field: "$type", Encoding: "dotnet"},
		Collections:  []collection{{Name: "units", IDField: "id"}},
		Scopes:       []scope{{Name: "purchases", Collection: "units", RootSchema: "root", MemberSchema: "member", EdgesField: "buys", TargetField: "next"}},
		Rules:        []rule{{ID: "progression", Operation: "purchaseProgression", Scope: "purchases", TiersField: "levels", FamilyField: "kind", Limits: progressionLimits{PathCount: 1, MaxTier: 2, MaxPurchasedPaths: 1, SecondaryTierLimit: 2, MaxPathsAboveSecondaryTier: 0}, RequireCompleteStates: true}},
	}
	records := recordIndex{"units": {}}
	for i, id := range []string{"base", "first", "second"} {
		edges := []any{}
		if i < 2 {
			edges = append(edges, map[string]any{"next": []string{"first", "second"}[i]})
		}
		records["units"][id] = []record{{File: id + ".json", Value: map[string]any{"id": id, "kind": "unit", "levels": []any{float64(i)}, "buys": edges, "origin": i == 0}}}
	}
	return p, schemaIndex{"root": resolved, "member": memberResolved}, records
}

func hasRuleCode(r Report, code string) bool {
	for _, err := range r.Errors {
		if err.Code == code {
			return true
		}
	}
	return false
}

func TestProgressionUsesProfileFieldsAndLimits(t *testing.T) {
	p, schemas, records := progressionFixture(t)
	if err := validateRules(p); err != nil {
		t.Fatal(err)
	}
	var report Report
	checkRules(p, schemas, records, &report)
	if len(report.Errors) > 0 {
		t.Fatalf("valid progression: %+v", report.Errors)
	}
	got := report.Rules[0]
	if got.Roots != 1 || got.RecordsChecked != 3 || got.TransitionsChecked != 2 || got.ExpectedStates != 3 || got.OutOfScopeRecords != 0 {
		t.Fatalf("coverage: %+v", got)
	}
}

func TestProgressionBrokenGraphs(t *testing.T) {
	tests := []struct {
		name, code string
		change     func(profile, recordIndex)
	}{
		{"missing target", "purchase-target", func(_ profile, r recordIndex) { delete(r["units"], "first") }},
		{"wrong family", "purchase-family", func(_ profile, r recordIndex) { r["units"]["first"][0].Value["kind"] = "other" }},
		{"skipped tier", "purchase-transition", func(_ profile, r recordIndex) {
			r["units"]["base"][0].Value["buys"] = []any{map[string]any{"next": "second"}}
		}},
		{"duplicate build", "duplicate-build", func(_ profile, r recordIndex) { r["units"]["second"][0].Value["levels"] = []any{float64(1)} }},
		{"duplicate transition", "duplicate-purchase", func(_ profile, r recordIndex) {
			r["units"]["base"][0].Value["buys"] = []any{map[string]any{"next": "first"}, map[string]any{"next": "first"}}
		}},
		{"missing edge", "missing-purchase", func(_ profile, r recordIndex) { r["units"]["first"][0].Value["buys"] = []any{} }},
		{"disconnected state", "missing-build", func(_ profile, r recordIndex) { r["units"]["first"][0].Value["buys"] = []any{} }},
		{"malformed root tiers", "illegal-build", func(_ profile, r recordIndex) { r["units"]["base"][0].Value["levels"] = []any{float64(-1)} }},
		{"nonzero root", "invalid-root-build", func(_ profile, r recordIndex) { r["units"]["base"][0].Value["levels"] = []any{float64(1)} }},
		{"missing roots", "scope-empty", func(_ profile, r recordIndex) { r["units"]["base"][0].Value["origin"] = false }},
		{"missing purchase array", "purchase-list", func(_ profile, r recordIndex) { delete(r["units"]["base"][0].Value, "buys") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, s, r := progressionFixture(t)
			tt.change(p, r)
			var report Report
			checkRules(p, s, r, &report)
			if !hasRuleCode(report, tt.code) {
				t.Fatalf("want %s, got %+v", tt.code, report.Errors)
			}
			if report.Rules[0].Errors != len(report.Errors) {
				t.Fatalf("wrong error coverage: %+v", report.Rules[0])
			}
		})
	}
}

func TestProgressionTemporaryRecordsStayOutsideScope(t *testing.T) {
	p, s, r := progressionFixture(t)
	r["units"]["temporary"] = []record{{File: "temporary.json", Value: map[string]any{"id": "temporary", "kind": "unit", "levels": []any{float64(0)}, "buys": []any{}}}}
	var report Report
	checkRules(p, s, r, &report)
	if len(report.Errors) > 0 || report.Rules[0].OutOfScopeRecords != 1 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestProgressionCompletenessOptional(t *testing.T) {
	p, s, r := progressionFixture(t)
	p.Rules[0].RequireCompleteStates = false
	r["units"]["base"][0].Value["buys"] = []any{}
	var report Report
	checkRules(p, s, r, &report)
	if len(report.Errors) > 0 || report.Rules[0].RecordsChecked != 1 || report.Rules[0].OutOfScopeRecords != 2 {
		t.Fatalf("unexpected report: %+v", report)
	}
}

func TestOrdinaryProgressionStateSpace(t *testing.T) {
	limits := progressionLimits{PathCount: 3, MaxTier: 5, MaxPurchasedPaths: 2, SecondaryTierLimit: 2, MaxPathsAboveSecondaryTier: 1}
	states, err := progressionStates(limits)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 64 {
		t.Fatalf("want 64 legal states, got %d", len(states))
	}
	transitions := 0
	for _, tiers := range states {
		for path := range tiers {
			next := append([]int(nil), tiers...)
			next[path]++
			if _, ok := states[tierKey(next)]; ok {
				transitions++
			}
		}
	}
	if transitions != 111 {
		t.Fatalf("want 111 transitions, got %d", transitions)
	}
	for _, tiers := range [][]any{{float64(1), float64(1), float64(1)}, {float64(3), float64(3), float64(0)}, {float64(1.5), float64(0), float64(0)}} {
		if _, err := readTiers(tiers, limits); err == nil {
			t.Fatalf("accepted illegal tiers %v", tiers)
		}
	}
}

func TestRulesRejectInvalidConfiguration(t *testing.T) {
	tests := []struct {
		name, want string
		change     func(*profile)
	}{
		{"unknown operation", "unknown operation", func(p *profile) { p.Rules[0].Operation = "invented" }},
		{"unknown scope", "unknown scope", func(p *profile) { p.Rules[0].Scope = "missing" }},
		{"duplicate ID", "duplicate rule", func(p *profile) { p.Rules = append(p.Rules, p.Rules[0]) }},
		{"duplicate scope", "duplicate scope", func(p *profile) { p.Scopes = append(p.Scopes, p.Scopes[0]) }},
		{"unknown collection", "unknown collection", func(p *profile) { p.Scopes[0].Collection = "missing" }},
		{"missing field", "tiersField", func(p *profile) { p.Rules[0].TiersField = "" }},
		{"missing edge field", "edgesField", func(p *profile) { p.Scopes[0].EdgesField = "" }},
		{"invalid limits", "invalid progression limits", func(p *profile) { p.Rules[0].Limits.MaxPurchasedPaths = 2 }},
		{"unbounded state space", "exceeds", func(p *profile) { p.Rules[0].Limits.MaxTier = 1000000; p.Rules[0].Limits.SecondaryTierLimit = 1000000 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, _, _ := progressionFixture(t)
			tt.change(&p)
			err := validateRules(p)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want %q, got %v", tt.want, err)
			}
		})
	}
}

func TestMissingPurchaseFailsEvenWhenAllStatesRemainReachable(t *testing.T) {
	p, s, r := progressionFixture(t)
	p.Rules[0].Limits = progressionLimits{PathCount: 2, MaxTier: 1, MaxPurchasedPaths: 2, SecondaryTierLimit: 1}
	r["units"] = map[string][]record{}
	fixtures := []struct {
		id      string
		tiers   []any
		targets []string
	}{
		{"base", []any{float64(0), float64(0)}, []string{"left", "right"}},
		{"left", []any{float64(1), float64(0)}, []string{"both"}},
		{"right", []any{float64(0), float64(1)}, nil},
		{"both", []any{float64(1), float64(1)}, nil},
	}
	for _, fixture := range fixtures {
		edges := []any{}
		for _, target := range fixture.targets {
			edges = append(edges, map[string]any{"next": target})
		}
		r["units"][fixture.id] = []record{{File: fixture.id + ".json", Value: map[string]any{"id": fixture.id, "kind": "unit", "levels": fixture.tiers, "buys": edges, "origin": fixture.id == "base"}}}
	}
	var report Report
	checkRules(p, s, r, &report)
	if len(report.Errors) != 1 || report.Errors[0].Code != "missing-purchase" || report.Errors[0].File != "right.json" {
		t.Fatalf("want only missing right-to-both purchase, got %+v", report.Errors)
	}
	if report.Rules[0].RecordsChecked != 4 {
		t.Fatalf("all four states must remain reachable: %+v", report.Rules[0])
	}
}

func TestReachableIllegalCrosspaths(t *testing.T) {
	for _, tiers := range [][]any{{float64(1), float64(1), float64(1)}, {float64(3), float64(3), float64(0)}} {
		p, s, r := progressionFixture(t)
		p.Rules[0].RequireCompleteStates = false
		p.Rules[0].Limits = progressionLimits{PathCount: 3, MaxTier: 5, MaxPurchasedPaths: 2, SecondaryTierLimit: 2, MaxPathsAboveSecondaryTier: 1}
		r["units"]["base"][0].Value["levels"] = []any{float64(0), float64(0), float64(0)}
		r["units"]["first"][0].Value["levels"] = tiers
		r["units"]["first"][0].Value["buys"] = []any{}
		delete(r["units"], "second")
		var report Report
		checkRules(p, s, r, &report)
		if !hasRuleCode(report, "illegal-build") {
			t.Fatalf("accepted reachable tiers %v: %+v", tiers, report)
		}
	}
}

func TestProgressionRequiresRootForEveryMemberFamily(t *testing.T) {
	for _, mutation := range []string{"deleted base", "unrecognized base", "illegal orphan"} {
		t.Run(mutation, func(t *testing.T) {
			p, s, r := progressionFixture(t)
			// A complete valid family must not hide an incomplete second family.
			r["units"]["other-base"] = []record{{File: "other-base.json", Value: map[string]any{"id": "other-base", "kind": "other", "levels": []any{float64(0)}, "buys": []any{map[string]any{"next": "other-first"}}, "origin": true}}}
			r["units"]["other-first"] = []record{{File: "other-first.json", Value: map[string]any{"id": "other-first", "kind": "other", "levels": []any{float64(1)}, "buys": []any{map[string]any{"next": "other-second"}}}}}
			r["units"]["other-second"] = []record{{File: "other-second.json", Value: map[string]any{"id": "other-second", "kind": "other", "levels": []any{float64(2)}, "buys": []any{}}}}
			switch mutation {
			case "deleted base":
				delete(r["units"], "other-base")
			case "unrecognized base":
				r["units"]["other-base"][0].Value["origin"] = false
			case "illegal orphan":
				delete(r["units"], "other-base")
				r["units"]["other-first"][0].Value["levels"] = []any{float64(100)}
			}
			var report Report
			checkRules(p, s, r, &report)
			if len(report.Errors) != 1 || report.Errors[0].Code != "missing-root" || report.Errors[0].Pointer != "/kind" {
				t.Fatalf("want one missing-root error for other family, got %+v", report.Errors)
			}
			if report.Rules[0].Roots != 1 || report.Rules[0].RecordsChecked != 3 {
				t.Fatalf("existing family should be checked: %+v", report.Rules[0])
			}
		})
	}
}

func TestCompleteProgressionAllowsEmptyCollection(t *testing.T) {
	p, s, r := progressionFixture(t)
	delete(r, "units")
	var report Report
	checkRules(p, s, r, &report)
	if len(report.Errors) != 0 {
		t.Fatalf("empty unused collection must be valid: %+v", report.Errors)
	}
	p.Rules[0].RequireCompleteStates = false
	report = Report{}
	checkRules(p, s, r, &report)
	if len(report.Errors) != 0 {
		t.Fatalf("optional coverage should allow empty input: %+v", report.Errors)
	}
}

func TestScopeRequiresMemberSelector(t *testing.T) {
	p, _, _ := progressionFixture(t)
	p.Scopes[0].MemberSchema = ""
	if err := validateRules(p); err == nil || !strings.Contains(err.Error(), "memberSchema") {
		t.Fatalf("want memberSchema configuration error, got %v", err)
	}
}

func TestPurchaseTargetsUseMatchingReferenceAliases(t *testing.T) {
	tests := []struct {
		name      string
		change    func(*profile, map[string]any)
		wantError bool
	}{
		{"matching typed alias", func(_ *profile, _ map[string]any) {}, false},
		{"different model", func(p *profile, _ map[string]any) { p.References[0].Models = []string{"OtherModel"} }, true},
		{"different field", func(p *profile, _ map[string]any) { p.References[0].Field = "other" }, true},
		{"different collection", func(p *profile, _ map[string]any) { p.References[0].Target = "other" }, true},
		{"untyped edge", func(_ *profile, edge map[string]any) { delete(edge, "$type") }, true},
		{"single hop only", func(p *profile, _ map[string]any) {
			p.References[0].Aliases = map[string]string{"alternate": "intermediate", "intermediate": "first"}
		}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, s, r := progressionFixture(t)
			p.References = []referenceRule{{Models: []string{"PurchaseModel"}, Field: "next", Target: "units", Aliases: map[string]string{"alternate": "first"}}}
			edge := r["units"]["base"][0].Value["buys"].([]any)[0].(map[string]any)
			edge["$type"] = "Example.PurchaseModel, Assembly"
			edge["next"] = "alternate"
			tt.change(&p, edge)
			var report Report
			checkRules(p, s, r, &report)
			if !tt.wantError {
				if len(report.Errors) > 0 {
					t.Fatalf("matching alias rejected: %+v", report.Errors)
				}
				if report.Rules[0].RecordsChecked != 3 || report.Rules[0].TransitionsChecked != 2 {
					t.Fatalf("alias must preserve coverage: %+v", report.Rules[0])
				}
				return
			}
			if !hasRuleCode(report, "purchase-target") {
				t.Fatalf("unrelated or chained alias accepted: %+v", report)
			}
			for _, diagnostic := range report.Errors {
				if diagnostic.Code == "purchase-target" && !strings.Contains(diagnostic.Message, `"alternate"`) {
					t.Fatalf("diagnostic must retain raw alias: %+v", diagnostic)
				}
			}
		})
	}
}

func TestPurchaseProgressionRequiresMembersForRootsAndTargets(t *testing.T) {
	for _, tt := range []struct{ recordID, code string }{
		{"base", "root-outside-members"},
		{"first", "purchase-scope"},
	} {
		t.Run(tt.recordID, func(t *testing.T) {
			p, _, records := progressionFixture(t)
			p.Selectors = map[string]sourceSelector{
				"root":   {Fields: map[string][]any{"origin": {true}}},
				"member": {Fields: map[string][]any{"summoned": {false}}},
			}
			p.Scopes[0].RootSchema, p.Scopes[0].MemberSchema = "", ""
			p.Scopes[0].RootSelector, p.Scopes[0].MemberSelector = "root", "member"
			for _, matches := range records["units"] {
				matches[0].Value["summoned"] = false
			}
			records["units"][tt.recordID][0].Value["summoned"] = true
			var report Report
			checkRules(p, nil, records, &report)
			if !hasRuleCode(report, tt.code) {
				t.Fatalf("want %s for nonmember %s, got %+v", tt.code, tt.recordID, report.Errors)
			}
			if tt.recordID == "first" {
				if report.ruleRecords[p.Rules[0].ID]["first.json"] || report.Rules[0].RecordsChecked != 1 {
					t.Fatalf("nonmember target must not be traversed or counted as a progression state: %+v", report)
				}
				if !hasRuleCode(report, "missing-build") {
					t.Fatalf("nonmember target must not satisfy required state completeness: %+v", report.Errors)
				}
			}
		})
	}
}
