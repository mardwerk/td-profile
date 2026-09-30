package atlasvalidate

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

func (c collection) active() bool { return c.Enabled == nil || *c.Enabled }
func (p profile) modelKind(raw string) string {
	if p.TypeIdentity.Encoding == "literal" {
		return raw
	}
	return typeName(raw)
}
func (p profile) kind(value map[string]any) string {
	raw, _ := value[p.TypeIdentity.Field].(string)
	return p.modelKind(raw)
}
func recordIDs(c collection, path string, object map[string]any) []string {
	switch c.IDField {
	case "@keys":
		var ids []string
		for key := range object {
			if !slices.Contains(c.ExcludedKeys, key) {
				ids = append(ids, key)
			}
		}
		return ids
	case "@parent":
		return []string{filepath.Base(filepath.Dir(path))}
	case "@stem":
		return []string{strings.TrimSuffix(filepath.Base(path), ".json")}
	case "@path":
		return []string{strings.TrimPrefix(path, c.Path+"/")}
	default:
		id, _ := object[c.IDField].(string)
		if id != "" {
			return []string{id}
		}
	}
	return nil
}
func dataPaths(directory string) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(directory, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			rel, _ := filepath.Rel(directory, path)
			paths = append(paths, filepath.ToSlash(rel))
		}
		return nil
	})
	sort.Strings(paths)
	return paths, err
}
func walkObjects(value any, visit func(map[string]any)) {
	switch v := value.(type) {
	case map[string]any:
		visit(v)
		for _, child := range v {
			walkObjects(child, visit)
		}
	case []any:
		for _, child := range v {
			walkObjects(child, visit)
		}
	}
}

func (p profile) recordHasModelRoot(path string) bool {
	matched := false
	for _, c := range p.Collections {
		if !c.active() || !matches(path, c.Path) {
			continue
		}
		matched = true
		if c.Typed || c.IDField != "@keys" {
			return true
		}
	}
	return !matched
}

func (p profile) walkRecordObjects(path string, value map[string]any, visit func(map[string]any)) {
	if p.recordHasModelRoot(path) {
		walkObjects(value, visit)
		return
	}
	for _, child := range value {
		walkObjects(child, visit)
	}
}

// Conditions describe consumers, not a saved inventory of a capture.
func requiredRole(c collection, p profile, schemas schemaIndex, values map[string]map[string]any) bool {
	if !c.active() {
		return false
	}
	for _, condition := range c.RequireWhen {
		for path, value := range values {
			if condition.Collection != "" {
				for _, source := range p.Collections {
					if source.Name == condition.Collection && source.active() && matches(path, source.Path) && (condition.SelectorSchema == "" || schemas[condition.SelectorSchema].Validate(value) == nil) && (condition.Selector == "" || p.matchesSelector(condition.Selector, value)) {
						return true
					}
				}
			} else {
				found := false
				p.walkRecordObjects(path, value, func(object map[string]any) {
					for _, model := range condition.Models {
						if p.kind(object) == model {
							if field, ok := object[condition.Field]; ok && field != nil && field != "" {
								found = true
							}
						}
					}
				})
				if found {
					return true
				}
			}
		}
	}
	return false
}

// Selection indexes identities to locate outgoing dependencies. Errors and coverage
// are collected only later, from the selected paths, never from the whole index.
func selectTower(directory, target string, p profile, schemas schemaIndex) ([]string, error) {
	root, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return nil, err
	}
	relative, err := filepath.Rel(root, targetAbs)
	if err != nil || !filepath.IsLocal(relative) { // Also accept a game-data relative path.
		if !filepath.IsLocal(target) {
			return nil, fmt.Errorf("tower must be inside game-data")
		}
		relative = filepath.Clean(target)
	}
	relative = filepath.ToSlash(relative)
	targetPath := filepath.Join(root, filepath.FromSlash(relative))
	resolved, err := filepath.EvalSymlinks(targetPath)
	if err != nil {
		return nil, fmt.Errorf("tower: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	containment, err := filepath.Rel(resolvedRoot, resolved)
	if err != nil || !filepath.IsLocal(containment) {
		return nil, fmt.Errorf("tower must be inside game-data")
	}
	paths, err := dataPaths(root)
	if err != nil {
		return nil, err
	}
	values := map[string]map[string]any{}
	indices := map[string]map[string][]string{}
	for _, c := range p.Collections {
		indices[c.Name] = map[string][]string{}
	}
	for _, path := range paths {
		value, err := readJSON(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			continue
		}
		object, ok := value.(map[string]any)
		if !ok {
			continue
		}
		values[path] = object
		for _, c := range p.Collections {
			if c.active() && matches(path, c.Path) {
				for _, id := range recordIDs(c, path, object) {
					indices[c.Name][id] = append(indices[c.Name][id], path)
				}
			}
		}
	}
	object := values[relative]
	if object == nil {
		return nil, fmt.Errorf("tower must be a readable JSON object")
	}
	var targetCollection collection
	for _, c := range p.Collections {
		if c.Name == p.Scoring.Collection {
			targetCollection = c
		}
	}
	if !targetCollection.active() || !matches(relative, targetCollection.Path) {
		return nil, fmt.Errorf("tower must belong to scoring collection %q", p.Scoring.Collection)
	}
	family, _ := object[p.Scoring.FamilyField].(string)
	if family == "" {
		return nil, fmt.Errorf("tower needs family field %q", p.Scoring.FamilyField)
	}
	selected := map[string]bool{}
	var queue []string
	add := func(path string) {
		if !selected[path] {
			selected[path] = true
			queue = append(queue, path)
		}
	}
	addFamily := func(path string) {
		if selected[path] {
			return
		}
		add(path)
		if !matches(path, targetCollection.Path) {
			return
		}
		member := values[path]
		family, _ := member[p.Scoring.FamilyField].(string)
		for _, candidate := range paths {
			if !matches(candidate, targetCollection.Path) {
				continue
			}
			// Retain malformed state files in the same configured family directory.
			sameDirectory := targetCollection.Layout != nil && targetCollection.Layout.ParentField == p.Scoring.FamilyField && filepath.Dir(candidate) == filepath.Dir(path)
			candidateFamily, _ := values[candidate][p.Scoring.FamilyField].(string)
			if sameDirectory || (family != "" && candidateFamily == family) {
				add(candidate)
			}
		}
	}
	addFamily(relative)
	rolesIncluded := map[string]bool{}
	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]
		value := values[path]
		p.walkRecordObjects(path, value, func(object map[string]any) {
			for _, ref := range p.References {
				for _, model := range ref.Models {
					if model != p.kind(object) {
						continue
					}
					var ids []string
					switch raw := object[ref.Field].(type) {
					case string:
						ids = []string{raw}
					case []any:
						for _, item := range raw {
							if id, ok := item.(string); ok {
								ids = append(ids, id)
							}
						}
					}
					for _, id := range ids {
						if alias, ok := ref.Aliases[id]; ok {
							id = alias
						}
						for _, dependency := range indices[ref.Target][id] {
							addFamily(dependency)
						}
					}
				}
			}
		})
		current := map[string]map[string]any{path: value}
		for _, c := range p.Collections {
			if !rolesIncluded[c.Name] && requiredRole(c, p, schemas, current) {
				rolesIncluded[c.Name] = true
				for _, candidate := range paths {
					if matches(candidate, c.Path) {
						add(candidate)
					}
				}
			}
		}
	}
	result := make([]string, 0, len(selected))
	for path := range selected {
		result = append(result, path)
	}
	sort.Strings(result)
	return result, nil
}
