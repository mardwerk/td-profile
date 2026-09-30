package atlasvalidate

import (
	"reflect"
	"testing"
)

func projectionProfile() profile {
	return profile{
		TypeIdentity: typeIdentity{Field: "class", Encoding: "literal"},
		MechanicByModel: map[string]mechanic{
			"Unit":   {ID: "tower", Fields: map[string]string{"id": "name", "attacks": "weapons", "price": "cost", "description": "text"}},
			"Weapon": {ID: "attack", Fields: map[string]string{"damage": "power"}},
		},
	}
}

func TestProjectionBuildsNestedViewWithoutChangingSource(t *testing.T) {
	p := projectionProfile()
	source := map[string]any{
		"class": "Unit", "name": "Scout", "cost": nil,
		"weapons": []any{map[string]any{"class": "Weapon", "power": float64(5)}},
		"extra":   map[string]any{"enabled": true},
	}
	view := p.project(source).(map[string]any)
	if view["kind"] != "tower" || view["id"] != "Scout" {
		t.Fatalf("projected identity: %#v", view)
	}
	if price, present := view["price"]; !present || price != nil {
		t.Fatalf("explicit null lost: %#v", view)
	}
	if _, present := view["description"]; present {
		t.Fatal("absent source field became a present field")
	}
	attack := view["attacks"].([]any)[0].(map[string]any)
	if attack["kind"] != "attack" || attack["damage"] != float64(5) {
		t.Fatalf("nested model: %#v", attack)
	}
	attack["power"] = float64(99)
	view["extra"].(map[string]any)["enabled"] = false
	if _, changed := source["kind"]; changed {
		t.Fatal("projection changed source identity")
	}
	if source["weapons"].([]any)[0].(map[string]any)["power"] != float64(5) || source["extra"].(map[string]any)["enabled"] != true {
		t.Fatal("projection retained mutable source containers")
	}
}

func TestProjectionLiteralKindAndDiscriminatorErrors(t *testing.T) {
	p := projectionProfile()
	p.TypeIdentity.Field = "kind"
	p.MechanicByModel["Unit"] = mechanic{ID: "tower", Fields: map[string]string{"sourceKind": "kind", "id": "name"}}
	source := map[string]any{"kind": "Unit", "name": "Scout"}
	view := p.project(source).(map[string]any)
	if view["kind"] != "tower" || view["sourceKind"] != "Unit" || source["kind"] != "Unit" {
		t.Fatalf("literal discriminator overwritten before binding: %#v", view)
	}
	for _, discriminator := range []any{nil, float64(3), false} {
		view := p.project(map[string]any{"kind": discriminator}).(map[string]any)
		if !reflect.DeepEqual(view["kind"], discriminator) {
			t.Fatalf("discriminator %v became %v", discriminator, view["kind"])
		}
	}
	invalidSource := map[string]any{"kind": map[string]any{"bad": true}}
	invalidView := p.project(invalidSource).(map[string]any)
	invalidView["kind"].(map[string]any)["bad"] = false
	if invalidSource["kind"].(map[string]any)["bad"] != true {
		t.Fatal("invalid discriminator retained a mutable source container")
	}
	p.TypeIdentity = typeIdentity{Field: "runtimeType", Encoding: "dotnet"}
	view = p.project(map[string]any{"runtimeType": "Example.Unit, Game"}).(map[string]any)
	if view["kind"] != "tower" {
		t.Fatalf("dotnet discriminator: %#v", view)
	}
}

func TestProjectionUnknownKindCannotImpersonateCanonicalMechanic(t *testing.T) {
	p := projectionProfile()
	p.MechanicByModel["Collision"] = mechanic{ID: "unbound:tower"}
	p.MechanicByModel["AnotherCollision"] = mechanic{ID: "unbound:unbound:tower"}
	view := p.project(map[string]any{"class": "tower"}).(map[string]any)
	for _, binding := range p.MechanicByModel {
		if view["kind"] == binding.ID {
			t.Fatalf("unknown source kind impersonates mechanic %q", binding.ID)
		}
	}
	if _, ok := view["kind"].(string); !ok {
		t.Fatal("unknown kind no longer fits a generic model")
	}
}

func TestProjectionNestedModelNeedsSourceDiscriminator(t *testing.T) {
	p := projectionProfile()
	source := map[string]any{"class": "Unit", "name": "Scout", "weapons": []any{map[string]any{"kind": "attack", "damage": float64(5)}}}
	view := p.project(source).(map[string]any)
	if _, exists := view["attacks"].([]any)[0].(map[string]any)["kind"]; exists {
		t.Fatal("nested raw generic kind became a model discriminator")
	}
	if source["weapons"].([]any)[0].(map[string]any)["kind"] != "attack" {
		t.Fatal("raw nested kind changed")
	}
	table := map[string]any{"kind": "Label", "id": "Identity"}
	if projected := p.projectRecord(table, collection{IDField: "@keys"}, "strings.json"); !reflect.DeepEqual(projected, table) {
		t.Fatalf("untyped table changed: %#v", projected)
	}
	p.TypeIdentity.Field = "kind"
	if projected := p.projectRecord(table, collection{IDField: "@keys"}, "strings.json"); !reflect.DeepEqual(projected, table) {
		t.Fatalf("literal-discriminator table changed: %#v", projected)
	}
}

func TestProjectionAliasesReadOriginalFields(t *testing.T) {
	p := projectionProfile()
	p.MechanicByModel["Unit"] = mechanic{ID: "tower", Fields: map[string]string{"first": "second", "second": "first"}}
	view := p.project(map[string]any{"class": "Unit", "first": "one", "second": "two"}).(map[string]any)
	if view["first"] != "two" || view["second"] != "one" {
		t.Fatalf("aliases changed each other's input: %#v", view)
	}
}

func TestProjectionMissingAliasCannotUseCoincidentalGenericField(t *testing.T) {
	p := projectionProfile()
	source := map[string]any{"class": "Unit", "id": "Misleading", "price": float64(2), "code": "Fallback"}
	view := p.projectRecord(source, collection{IDField: "code"}, "Units/Scout.json").(map[string]any)
	for _, field := range []string{"id", "price"} {
		if _, exists := view[field]; exists {
			t.Fatalf("missing bound source retained generic %q: %#v", field, view)
		}
	}
	if source["id"] != "Misleading" || source["price"] != float64(2) {
		t.Fatal("alias deletion mutated raw source")
	}
}

func TestProjectionKindAlwaysUsesMechanicIdentity(t *testing.T) {
	p := projectionProfile()
	for _, sourceField := range []string{"other", "absent"} {
		p.MechanicByModel["Unit"] = mechanic{ID: "tower", Fields: map[string]string{"kind": sourceField}}
		view := p.project(map[string]any{"class": "Unit", "other": "Wrong"}).(map[string]any)
		if view["kind"] != "tower" {
			t.Fatalf("kind alias replaced mechanic identity: %#v", view)
		}
	}
}

func TestProjectionRecordIdentityFallback(t *testing.T) {
	p := projectionProfile()
	p.MechanicByModel["UnboundIdentity"] = mechanic{ID: "tower"}
	for _, test := range []struct {
		field string
		want  string
	}{
		{"code", "Scout"}, {"@stem", "Scout-1"}, {"@parent", "Scout"}, {"@path", "Scout/Scout-1.json"},
	} {
		t.Run(test.field, func(t *testing.T) {
			view := p.projectRecord(map[string]any{"class": "UnboundIdentity", "code": "Scout"}, collection{Path: "Units", IDField: test.field}, "Units/Scout/Scout-1.json").(map[string]any)
			if view["id"] != test.want {
				t.Fatalf("fallback identity: %#v", view)
			}
		})
	}
	for _, source := range []map[string]any{
		{"class": "Unit", "name": "Bound", "code": "Fallback"},
		{"class": "Unit", "name": nil, "code": "Fallback"},
	} {
		view := p.projectRecord(source, collection{IDField: "code"}, "Units/Scout.json").(map[string]any)
		if view["id"] != source["name"] {
			t.Fatalf("bound identity replaced: %#v", view)
		}
	}
	for _, test := range []struct {
		source map[string]any
		field  string
	}{
		{map[string]any{"hello": "Hello"}, "@stem"},
		{map[string]any{"class": "Unknown"}, "@keys"},
	} {
		view := p.projectRecord(test.source, collection{IDField: test.field}, "Strings.json").(map[string]any)
		if _, present := view["id"]; present {
			t.Fatalf("synthetic identity for untyped or multi-identity record: %#v", view)
		}
		if _, typed := test.source["class"]; !typed && !reflect.DeepEqual(view, test.source) {
			t.Fatalf("string table content changed: %#v", view)
		}
	}
}
