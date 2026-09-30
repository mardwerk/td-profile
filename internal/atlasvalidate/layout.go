package atlasvalidate

import (
	"fmt"
	"path"
	"strings"
)

// Layout bindings constrain paths independently of record identity and schemas.
func checkLayout(file string, object map[string]any, c collection, report *Report) {
	relative := file
	if file == c.Path {
		relative = ""
	} else if c.Path != "." {
		relative = strings.TrimPrefix(file, c.Path+"/")
	}
	depth := 0
	if relative != "" {
		depth = len(strings.Split(relative, "/"))
	}
	if depth != c.Layout.Depth {
		report.add(file, "", "layout_depth", fmt.Sprintf("collection %q requires %d path segments below %q; found %d", c.Name, c.Layout.Depth, c.Path, depth))
	}
	bindings := []struct {
		field, actual, code, label string
	}{
		{c.Layout.ParentField, path.Base(path.Dir(file)), "layout_parent", "parent directory"},
		{c.Layout.StemField, strings.TrimSuffix(path.Base(file), ".json"), "layout_stem", "filename stem"},
	}
	for _, binding := range bindings {
		if binding.field == "" {
			continue
		}
		pointer := "/" + pointerToken(binding.field)
		value, ok := object[binding.field].(string)
		if !ok || value == "" {
			report.add(file, pointer, "layout_field", "layout binding requires a nonempty string field")
			continue
		}
		if value != binding.actual {
			report.add(file, pointer, binding.code, fmt.Sprintf("%s %q must match field %q value %q", binding.label, binding.actual, binding.field, value))
		}
	}
}
