package atlasvalidate

import (
	"fmt"
	"sort"
)

type classificationsDocument struct{ Groups []classificationGroup }
type classificationGroup struct {
	Collection string
	Types      []recordType
}
type recordType struct {
	ID, SelectorSchema, Selector, Rule, Membership, Validation, Reason string
}
type RecordTypeResult struct {
	Collection string `json:"collection"`
	Type       string `json:"type"`
	Validation string `json:"validation"`
	Rule       string `json:"rule,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Records    int    `json:"records"`
}

func (r *Report) checked(ruleID, file string) {
	if r.ruleRecords == nil {
		r.ruleRecords = map[string]map[string]bool{}
	}
	if r.ruleRecords[ruleID] == nil {
		r.ruleRecords[ruleID] = map[string]bool{}
	}
	r.ruleRecords[ruleID][file] = true
}

func validateClassifications(p profile) error {
	collections := map[string]bool{}
	for _, c := range p.Collections {
		collections[c.Name] = true
	}
	rules := map[string]string{}
	scopes := map[string]string{}
	for _, s := range p.Scopes {
		scopes[s.Name] = s.Collection
	}
	for _, r := range p.Rules {
		rules[r.ID] = scopes[r.Scope]
	}
	seen := map[string]bool{}
	for _, group := range p.Classifications.Groups {
		if !collections[group.Collection] {
			return fmt.Errorf("unknown classification collection %q", group.Collection)
		}
		if seen[group.Collection] {
			return fmt.Errorf("duplicate classification group %q", group.Collection)
		}
		seen[group.Collection] = true
		types := map[string]bool{}
		for _, kind := range group.Types {
			if types[kind.ID] {
				return fmt.Errorf("duplicate record type %q", kind.ID)
			}
			types[kind.ID] = true
			if kind.Selector != "" {
				if _, ok := p.Selectors[kind.Selector]; !ok {
					return fmt.Errorf("unknown record type selector %q", kind.Selector)
				}
			}
			if kind.Rule != "" && rules[kind.Rule] != group.Collection {
				return fmt.Errorf("record type %q requires a rule in collection %q", kind.ID, group.Collection)
			}
		}
	}
	return nil
}

func checkClassifications(p profile, schemas schemaIndex, records recordIndex, r *Report) {
	for _, group := range p.Classifications.Groups {
		seen := map[string]bool{}
		var all []record
		for _, entries := range records[group.Collection] {
			for _, rec := range entries {
				if !seen[rec.File] {
					all = append(all, rec)
					seen[rec.File] = true
				}
			}
		}
		sort.Slice(all, func(i, j int) bool { return all[i].File < all[j].File })
		results := make([]RecordTypeResult, len(group.Types))
		for i, kind := range group.Types {
			results[i] = RecordTypeResult{Collection: group.Collection, Type: kind.ID, Validation: kind.Validation, Rule: kind.Rule, Reason: kind.Reason}
		}
		for _, rec := range all {
			var matches []int
			for i, kind := range group.Types {
				selected := false
				if kind.Selector != "" {
					selected = p.matchesSelector(kind.Selector, rec.Value)
				} else {
					selected = schemas[kind.SelectorSchema].Validate(rec.Value) == nil
				}
				if !selected {
					continue
				}
				if kind.Rule != "" {
					checked := r.ruleRecords[kind.Rule][rec.File]
					if (kind.Membership == "checked") != checked {
						continue
					}
				}
				matches = append(matches, i)
			}
			switch len(matches) {
			case 0:
				r.add(rec.File, "", "unknown_record_type", fmt.Sprintf("%s record has no declared type", group.Collection))
			case 1:
				results[matches[0]].Records++
			default:
				var names []string
				for _, index := range matches {
					names = append(names, group.Types[index].ID)
				}
				r.add(rec.File, "", "ambiguous_record_type", fmt.Sprintf("record matches several types: %v", names))
			}
		}
		r.RecordTypes = append(r.RecordTypes, results...)
	}
}
