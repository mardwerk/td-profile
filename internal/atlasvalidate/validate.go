// Package atlasvalidate checks records against a self-contained data Profile.
package atlasvalidate

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type pendingReference struct {
	File, Pointer, Value string
	Rule                 referenceRule
}

func matches(path, collectionPath string) bool {
	return collectionPath == "." || path == collectionPath || strings.HasPrefix(path, collectionPath+"/")
}

// Validate reads game-data and its Profile without modifying either directory.
// Status is 0 for valid data, 1 for invalid data, and 2 for invocation/Profile errors.
func Validate(dataDirectory, profileDirectory string, includeRelations bool) (Report, int) {
	return validate(dataDirectory, profileDirectory, includeRelations, "")
}

func validate(dataDirectory, profileDirectory string, includeRelations bool, tower string) (Report, int) {
	r := Report{checks: map[string]map[string]bool{}, Errors: []Diagnostic{}, Checker: checker(), Rules: []RuleResult{}, Coverage: Coverage{UnboundModelTypes: []ModelCount{}}}
	p, schemas, err := loadProfile(profileDirectory)
	if err != nil {
		file := "manifest.json"
		var docErr *profileDocumentError
		if errors.As(err, &docErr) {
			file = docErr.File
		}
		r.add(file, "", "profile", err.Error())
		return r, 2
	}
	r.Game = p.Manifest.Game
	r.Profile = &p.Identity
	if p.ModelContracts.Contracts != nil {
		r.ModelContracts = &ModelContractCoverage{Types: len(p.ModelContracts.Contracts), Required: p.ModelContracts.Required, Provenance: p.ContractProvenance}
	}
	info, err := os.Stat(dataDirectory)
	if err != nil || !info.IsDir() {
		r.add(dataDirectory, "", "data_directory", "data path must be an existing directory")
		return r, 2
	}
	var paths []string
	if tower != "" {
		paths, err = selectTower(dataDirectory, tower, p, schemas)
	} else {
		paths, err = dataPaths(dataDirectory)
	}
	if err != nil {
		if tower != "" {
			r.add(dataDirectory, "", "selection", err.Error())
			return r, 2
		}
		r.add(dataDirectory, "", "read", err.Error())
		return r, 1
	}
	sort.Strings(paths)
	files := map[string]bool{}
	for _, path := range paths {
		files[path] = true
	}
	indices := map[string]map[string][]string{}
	for _, collection := range p.Collections {
		indices[collection.Name] = map[string][]string{}
	}
	records := recordIndex{}
	for _, s := range p.Scopes {
		records[s.Collection] = map[string][]record{}
	}
	for _, group := range p.Classifications.Groups {
		if records[group.Collection] == nil {
			records[group.Collection] = map[string][]record{}
		}
	}
	values := map[string]map[string]any{}
	unknownModels := map[string]int{}
	unknownExamples := map[string]*Location{}
	unitsByModel := map[string][]unitBinding{}
	for _, binding := range p.Units.Bindings {
		for _, model := range binding.Models {
			unitsByModel[model] = append(unitsByModel[model], binding)
		}
	}
	rulesByModel := map[string][]referenceRule{}
	for _, rule := range p.References {
		for _, model := range rule.Models {
			rulesByModel[model] = append(rulesByModel[model], rule)
		}
	}
	var pending []pendingReference
	for _, path := range paths {
		disabled := false
		active := false
		for _, c := range p.Collections {
			if matches(path, c.Path) {
				if c.active() {
					active = true
				} else {
					disabled = true
				}
			}
		}
		if disabled && !active {
			r.Coverage.FilesSkipped++
			continue
		}
		r.attempt("schema", path)
		value, err := readJSON(filepath.Join(dataDirectory, filepath.FromSlash(path)))
		r.FilesChecked++
		if err != nil {
			r.add(path, "", "json", err.Error())
			continue
		}
		object, ok := value.(map[string]any)
		if !ok {
			r.add(path, "", "schema", "capture files must contain a JSON object")
			continue
		}
		values[path] = object
		type rootProjection struct {
			schema string
			value  map[string]any
			model  bool
		}
		var rootViews []rootProjection
		typedRequired := false
		detectRootModel := p.recordHasModelRoot(path)
		layoutChecked := false
		for _, collection := range p.Collections {
			if !collection.active() || !matches(path, collection.Path) {
				continue
			}
			if collection.Layout != nil {
				r.attempt("layout", path)
				checkLayout(path, object, collection, &r)
				layoutChecked = true
			}
			typedRequired = typedRequired || collection.Typed
			projected := p.projectRecord(object, collection, path).(map[string]any)
			modelRoot := collection.Typed || collection.IDField != "@keys"
			rootViews = append(rootViews, rootProjection{collection.Schema, projected, modelRoot})
			if err := schemas[collection.Schema].Validate(projected); err != nil {
				r.add(path, "", "schema", err.Error())
			}
			ids := recordIDs(collection, path, object)
			if len(ids) == 0 && !strings.HasPrefix(collection.IDField, "@") {
				r.add(path, "/"+pointerToken(collection.IDField), "identity", "record identity must be a nonempty string")
			}
			for _, id := range ids {
				if scoped := records[collection.Name]; scoped != nil {
					scoped[id] = append(scoped[id], record{File: path, Value: object})
				}
				if prior, exists := indices[collection.Name][id]; exists && collection.IDField != "@parent" {
					r.add(path, "", "duplicate_identity", fmt.Sprintf("%s identity %q is already defined in %s", collection.Name, id, strings.Join(prior, ", ")))
				} else {
					indices[collection.Name][id] = append(indices[collection.Name][id], path)
				}
			}
		}
		if _, present := object[p.TypeIdentity.Field]; typedRequired && !present {
			r.add(path, "/"+pointerToken(p.TypeIdentity.Field), "schema", "typed collection records require discriminator field "+p.TypeIdentity.Field)
		}
		if layoutChecked {
			r.Coverage.LayoutFilesChecked++
		}
		if len(rootViews) > 0 {
			r.Coverage.FilesWithSchema++
		} else {
			r.Coverage.FilesWithoutSchema++
			r.checks["schema"][path] = false
			if p.UnmatchedFiles == "error" {
				r.add(path, "", "unmatched_file", "file does not belong to a declared collection")
			}
		}
		var walk func(any, string)
		walk = func(value any, pointer string) {
			switch value := value.(type) {
			case map[string]any:
				kind := ""
				sourceType := ""
				detectModel := pointer != "" || detectRootModel
				if rawType, exists := value[p.TypeIdentity.Field]; exists && detectModel {
					typeString, ok := rawType.(string)
					if !ok || typeString == "" {
						r.add(path, pointer+"/"+pointerToken(p.TypeIdentity.Field), "schema", p.TypeIdentity.Field+" must be a nonempty string")
					} else {
						sourceType = typeString
						kind = p.modelKind(typeString)
						if kind == "" {
							r.add(path, pointer+"/"+pointerToken(p.TypeIdentity.Field), "schema", "model discriminator must resolve to a nonempty model name")
						}
					}
				}
				if sourceType != "" {
					canonical := p.ModelSchemas[kind] != ""
					if canonical {
						r.Coverage.CanonicalSchemaModelInstances++
					}
					structural := false
					if p.ModelContracts.Contracts != nil {
						structural = p.ModelContracts.check(value, path, pointer, &r)
						if structural {
							r.Coverage.StructuralContractModelInstances++
						}
					}
					covered := canonical || structural
					if p.ModelContracts.Required {
						covered = structural
					}
					if covered {
						r.Coverage.BoundModelInstances++
					} else {
						r.Coverage.UnboundModelInstances++
						unknownKind := kind
						if unknownKind == "" {
							unknownKind = sourceType
						}
						unknownModels[unknownKind]++
						// Pick the first location lexically, independent of map traversal order.
						if example := unknownExamples[unknownKind]; example == nil || path < example.File || (path == example.File && pointer < example.Pointer) {
							unknownExamples[unknownKind] = &Location{File: path, Pointer: pointer}
						}
						if p.UnknownModels == "error" {
							message := fmt.Sprintf("model %q has no mechanic schema binding or structural contract", unknownKind)
							if p.ModelContracts.Required {
								message = fmt.Sprintf("model %q has no required structural contract for source type %q", unknownKind, sourceType)
							}
							r.add(path, pointer+"/"+pointerToken(p.TypeIdentity.Field), "unknown_model", message)
						}
					}
					for _, binding := range unitsByModel[kind] {
						if number, exists := value[binding.Field]; exists {
							r.attempt("units", path)
							r.Coverage.UnitBindingsChecked++
							if _, ok := number.(float64); !ok {
								r.add(path, pointer+"/"+pointerToken(binding.Field), "unit_type", fmt.Sprintf("field bound to unit %q must be numeric", binding.Unit))
							}
						}
					}
				}
				if schema := p.ModelSchemas[kind]; schema != "" {
					views := rootViews
					if pointer != "" || len(views) == 0 {
						views = []rootProjection{{value: p.project(value).(map[string]any), model: true}}
					}
					for _, projected := range views {
						if !projected.model {
							continue
						}
						if err := p.requiredModelFields(kind, projected.value); err != nil {
							r.add(path, pointer, "schema", err.Error())
						}
						if projected.schema != schema {
							if err := schemas[schema].Validate(projected.value); err != nil {
								r.add(path, pointer, "schema", err.Error())
							}
						}
					}
				}
				for _, rule := range rulesByModel[kind] {
					reference := value[rule.Field]
					if reference == nil {
						continue
					}
					var collect func(any, string)
					collect = func(value any, pointer string) {
						switch value := value.(type) {
						case string:
							if value != "" {
								pending = append(pending, pendingReference{path, pointer, value, rule})
							}
						case []any:
							for i, item := range value {
								itemPointer := pointer + "/" + strconv.Itoa(i)
								if _, ok := item.(string); !ok {
									r.add(path, itemPointer, "reference_type", "reference array items must be strings")
								} else {
									collect(item, itemPointer)
								}
							}
						default:
							r.add(path, pointer, "reference_type", "reference must be a string or an array of strings")
						}
					}
					collect(reference, pointer+"/"+pointerToken(rule.Field))
				}
				keys := make([]string, 0, len(value))
				for key := range value {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				for _, key := range keys {
					if !detectModel || key != p.TypeIdentity.Field {
						walk(value[key], pointer+"/"+pointerToken(key))
					}
				}
			case []any:
				for i, item := range value {
					walk(item, pointer+"/"+strconv.Itoa(i))
				}
			}
		}
		walk(object, "")
	}
	if includeRelations {
		r.Backlinks = map[string][]Location{}
	}
	for _, reference := range pending {
		r.attempt("references", reference.File)
		r.ReferencesChecked++
		value := reference.Value
		if alias, exists := reference.Rule.Aliases[value]; exists {
			value = alias
		}
		targets := indices[reference.Rule.Target][value]
		external := false
		if len(targets) == 0 {
			_, external = reference.Rule.ExternalSymbols[reference.Value]
		}
		if external {
			r.ExternalReferences++
		}
		if len(targets) == 0 && !external {
			r.add(reference.File, reference.Pointer, "missing_reference", fmt.Sprintf("%s record %q does not exist", reference.Rule.Target, reference.Value))
			continue
		}
		if includeRelations {
			if targets == nil {
				targets = []string{}
			}
			r.Relations = append(r.Relations, Relation{reference.File, reference.Pointer, reference.Value, reference.Rule.Target, value, targets, external})
			for _, target := range targets {
				r.Backlinks[target] = append(r.Backlinks[target], Location{reference.File, reference.Pointer})
			}
		}
	}
	for _, c := range p.Collections {
		if requiredRole(c, p, schemas, values) {
			present := false
			for path := range files {
				if matches(path, c.Path) {
					present = true
				}
			}
			r.attempt("layout", c.Path)
			if !present {
				r.add(c.Path, "", "missing_file", fmt.Sprintf("required data role %q is missing", c.Name))
			} else if len(indices[c.Name]) == 0 {
				r.add(c.Path, "", "empty_required_role", fmt.Sprintf("required data role %q contains no identifiable records", c.Name))
			}
		}
	}
	if tower == "" {
		readCaptureMetadata(dataDirectory, p, &r)
	}
	for kind, count := range unknownModels {
		r.Coverage.UnboundModelTypes = append(r.Coverage.UnboundModelTypes, ModelCount{Type: kind, Instances: count, Example: unknownExamples[kind]})
	}
	sort.Slice(r.Coverage.UnboundModelTypes, func(i, j int) bool {
		return r.Coverage.UnboundModelTypes[i].Type < r.Coverage.UnboundModelTypes[j].Type
	})
	r.IntegrityValid = len(r.Errors) == 0
	checkRules(p, schemas, records, &r)
	checkClassifications(p, schemas, records, &r)
	for _, diagnostic := range r.Errors {
		if diagnostic.RuleID == "" {
			r.IntegrityValid = false
		}
	}
	r.RulesValid = true
	for _, result := range r.Rules {
		if result.Errors > 0 {
			r.RulesValid = false
		}
	}
	sort.Slice(r.Errors, func(i, j int) bool {
		a, b := r.Errors[i], r.Errors[j]
		return a.File+"\x00"+a.Pointer+"\x00"+a.Code+"\x00"+a.Message < b.File+"\x00"+b.Pointer+"\x00"+b.Code+"\x00"+b.Message
	})
	if tower != "" {
		r.Score = scoreReport(p, &r)
		r.Score.Tower = tower
	}
	r.Valid = len(r.Errors) == 0
	if !r.Valid {
		return r, 1
	}
	return r, 0
}
