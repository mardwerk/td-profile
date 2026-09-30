package atlasvalidate

// project builds the temporary validation view. Bindings describe raw records;
// generic schemas see mechanic IDs and field names without rewriting the data.
func (p profile) project(value any) any {
	return p.projectValue(value, false)
}

func (p profile) projectValue(value any, insideModel bool) any {
	switch source := value.(type) {
	case map[string]any:
		discriminator, present := source[p.TypeIdentity.Field]
		view := make(map[string]any, len(source))
		for field, child := range source {
			view[field] = p.projectValue(child, insideModel || present)
		}
		if !present {
			if insideModel {
				// In a model graph, generic kind comes only from the configured
				// source discriminator, never from a coincidental raw field.
				delete(view, "kind")
			}
			return view
		}
		if _, valid := discriminator.(string); !valid {
			// Keep invalid values visible to the generic model schema.
			view["kind"] = view[p.TypeIdentity.Field]
			return view
		}
		kind := p.kind(source)
		if binding, known := p.MechanicByModel[kind]; known {
			// Snapshot the projected source fields so aliases cannot overwrite
			// another alias's input or project the same subtree repeatedly.
			projectedFields := view
			view = make(map[string]any, len(projectedFields)+len(binding.Fields)+1)
			for field, child := range projectedFields {
				view[field] = child
			}
			for field, rawField := range binding.Fields {
				if child, exists := projectedFields[rawField]; exists {
					view[field] = child
				} else {
					delete(view, field)
				}
			}
			view["kind"] = binding.ID
		} else {
			view["kind"] = p.unboundKind(kind)
		}
		return view
	case []any:
		view := make([]any, len(source))
		for i, child := range source {
			view[i] = p.projectValue(child, insideModel)
		}
		return view
	default:
		return value
	}
}

// Unknown source names cannot impersonate a known canonical mechanic kind.
// A string keeps generic model schemas compatible with report-only coverage.
func (p profile) unboundKind(kind string) string {
	projected := "unbound:" + kind
	for {
		collision := false
		for _, binding := range p.MechanicByModel {
			if binding.ID == projected {
				collision = true
				break
			}
		}
		if !collision {
			return projected
		}
		projected = "unbound:" + projected
	}
}

func (p profile) projectRecord(value map[string]any, c collection, path string) any {
	if c.IDField == "@keys" && !c.Typed {
		// A catalog's keys are identities, even when a key happens to have
		// the configured discriminator's name. Its entries may still be models.
		view := make(map[string]any, len(value))
		for field, child := range value {
			view[field] = p.projectValue(child, true)
		}
		return view
	}
	view := p.projectValue(value, c.Typed).(map[string]any)
	if _, typed := value[p.TypeIdentity.Field]; !typed {
		return view
	}
	if _, hasID := view["id"]; hasID || c.IDField == "@keys" {
		return view
	}
	if binding, known := p.MechanicByModel[p.kind(value)]; known {
		if _, bound := binding.Fields["id"]; bound {
			return view
		}
	}
	if ids := recordIDs(c, path, value); len(ids) == 1 {
		view["id"] = ids[0]
	}
	return view
}
