package atlasvalidate

import (
	"fmt"
	"math"
	"sort"
)

func validateLevelRange(levels levelRange) error {
	// Limit the absolute values too: JSON numbers must represent each level
	// exactly, and the inclusive iteration must never overflow.
	const maxExactLevel uint64 = 1<<53 - 1
	if levels.First < 0 || levels.Last < levels.First || uint64(levels.Last) > maxExactLevel {
		return fmt.Errorf("invalid level range: require 0 <= first <= last <= %d", maxExactLevel)
	}
	if levels.Last-levels.First >= maxProgressionStates {
		return fmt.Errorf("linear progression exceeds %d levels", maxProgressionStates)
	}
	return nil
}

func readLevel(value any, levels levelRange) (int, error) {
	number, ok := value.(float64)
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) || math.Trunc(number) != number || number < float64(levels.First) || number > float64(levels.Last) {
		return 0, fmt.Errorf("level must be an integer from %d to %d", levels.First, levels.Last)
	}
	return int(number), nil
}

// Linear progressions share the same collection, selector and reference
// configuration as purchase progressions, but contain one state per level.
func checkLinearRule(p profile, schemas schemaIndex, records recordIndex, rule rule, s scope, report *Report) {
	result := RuleResult{ID: rule.ID, Operation: rule.Operation, Scope: rule.Scope}
	before := len(report.Errors)
	add := func(file, pointer, code, message string) { report.addRule(rule.ID, file, pointer, code, message) }
	defer func() {
		result.Errors = len(report.Errors) - before
		report.Rules = append(report.Rules, result)
	}()
	if err := validateLevelRange(rule.Levels); err != nil {
		add("", "", "rule-configuration", err.Error())
		return
	}
	result.ExpectedStates = rule.Levels.Last - rule.Levels.First + 1
	var all []record
	for _, matches := range records[s.Collection] {
		all = append(all, matches...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].File < all[j].File })
	type linearFamily struct {
		members []record
		roots   []record
		levels  map[int]record
	}
	families := map[string]*linearFamily{}
	members := map[string]bool{}
	for _, rec := range all {
		member := p.scopeMatches(s, false, schemas, rec.Value)
		root := p.scopeMatches(s, true, schemas, rec.Value)
		if root {
			result.Roots++
		}
		if !member && !root {
			continue
		}
		result.RecordsChecked++
		report.checked(rule.ID, rec.File)
		members[rec.File] = member
		if root && !member {
			add(rec.File, "", "root-outside-members", "progression root does not match its member selector")
		}
		family, ok := rec.Value[rule.FamilyField].(string)
		if !ok || family == "" {
			add(rec.File, "/"+pointerToken(rule.FamilyField), "rule-family", "progression member requires a nonempty family identifier")
			continue
		}
		group := families[family]
		if group == nil {
			group = &linearFamily{levels: map[int]record{}}
			families[family] = group
		}
		group.members = append(group.members, rec)
		if root {
			group.roots = append(group.roots, rec)
		}
		level, err := readLevel(rec.Value[rule.LevelField], rule.Levels)
		if err != nil {
			add(rec.File, "/"+pointerToken(rule.LevelField), "illegal-level", err.Error())
			continue
		}
		if previous, exists := group.levels[level]; exists {
			add(rec.File, "/"+pointerToken(rule.LevelField), "duplicate-level", fmt.Sprintf("family %q repeats level %d from %s", family, level, previous.File))
		} else {
			group.levels[level] = rec
		}
		if root && level != rule.Levels.First {
			add(rec.File, "/"+pointerToken(rule.LevelField), "invalid-root-level", fmt.Sprintf("progression root must start at level %d", rule.Levels.First))
		}
	}
	result.OutOfScopeRecords = len(all) - result.RecordsChecked
	familyNames := make([]string, 0, len(families))
	for family := range families {
		familyNames = append(familyNames, family)
	}
	sort.Strings(familyNames)
	for _, family := range familyNames {
		group := families[family]
		if len(group.roots) == 0 && rule.RequireCompleteStates {
			add(group.members[0].File, "/"+pointerToken(rule.FamilyField), "missing-root", fmt.Sprintf("family %q has progression members but no root in scope %q", family, s.Name))
		}
		for i, root := range group.roots {
			if i == 0 {
				continue
			}
			add(root.File, "/"+pointerToken(rule.FamilyField), "duplicate-progression-root", fmt.Sprintf("family %q has another progression root in %s", family, group.roots[0].File))
		}
		if rule.RequireCompleteStates {
			for level := rule.Levels.First; ; level++ {
				if _, exists := group.levels[level]; !exists {
					add(group.members[0].File, "/"+pointerToken(rule.LevelField), "missing-level", fmt.Sprintf("family %q has no state for required level %d", family, level))
				}
				if level == rule.Levels.Last {
					break
				}
			}
		}
		for _, rec := range group.members {
			level, levelErr := readLevel(rec.Value[rule.LevelField], rule.Levels)
			edges, ok := rec.Value[s.EdgesField].([]any)
			if !ok {
				add(rec.File, "/"+pointerToken(s.EdgesField), "purchase-list", "expected an array of progression transitions")
			}
			if len(edges) > 1 {
				add(rec.File, "/"+pointerToken(s.EdgesField), "linear-transition-count", "linear progression permits at most one outgoing transition")
			}
			outgoing := map[string]bool{}
			validNext := false
			for i, edge := range edges {
				pointer := fmt.Sprintf("/%s/%d/%s", pointerToken(s.EdgesField), i, pointerToken(s.TargetField))
				object, ok := edge.(map[string]any)
				target, targetOK := object[s.TargetField].(string)
				if !ok || !targetOK || target == "" {
					add(rec.File, pointer, "purchase-target", "transition requires a nonempty target identity")
					continue
				}
				result.TransitionsChecked++
				canonical := purchaseTarget(p, s, object, target)
				if outgoing[canonical] {
					add(rec.File, pointer, "duplicate-purchase", fmt.Sprintf("transition to %q is repeated", target))
				}
				outgoing[canonical] = true
				matches := records[s.Collection][canonical]
				if len(matches) != 1 {
					add(rec.File, pointer, "purchase-target", fmt.Sprintf("transition target %q resolves to %d records", target, len(matches)))
					continue
				}
				next := matches[0]
				if next.Value[rule.FamilyField] != family {
					add(rec.File, pointer, "purchase-family", fmt.Sprintf("transition target %q is outside family %q", target, family))
					continue
				}
				if !members[next.File] {
					add(rec.File, pointer, "purchase-scope", fmt.Sprintf("transition target %q is outside the progression member selector", target))
					continue
				}
				nextLevel, nextErr := readLevel(next.Value[rule.LevelField], rule.Levels)
				if levelErr != nil || nextErr != nil {
					continue
				}
				if level == rule.Levels.Last || nextLevel != level+1 {
					add(rec.File, pointer, "purchase-transition", fmt.Sprintf("transition from level %d to %d must advance to the next configured level", level, nextLevel))
					continue
				}
				validNext = true
			}
			if rule.RequireCompleteStates && levelErr == nil && level < rule.Levels.Last && !validNext {
				add(rec.File, "/"+pointerToken(s.EdgesField), "missing-purchase", fmt.Sprintf("level %d has no transition to required level %d", level, level+1))
			}
		}
	}
}
