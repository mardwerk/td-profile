package atlasvalidate

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// Source contracts describe the serialized structure before canonical projection.
// They deliberately make no claim about the runtime meaning of a model.
type modelContractsDocument struct {
	Includes         []string        `json:"includes,omitempty"`
	Contracts        []modelContract `json:"contracts,omitempty"`
	RequireContracts bool            `json:"requireContracts,omitempty"`
	Provenance       map[string]any  `json:"provenance,omitempty"`
}

type modelContract struct {
	SourceType       string               `json:"sourceType"`
	Fields           []modelContractField `json:"fields"`
	AdditionalFields *modelShape          `json:"additionalFields,omitempty"`
}

type modelContractField struct {
	Field string     `json:"field"`
	Shape modelShape `json:"shape"`
}

type modelShape struct {
	Type             string               `json:"type,omitempty"`
	AnyOf            []modelShape         `json:"anyOf,omitempty"`
	Fields           []modelContractField `json:"fields,omitempty"`
	AdditionalFields *modelShape          `json:"additionalFields,omitempty"`
	Items            *modelShape          `json:"items,omitempty"`
	MaxItems         *int                 `json:"maxItems,omitempty"`
	Models           []string             `json:"models,omitempty"`
	Enum             []string             `json:"enum,omitempty"`
}

type modelContractIndex struct {
	Contracts map[string]modelContract
	TypeField string
	Required  bool
}

const maxModelShapeDepth = 64

func compileModelContracts(doc modelContractsDocument, identity typeIdentity) (modelContractIndex, error) {
	index := modelContractIndex{Contracts: map[string]modelContract{}, TypeField: identity.Field, Required: doc.RequireContracts}
	if identity.Field == "" {
		return index, fmt.Errorf("source contracts require a configured discriminator field")
	}
	for _, contract := range doc.Contracts {
		if contract.SourceType == "" {
			return index, fmt.Errorf("source contract type must not be empty")
		}
		if _, exists := index.Contracts[contract.SourceType]; exists {
			return index, fmt.Errorf("duplicate source contract for %q", contract.SourceType)
		}
		index.Contracts[contract.SourceType] = contract
	}
	for i, contract := range doc.Contracts {
		location := "/contracts/" + strconv.Itoa(i)
		for _, field := range contract.Fields {
			if field.Field == identity.Field {
				return index, fmt.Errorf("%s: discriminator %q must not be declared as a contract field", location, identity.Field)
			}
		}
		shape := modelShape{Type: "object", Fields: contract.Fields, AdditionalFields: contract.AdditionalFields}
		if err := index.validateShape(shape, location, 0); err != nil {
			return index, fmt.Errorf("source contract %q: %w", contract.SourceType, err)
		}
	}
	return index, nil
}

func (index modelContractIndex) validateShape(shape modelShape, location string, depth int) error {
	fail := func(message string) error { return fmt.Errorf("%s: %s", location, message) }
	if depth > maxModelShapeDepth {
		return fail("shape nesting exceeds limit of 64")
	}
	if shape.AnyOf != nil {
		if len(shape.AnyOf) < 2 {
			return fail("anyOf requires at least two alternatives")
		}
		if shape.Type != "" || shape.Fields != nil || shape.AdditionalFields != nil || shape.Items != nil || shape.MaxItems != nil || shape.Models != nil || shape.Enum != nil {
			return fail("anyOf cannot be combined with type-specific constraints")
		}
		for i, alternative := range shape.AnyOf {
			if err := index.validateShape(alternative, location+"/anyOf/"+strconv.Itoa(i), depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if shape.Type != "object" && (shape.Fields != nil || shape.AdditionalFields != nil) {
		return fail("fields and additionalFields require object type")
	}
	if shape.Type != "array" && (shape.Items != nil || shape.MaxItems != nil) {
		return fail("items and maxItems require array type")
	}
	if shape.Type != "model" && shape.Models != nil {
		return fail("models requires model type")
	}
	if shape.Type != "string" && shape.Enum != nil {
		return fail("enum requires string type")
	}
	switch shape.Type {
	case "null", "number", "integer", "boolean":
	case "string":
		if shape.Enum != nil {
			if len(shape.Enum) == 0 {
				return fail("enum must not be empty")
			}
			seen := map[string]bool{}
			for _, value := range shape.Enum {
				if seen[value] {
					return fail("duplicate enum value " + strconv.Quote(value))
				}
				seen[value] = true
			}
		}
	case "model":
		if len(shape.Models) == 0 {
			return fail("model shape requires allowed source types")
		}
		seen := map[string]bool{}
		for _, sourceType := range shape.Models {
			if seen[sourceType] {
				return fail("duplicate nested model type " + strconv.Quote(sourceType))
			}
			seen[sourceType] = true
			if _, exists := index.Contracts[sourceType]; !exists {
				return fail("unknown nested model type " + strconv.Quote(sourceType))
			}
		}
	case "array":
		if shape.MaxItems != nil && *shape.MaxItems < 0 {
			return fail("maxItems must be nonnegative")
		}
		if shape.Items == nil && (shape.MaxItems == nil || *shape.MaxItems != 0) {
			return fail("array requires items or maxItems of zero")
		}
		if shape.Items != nil {
			return index.validateShape(*shape.Items, location+"/items", depth+1)
		}
	case "object":
		seen := map[string]bool{}
		for i, field := range shape.Fields {
			if seen[field.Field] {
				return fail("duplicate field " + strconv.Quote(field.Field))
			}
			seen[field.Field] = true
			if err := index.validateShape(field.Shape, location+"/fields/"+strconv.Itoa(i)+"/shape", depth+1); err != nil {
				return err
			}
		}
		if shape.AdditionalFields != nil {
			return index.validateShape(*shape.AdditionalFields, location+"/additionalFields", depth+1)
		}
	default:
		return fail("unknown shape type " + strconv.Quote(shape.Type))
	}
	return nil
}

type contractFailure struct{ Pointer, Message string }

// check validates this model's serialized fields. Model-valued children have
// their discriminator checked here; the normal record walk checks their own
// contracts once. This avoids repeated validation of large nested models.
func (index modelContractIndex) check(value map[string]any, file, pointer string, report *Report) bool {
	sourceType, valid := value[index.TypeField].(string)
	contract, bound := index.Contracts[sourceType]
	if !valid || sourceType == "" {
		if index.Required {
			report.add(file, pointer+"/"+pointerToken(index.TypeField), "model_contract", "source discriminator must be a nonempty string")
		}
		return false
	}
	if !bound {
		if index.Required {
			report.add(file, pointer+"/"+pointerToken(index.TypeField), "model_contract", "no source contract for "+strconv.Quote(sourceType))
		}
		return false
	}
	for _, failure := range index.checkFields(value, contract.Fields, contract.AdditionalFields, pointer, index.TypeField, 0) {
		report.add(file, failure.Pointer, "model_contract", fmt.Sprintf("%s: %s", sourceType, failure.Message))
	}
	return true
}

func (index modelContractIndex) checkFields(value map[string]any, fields []modelContractField, additional *modelShape, pointer, discriminator string, depth int) []contractFailure {
	var failures []contractFailure
	known := map[string]bool{}
	if discriminator != "" {
		known[discriminator] = true
	}
	for _, field := range fields {
		known[field.Field] = true
		fieldPointer := pointer + "/" + pointerToken(field.Field)
		item, exists := value[field.Field]
		if !exists {
			failures = append(failures, contractFailure{fieldPointer, "required source field is missing"})
			continue
		}
		failures = append(failures, index.checkShape(item, field.Shape, fieldPointer, depth+1)...)
	}
	var extra []string
	for field := range value {
		if !known[field] {
			extra = append(extra, field)
		}
	}
	sort.Strings(extra)
	for _, field := range extra {
		fieldPointer := pointer + "/" + pointerToken(field)
		if additional == nil {
			failures = append(failures, contractFailure{fieldPointer, "unexpected source field"})
		} else {
			failures = append(failures, index.checkShape(value[field], *additional, fieldPointer, depth+1)...)
		}
	}
	return failures
}

func (index modelContractIndex) checkShape(value any, shape modelShape, pointer string, depth int) []contractFailure {
	fail := func(message string) []contractFailure { return []contractFailure{{pointer, message}} }
	if depth > maxModelShapeDepth {
		return fail("value nesting exceeds source contract limit of 64")
	}
	if shape.AnyOf != nil {
		var reasons []string
		for _, alternative := range shape.AnyOf {
			failures := index.checkShape(value, alternative, pointer, depth+1)
			if len(failures) == 0 {
				return nil
			}
			reasons = append(reasons, failures[0].Pointer+": "+failures[0].Message)
		}
		return fail("no allowed shape matched (" + strings.Join(reasons, "; ") + ")")
	}
	switch shape.Type {
	case "null":
		if value == nil {
			return nil
		}
	case "boolean":
		if _, ok := value.(bool); ok {
			return nil
		}
	case "number", "integer":
		if number, ok := value.(float64); ok && !math.IsNaN(number) && !math.IsInf(number, 0) {
			if shape.Type == "number" || math.Trunc(number) == number {
				return nil
			}
		}
	case "string":
		if text, ok := value.(string); ok {
			if shape.Enum == nil {
				return nil
			}
			for _, allowed := range shape.Enum {
				if text == allowed {
					return nil
				}
			}
			return fail("string must be one of " + strings.Join(shape.Enum, ", "))
		}
	case "model":
		if object, ok := value.(map[string]any); ok {
			sourceType, ok := object[index.TypeField].(string)
			if !ok || sourceType == "" {
				return []contractFailure{{pointer + "/" + pointerToken(index.TypeField), "nested model requires a nonempty source discriminator"}}
			}
			for _, allowed := range shape.Models {
				if sourceType == allowed {
					return nil
				}
			}
			return []contractFailure{{pointer + "/" + pointerToken(index.TypeField), "nested source model " + strconv.Quote(sourceType) + " is not allowed here"}}
		}
	case "object":
		if object, ok := value.(map[string]any); ok {
			return index.checkFields(object, shape.Fields, shape.AdditionalFields, pointer, "", depth)
		}
	case "array":
		if items, ok := value.([]any); ok {
			if shape.MaxItems != nil && len(items) > *shape.MaxItems {
				return fail(fmt.Sprintf("array contains %d items; maximum is %d", len(items), *shape.MaxItems))
			}
			var failures []contractFailure
			for i, item := range items {
				if shape.Items == nil {
					return fail("array has no allowed item shape")
				}
				failures = append(failures, index.checkShape(item, *shape.Items, pointer+"/"+strconv.Itoa(i), depth+1)...)
			}
			return failures
		}
	}
	return fail("expected " + shape.Type)
}
