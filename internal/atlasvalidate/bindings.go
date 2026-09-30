package atlasvalidate

import "fmt"

// Profile-required fields express supported concepts. Shared schemas constrain
// their shapes when present without forcing every game to support those concepts.
func (p profile) requiredModelFields(kind string, view map[string]any) error {
	binding := p.MechanicByModel[kind]
	for _, field := range binding.RequiredFields {
		if _, exists := view[field]; !exists {
			source := binding.Fields[field]
			if source != "" {
				return fmt.Errorf("mechanic %q requires %q, bound to source field %q", binding.ID, field, source)
			}
			return fmt.Errorf("mechanic %q requires field %q", binding.ID, field)
		}
	}
	return nil
}
