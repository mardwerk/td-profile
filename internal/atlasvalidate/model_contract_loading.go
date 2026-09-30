package atlasvalidate

import (
	"fmt"
	"path/filepath"

	"github.com/google/jsonschema-go/jsonschema"
)

// Contract includes are Profile dependencies, subject to the same local-file
// loader and digest as schemas. A shared include is loaded only once.
func loadModelContractDocuments(loader *profileLoader, schema *jsonschema.Resolved, root string, document modelContractsDocument) (modelContractsDocument, error) {
	combined := modelContractsDocument{RequireContracts: document.RequireContracts, Provenance: document.Provenance}
	states := map[string]int{}
	var visit func(string, modelContractsDocument, int) error
	visit = func(file string, doc modelContractsDocument, depth int) error {
		if depth > maxModelShapeDepth {
			return documentError(file, fmt.Errorf("model-contract include depth exceeds %d", maxModelShapeDepth))
		}
		file = filepath.ToSlash(filepath.Clean(file))
		if states[file] == 1 {
			return documentError(file, fmt.Errorf("model-contract include cycle at %q", file))
		}
		if states[file] == 2 {
			return nil
		}
		states[file] = 1
		combined.Contracts = append(combined.Contracts, doc.Contracts...)
		combined.RequireContracts = combined.RequireContracts || doc.RequireContracts
		for _, include := range doc.Includes {
			if !filepath.IsLocal(filepath.FromSlash(include)) {
				return documentError(file, fmt.Errorf("model-contract include %q must be a local relative path", include))
			}
			path := filepath.ToSlash(filepath.Join(filepath.Dir(file), filepath.FromSlash(include)))
			value, err := loader.read(path)
			if err != nil {
				return err
			}
			if err := schema.Validate(value); err != nil {
				return documentError(path, err)
			}
			var child modelContractsDocument
			if err := decodeDocument(value, &child); err != nil {
				return documentError(path, err)
			}
			if err := visit(path, child, depth+1); err != nil {
				return err
			}
		}
		states[file] = 2
		return nil
	}
	err := visit(root, document, 0)
	return combined, err
}
