package atlasvalidate

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// These are evaluation limits, not game rules. Refuse profiles whose complete
// state space cannot be checked within a predictable memory budget.
const maxProgressionStates = 100000
const maxProgressionPaths = 64

func validateRules(p profile) error {
	collections := map[string]collection{}
	for _, c := range p.Collections {
		collections[c.Name] = c
	}
	scopes := map[string]scope{}
	for _, s := range p.Scopes {
		if s.Name == "" {
			return fmt.Errorf("scope name must not be empty")
		}
		if _, exists := scopes[s.Name]; exists {
			return fmt.Errorf("duplicate scope %q", s.Name)
		}
		c, exists := collections[s.Collection]
		if !exists {
			return fmt.Errorf("scope %q names unknown collection %q", s.Name, s.Collection)
		}
		if strings.HasPrefix(c.IDField, "@") {
			return fmt.Errorf("scope %q requires a record identity field", s.Name)
		}
		if s.EdgesField == "" || s.TargetField == "" {
			return fmt.Errorf("scope %q requires edgesField and targetField", s.Name)
		}
		for _, binding := range []struct{ label, schema, selector string }{
			{"root", s.RootSchema, s.RootSelector},
			{"member", s.MemberSchema, s.MemberSelector},
		} {
			if (binding.schema == "") == (binding.selector == "") {
				return fmt.Errorf("scope %q requires exactly one of %sSchema and %sSelector", s.Name, binding.label, binding.label)
			}
			if binding.selector != "" {
				if _, exists := p.Selectors[binding.selector]; !exists {
					return fmt.Errorf("scope %q names unknown %s selector %q", s.Name, binding.label, binding.selector)
				}
			}
		}
		scopes[s.Name] = s
	}
	seen := map[string]bool{}
	for _, rule := range p.Rules {
		if rule.ID == "" || seen[rule.ID] {
			return fmt.Errorf("empty or duplicate rule ID %q", rule.ID)
		}
		seen[rule.ID] = true
		if rule.Operation != "purchaseProgression" && rule.Operation != "linearProgression" {
			return fmt.Errorf("rule %q has unknown operation %q", rule.ID, rule.Operation)
		}
		if _, exists := scopes[rule.Scope]; !exists {
			return fmt.Errorf("rule %q names unknown scope %q", rule.ID, rule.Scope)
		}
		if rule.Operation == "linearProgression" {
			if rule.LevelField == "" || rule.FamilyField == "" {
				return fmt.Errorf("rule %q requires levelField and familyField", rule.ID)
			}
			if err := validateLevelRange(rule.Levels); err != nil {
				return fmt.Errorf("rule %q: %w", rule.ID, err)
			}
			continue
		}
		if rule.TiersField == "" || rule.FamilyField == "" {
			return fmt.Errorf("rule %q requires tiersField and familyField", rule.ID)
		}
		l := rule.Limits
		if l.PathCount < 1 || l.PathCount > maxProgressionPaths || l.MaxTier < 0 || l.MaxPurchasedPaths < 0 || l.MaxPurchasedPaths > l.PathCount || l.SecondaryTierLimit < 0 || l.SecondaryTierLimit > l.MaxTier || l.MaxPathsAboveSecondaryTier < 0 || l.MaxPathsAboveSecondaryTier > l.MaxPurchasedPaths {
			return fmt.Errorf("rule %q has invalid progression limits (at most %d paths supported)", rule.ID, maxProgressionPaths)
		}
		if _, err := progressionStates(l); err != nil {
			return fmt.Errorf("rule %q: %w", rule.ID, err)
		}
	}
	return nil
}

func progressionStates(l progressionLimits) (map[string][]int, error) {
	states := map[string][]int{}
	tiers := make([]int, l.PathCount)
	var visit func(int, int, int) error
	visit = func(path, purchased, high int) error {
		if path == len(tiers) {
			if len(states) == maxProgressionStates {
				return fmt.Errorf("progression exceeds %d legal states", maxProgressionStates)
			}
			states[tierKey(tiers)] = append([]int(nil), tiers...)
			return nil
		}
		limit := l.MaxTier
		if purchased == l.MaxPurchasedPaths {
			limit = 0
		}
		if high == l.MaxPathsAboveSecondaryTier && limit > l.SecondaryTierLimit {
			limit = l.SecondaryTierLimit
		}
		for tier := 0; ; tier++ {
			tiers[path] = tier
			nextPurchased, nextHigh := purchased, high
			if tier > 0 {
				nextPurchased++
			}
			if tier > l.SecondaryTierLimit {
				nextHigh++
			}
			if err := visit(path+1, nextPurchased, nextHigh); err != nil {
				return err
			}
			if tier == limit {
				break
			}
		}
		return nil
	}
	err := visit(0, 0, 0)
	return states, err
}

func tierKey(tiers []int) string {
	values := make([]string, len(tiers))
	for i, tier := range tiers {
		values[i] = strconv.Itoa(tier)
	}
	return strings.Join(values, "-")
}

func readTiers(value any, l progressionLimits) ([]int, error) {
	values, ok := value.([]any)
	if !ok || len(values) != l.PathCount {
		return nil, fmt.Errorf("expected %d integer tiers", l.PathCount)
	}
	tiers := make([]int, len(values))
	purchased, high := 0, 0
	for i, value := range values {
		number, ok := value.(float64)
		if !ok || math.IsNaN(number) || math.IsInf(number, 0) || number < 0 || number > float64(l.MaxTier) || math.Trunc(number) != number {
			return nil, fmt.Errorf("tier %d must be an integer from 0 to %d", i, l.MaxTier)
		}
		tiers[i] = int(number)
		if tiers[i] > 0 {
			purchased++
		}
		if tiers[i] > l.SecondaryTierLimit {
			high++
		}
	}
	if purchased > l.MaxPurchasedPaths {
		return nil, fmt.Errorf("build %s purchases %d paths; at most %d are allowed", tierKey(tiers), purchased, l.MaxPurchasedPaths)
	}
	if high > l.MaxPathsAboveSecondaryTier {
		return nil, fmt.Errorf("build %s has %d paths above tier %d; at most %d are allowed", tierKey(tiers), high, l.SecondaryTierLimit, l.MaxPathsAboveSecondaryTier)
	}
	return tiers, nil
}

func onePurchase(from, to []int) bool {
	changed := 0
	for i := range from {
		if from[i] == to[i] {
			continue
		}
		if to[i]-from[i] != 1 {
			return false
		}
		changed++
	}
	return changed == 1
}

// Apply the same typed, single-step aliases as reference validation.
func purchaseTarget(p profile, s scope, edge map[string]any, target string) string {
	rawType, _ := edge[p.TypeIdentity.Field].(string)
	if rawType == "" {
		return target
	}
	kind := p.modelKind(rawType)
	for _, reference := range p.References {
		if reference.Field != s.TargetField || reference.Target != s.Collection {
			continue
		}
		for _, model := range reference.Models {
			if model == kind {
				if canonical, exists := reference.Aliases[target]; exists {
					return canonical
				}
			}
		}
	}
	return target
}

func checkRules(p profile, schemas schemaIndex, records recordIndex, report *Report) {
	scopes := map[string]scope{}
	for _, s := range p.Scopes {
		scopes[s.Name] = s
	}
	for _, rule := range p.Rules {
		s := scopes[rule.Scope]
		if rule.Operation == "linearProgression" {
			checkLinearRule(p, schemas, records, rule, s, report)
			continue
		}
		result := RuleResult{ID: rule.ID, Operation: rule.Operation, Scope: rule.Scope}
		before := len(report.Errors)
		add := func(file, pointer, code, message string) { report.addRule(rule.ID, file, pointer, code, message) }
		expected, err := progressionStates(rule.Limits)
		if err != nil {
			add("", "", "rule-configuration", err.Error())
			result.Errors = len(report.Errors) - before
			report.Rules = append(report.Rules, result)
			continue
		}
		result.ExpectedStates = len(expected)
		var all, roots []record
		for _, matches := range records[s.Collection] {
			all = append(all, matches...)
		}
		sort.Slice(all, func(i, j int) bool { return all[i].File < all[j].File })
		members := 0
		for _, rec := range all {
			if p.scopeMatches(s, false, schemas, rec.Value) {
				members++
			}
			if p.scopeMatches(s, true, schemas, rec.Value) {
				roots = append(roots, rec)
			}
		}
		result.Roots = len(roots)
		if len(roots) == 0 && members > 0 && rule.RequireCompleteStates {
			add("", "", "scope-empty", fmt.Sprintf("scope %q matched no roots in collection %q", s.Name, s.Collection))
		}
		visited := map[string]bool{}
		rootFamilies := map[string]string{}
		for _, root := range roots {
			if !p.scopeMatches(s, false, schemas, root.Value) {
				add(root.File, "", "root-outside-members", "progression root does not match its member selector")
			}
			family, ok := root.Value[rule.FamilyField].(string)
			if !ok || family == "" {
				add(root.File, "/"+pointerToken(rule.FamilyField), "rule-family", "progression root requires a nonempty family identifier")
				continue
			}
			if previous, exists := rootFamilies[family]; exists {
				add(root.File, "/"+pointerToken(rule.FamilyField), "duplicate-progression-root", fmt.Sprintf("family %q has another progression root in %s", family, previous))
			}
			rootFamilies[family] = root.File
			seen := map[string]bool{}
			builds := map[string]record{}
			queue := []record{root}
			for len(queue) > 0 {
				rec := queue[0]
				queue = queue[1:]
				if seen[rec.File] {
					continue
				}
				seen[rec.File] = true
				if !visited[rec.File] {
					result.RecordsChecked++
					report.checked(rule.ID, rec.File)
					visited[rec.File] = true
				}
				tiers, tierErr := readTiers(rec.Value[rule.TiersField], rule.Limits)
				if tierErr != nil {
					add(rec.File, "/"+pointerToken(rule.TiersField), "illegal-build", tierErr.Error())
				} else {
					key := tierKey(tiers)
					if previous, exists := builds[key]; exists {
						add(rec.File, "/"+pointerToken(rule.TiersField), "duplicate-build", fmt.Sprintf("family %q repeats build %s from %s", family, key, previous.File))
					} else {
						builds[key] = rec
					}
					if rec.File == root.File {
						for _, tier := range tiers {
							if tier != 0 {
								add(rec.File, "/"+pointerToken(rule.TiersField), "invalid-root-build", "progression root must have no purchases")
								break
							}
						}
					}
				}
				edges, ok := rec.Value[s.EdgesField].([]any)
				if !ok {
					add(rec.File, "/"+pointerToken(s.EdgesField), "purchase-list", "expected an array of purchase transitions")
				}
				outgoing := map[string]bool{}
				for i, edge := range edges {
					pointer := fmt.Sprintf("/%s/%d/%s", pointerToken(s.EdgesField), i, pointerToken(s.TargetField))
					object, ok := edge.(map[string]any)
					target, targetOK := object[s.TargetField].(string)
					if !ok || !targetOK || target == "" {
						add(rec.File, pointer, "purchase-target", "purchase requires a nonempty target identity")
						continue
					}
					result.TransitionsChecked++
					matches := records[s.Collection][purchaseTarget(p, s, object, target)]
					if len(matches) != 1 {
						add(rec.File, pointer, "purchase-target", fmt.Sprintf("purchase target %q resolves to %d records", target, len(matches)))
						continue
					}
					next := matches[0]
					if next.Value[rule.FamilyField] != family {
						add(rec.File, pointer, "purchase-family", fmt.Sprintf("purchase target %q is outside family %q", target, family))
						continue
					}
					if !p.scopeMatches(s, false, schemas, next.Value) {
						add(rec.File, pointer, "purchase-scope", fmt.Sprintf("purchase target %q is outside the progression member selector", target))
						continue
					}
					queue = append(queue, next)
					nextTiers, nextErr := readTiers(next.Value[rule.TiersField], rule.Limits)
					if tierErr != nil || nextErr != nil {
						continue
					}
					if !onePurchase(tiers, nextTiers) {
						add(rec.File, pointer, "purchase-transition", fmt.Sprintf("purchase %s to %s must increment exactly one path by one", tierKey(tiers), tierKey(nextTiers)))
						continue
					}
					key := tierKey(nextTiers)
					if outgoing[key] {
						add(rec.File, pointer, "duplicate-purchase", fmt.Sprintf("purchase transition to build %s is repeated", key))
					}
					outgoing[key] = true
				}
				if rule.RequireCompleteStates && tierErr == nil {
					for path := range tiers {
						if tiers[path] >= rule.Limits.MaxTier {
							continue
						}
						target := append([]int(nil), tiers...)
						target[path]++
						key := tierKey(target)
						if _, legal := expected[key]; legal && !outgoing[key] {
							add(rec.File, "/"+pointerToken(s.EdgesField), "missing-purchase", fmt.Sprintf("build %s has no purchase transition to legal build %s", tierKey(tiers), key))
						}
					}
				}
			}
			if rule.RequireCompleteStates {
				var missing []string
				for key := range expected {
					if _, exists := builds[key]; !exists {
						missing = append(missing, key)
					}
				}
				sort.Strings(missing)
				for _, key := range missing {
					add(root.File, "/"+pointerToken(s.EdgesField), "missing-build", fmt.Sprintf("family %q cannot reach legal build %s", family, key))
				}
			}
		}
		if rule.RequireCompleteStates {
			missingFamilies := map[string]bool{}
			for _, rec := range all {
				if !p.scopeMatches(s, false, schemas, rec.Value) {
					continue
				}
				family, ok := rec.Value[rule.FamilyField].(string)
				if !ok || family == "" {
					add(rec.File, "/"+pointerToken(rule.FamilyField), "rule-family", "progression member requires a nonempty family identifier")
					continue
				}
				if _, exists := rootFamilies[family]; !exists && !missingFamilies[family] {
					add(rec.File, "/"+pointerToken(rule.FamilyField), "missing-root", fmt.Sprintf("family %q has progression members but no root in scope %q", family, s.Name))
					missingFamilies[family] = true
				}
			}
		}
		result.OutOfScopeRecords = len(all) - len(visited)
		result.Errors = len(report.Errors) - before
		report.Rules = append(report.Rules, result)
	}
}
