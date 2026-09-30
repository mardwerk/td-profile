package atlasvalidate

import (
	"fmt"
	"reflect"
)

type sourceSelector struct {
	Fields   map[string][]any
	Contains *modelContents
	All      []string
}
type modelContents struct {
	Field  string
	Models []string
}

// Source names and values are Profile settings. Schema files describe only the
// selector language; they contain no source-game flags or categories.
func (p profile) matchesSelector(name string, value map[string]any) bool {
	selector := p.Selectors[name]
	for _, reference := range selector.All {
		if !p.matchesSelector(reference, value) {
			return false
		}
	}
	for field, allowed := range selector.Fields {
		source, exists := value[field]
		if !exists {
			return false
		}
		found := false
		for _, candidate := range allowed {
			if reflect.DeepEqual(source, candidate) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if selector.Contains != nil {
		values, ok := value[selector.Contains.Field].([]any)
		if !ok {
			return false
		}
		found := false
		for _, item := range values {
			object, ok := item.(map[string]any)
			if !ok {
				continue
			}
			for _, model := range selector.Contains.Models {
				if p.kind(object) == model {
					found = true
				}
			}
		}
		if !found {
			return false
		}
	}
	return true
}
func (p profile) scopeMatches(s scope, root bool, schemas schemaIndex, value map[string]any) bool {
	schema, selector := s.MemberSchema, s.MemberSelector
	if root {
		schema, selector = s.RootSchema, s.RootSelector
	}
	if selector != "" {
		return p.matchesSelector(selector, value)
	}
	return schemas[schema].Validate(value) == nil
}
func validateSelectors(p profile) error {
	states := map[string]int{}
	var visit func(string) error
	visit = func(name string) error {
		selector, exists := p.Selectors[name]
		if !exists {
			return fmt.Errorf("unknown source selector %q", name)
		}
		if states[name] == 1 {
			return fmt.Errorf("source selector cycle at %q", name)
		}
		if states[name] == 2 {
			return nil
		}
		states[name] = 1
		for _, reference := range selector.All {
			if err := visit(reference); err != nil {
				return err
			}
		}
		states[name] = 2
		return nil
	}
	for name := range p.Selectors {
		if err := visit(name); err != nil {
			return err
		}
	}
	return nil
}
