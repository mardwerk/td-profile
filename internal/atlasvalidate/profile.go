package atlasvalidate

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
)

type profileDocumentError struct {
	File string
	Err  error
}

func (e *profileDocumentError) Error() string { return e.Err.Error() }
func (e *profileDocumentError) Unwrap() error { return e.Err }

func documentError(file string, err error) error {
	var previous *profileDocumentError
	if errors.As(err, &previous) {
		return err
	}
	return &profileDocumentError{File: file, Err: err}
}

// profileLoader resolves only files contained in the supplied Profile directory.
// Its read cache also defines the exact dependency closure used for the digest.
type profileLoader struct {
	root    string
	raw     map[string][]byte
	schemas map[string]*jsonschema.Schema
}

func newProfileLoader(directory string) (*profileLoader, error) {
	root, err := filepath.Abs(directory)
	if err != nil {
		return nil, err
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("profile path must be an existing directory")
	}
	return &profileLoader{root: root, raw: map[string][]byte{}, schemas: map[string]*jsonschema.Schema{}}, nil
}

func (l *profileLoader) read(relative string) (_ any, err error) {
	defer func() {
		if err != nil {
			err = documentError(relative, err)
		}
	}()
	path, err := localPath(l.root, relative)
	if err != nil {
		return nil, err
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", relative, err)
	}
	contained, err := filepath.Rel(l.root, resolved)
	if err != nil || !filepath.IsLocal(contained) {
		return nil, fmt.Errorf("%s: Profile dependency leaves its directory", relative)
	}
	relative = filepath.ToSlash(filepath.Clean(filepath.FromSlash(relative)))
	raw, exists := l.raw[relative]
	if !exists {
		raw, err = os.ReadFile(resolved)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", relative, err)
		}
		l.raw[relative] = raw
	}
	value, err := decodeJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", relative, err)
	}
	return value, nil
}

func decodeDocument(value any, target any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func (l *profileLoader) loadSchema(uri *url.URL) (*jsonschema.Schema, error) {
	if uri.Scheme != "file" || uri.Host != "" || uri.RawQuery != "" {
		return nil, fmt.Errorf("schema reference %q must resolve inside the Profile; network loading is disabled", uri.String())
	}
	relative, err := filepath.Rel(l.root, filepath.FromSlash(uri.Path))
	if err != nil || !filepath.IsLocal(relative) {
		return nil, fmt.Errorf("schema reference %q leaves the Profile directory", uri.String())
	}
	relative = filepath.ToSlash(relative)
	if schema := l.schemas[relative]; schema != nil {
		return schema, nil
	}
	value, err := l.read(relative)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(raw, &schema); err != nil {
		return nil, fmt.Errorf("%s: %w", relative, err)
	}
	l.schemas[relative] = &schema
	return &schema, nil
}

func (l *profileLoader) resolve(reference string) (_ *jsonschema.Resolved, err error) {
	defer func() {
		if err != nil {
			err = documentError(strings.SplitN(reference, "#", 2)[0], err)
		}
	}()
	ref, err := url.Parse(reference)
	if err != nil || ref.IsAbs() || ref.Host != "" || ref.RawQuery != "" || ref.Path == "" {
		return nil, fmt.Errorf("expected a local schema file reference, got %q", reference)
	}
	path, err := localPath(l.root, ref.Path)
	if err != nil {
		return nil, err
	}
	uri := &url.URL{Scheme: "file", Path: filepath.ToSlash(path)}
	schema, err := l.loadSchema(uri)
	if err != nil {
		return nil, err
	}
	uri.Fragment = ref.Fragment
	wrapper := &jsonschema.Schema{Schema: schema.Schema, Ref: uri.String()}
	// The wrapper must have a different URI from its target, or its $ref
	// resolves back to itself instead of loading the schema document.
	resolved, err := wrapper.Resolve(&jsonschema.ResolveOptions{BaseURI: "urn:atlasvalidate:entry", Loader: l.loadSchema, ValidateDefaults: true})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", reference, err)
	}
	return resolved, nil
}

func (l *profileLoader) identity(m manifest) ProfileIdentity {
	paths := make([]string, 0, len(l.raw))
	for path := range l.raw {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	hash := sha256.New()
	for _, path := range paths {
		raw := l.raw[path]
		fmt.Fprintf(hash, "%d:%s:%d:", len(path), path, len(raw))
		hash.Write(raw)
	}
	return ProfileIdentity{ID: m.ID, Revision: m.Revision, FormatVersion: m.FormatVersion, SHA256: hex.EncodeToString(hash.Sum(nil)), Files: len(paths)}
}

func loadProfile(directory string) (profile, schemaIndex, error) {
	var p profile
	loader, err := newProfileLoader(directory)
	if err != nil {
		return p, nil, err
	}
	schemas := schemaIndex{}
	resolve := func(reference string) error {
		if schemas[reference] != nil {
			return nil
		}
		resolved, err := loader.resolve(reference)
		if err == nil {
			schemas[reference] = resolved
		}
		return err
	}
	manifestValue, err := loader.read("manifest.json")
	if err != nil {
		return p, nil, err
	}
	manifestSchema := "schemas/profile/manifest.schema.json"
	if err := resolve(manifestSchema); err != nil {
		return p, nil, err
	}
	if err := schemas[manifestSchema].Validate(manifestValue); err != nil {
		return p, nil, fmt.Errorf("manifest.json: %w", err)
	}
	if err := decodeDocument(manifestValue, &p.Manifest); err != nil {
		return p, nil, fmt.Errorf("manifest.json: %w", err)
	}
	if p.Manifest.FormatVersion != profileFormatVersion || p.Manifest.ValidatorFormatVersion != validatorFormatVersion {
		return p, nil, fmt.Errorf("unsupported Profile format %d or validator format %d", p.Manifest.FormatVersion, p.Manifest.ValidatorFormatVersion)
	}
	var collections collectionsDocument
	var references referencesDocument
	var mechanics mechanicsDocument
	var rules rulesDocument
	var contracts modelContractsDocument
	targets := []struct {
		role   string
		target any
	}{
		{"scoring", &p.Scoring}, {"collections", &collections}, {"references", &references}, {"mechanics", &mechanics}, {"rules", &rules}, {"numericalUnits", &p.Units},
	}
	if _, exists := p.Manifest.Documents["classifications"]; exists {
		targets = append(targets, struct {
			role   string
			target any
		}{"classifications", &p.Classifications})
	}
	if _, exists := p.Manifest.Documents["modelContracts"]; exists {
		targets = append(targets, struct {
			role   string
			target any
		}{"modelContracts", &contracts})
	}
	for _, target := range targets {
		doc, exists := p.Manifest.Documents[target.role]
		if !exists {
			return p, nil, fmt.Errorf("manifest has no %s document", target.role)
		}
		value, err := loader.read(doc.File)
		if err != nil {
			return p, nil, err
		}
		if err := resolve(doc.Schema); err != nil {
			return p, nil, err
		}
		if err := schemas[doc.Schema].Validate(value); err != nil {
			return p, nil, documentError(doc.File, fmt.Errorf("%s: %w", doc.File, err))
		}
		if err := decodeDocument(value, target.target); err != nil {
			return p, nil, documentError(doc.File, fmt.Errorf("%s: %w", doc.File, err))
		}
	}
	p.Collections, p.Scopes = collections.Collections, collections.Scopes
	p.UnmatchedFiles = collections.UnmatchedFiles
	if p.UnmatchedFiles == "" {
		p.UnmatchedFiles = "report"
	}
	if p.UnmatchedFiles != "report" && p.UnmatchedFiles != "error" {
		return p, nil, documentError(p.Manifest.Documents["collections"].File, fmt.Errorf("unknown unmatched-file policy %q", p.UnmatchedFiles))
	}
	p.References, p.Mechanics, p.UnknownModels, p.Rules = references.References, mechanics.Mechanics, mechanics.UnknownModels, rules.Rules
	p.TypeIdentity = mechanics.TypeIdentity
	if ref, exists := p.Manifest.Documents["modelContracts"]; exists {
		contracts, err = loadModelContractDocuments(loader, schemas[ref.Schema], ref.File, contracts)
		if err != nil {
			return p, nil, err
		}
		p.ModelContracts, err = compileModelContracts(contracts, p.TypeIdentity)
		if err != nil {
			return p, nil, documentError(ref.File, err)
		}
		p.ContractProvenance = contracts.Provenance
	}
	p.Selectors = rules.Selectors
	if err := validateSelectors(p); err != nil {
		return p, nil, documentError(p.Manifest.Documents["rules"].File, err)
	}
	p.ModelSchemas = map[string]string{}
	p.MechanicByModel = map[string]mechanic{}
	names := map[string]bool{}
	for i := range p.Collections {
		c := &p.Collections[i]
		if names[c.Name] {
			return p, nil, documentError(p.Manifest.Documents["collections"].File, fmt.Errorf("duplicate collection %q", c.Name))
		}
		names[c.Name] = true
		path, err := localPath(".", c.Path)
		if err != nil {
			return p, nil, documentError(p.Manifest.Documents["collections"].File, err)
		}
		c.Path = filepath.ToSlash(path)
		if c.Layout != nil && c.Layout.Depth < 0 {
			return p, nil, documentError(p.Manifest.Documents["collections"].File, fmt.Errorf("collection %q has a negative layout depth", c.Name))
		}
		if err := resolve(c.Schema); err != nil {
			return p, nil, err
		}
		if strings.HasPrefix(c.IDField, "@") && c.IDField != "@keys" && c.IDField != "@parent" && c.IDField != "@stem" && c.IDField != "@path" {
			return p, nil, documentError(p.Manifest.Documents["collections"].File, fmt.Errorf("unknown identity selector %q", c.IDField))
		}
	}

	if !names[p.Scoring.Collection] {
		return p, nil, documentError(p.Manifest.Documents["scoring"].File, fmt.Errorf("unknown scoring collection %q", p.Scoring.Collection))
	}
	for _, c := range p.Collections {
		for _, condition := range c.RequireWhen {
			if condition.Collection != "" {
				if !names[condition.Collection] {
					return p, nil, fmt.Errorf("unknown required-role source %q", condition.Collection)
				}
			}
			if condition.Selector != "" {
				if _, exists := p.Selectors[condition.Selector]; !exists {
					return p, nil, fmt.Errorf("unknown required-role selector %q", condition.Selector)
				}
			}
			if condition.SelectorSchema != "" {
				if err := resolve(condition.SelectorSchema); err != nil {
					return p, nil, err
				}
			}
		}
	}
	if binding := p.Manifest.CaptureMetadata; binding != nil {
		cleaned := filepath.ToSlash(filepath.Clean(binding.Path))
		if !filepath.IsLocal(cleaned) {
			return p, nil, fmt.Errorf("capture metadata path must be local to its configured base")
		}
	}
	ids := map[string]bool{}
	for _, m := range p.Mechanics {
		if ids[m.ID] {
			return p, nil, documentError(p.Manifest.Documents["mechanics"].File, fmt.Errorf("duplicate mechanic %q", m.ID))
		}
		ids[m.ID] = true
		if err := resolve(m.Schema); err != nil {
			return p, nil, err
		}
		for _, model := range m.Models {
			if _, exists := p.ModelSchemas[model]; exists {
				return p, nil, documentError(p.Manifest.Documents["mechanics"].File, fmt.Errorf("model %q has more than one mechanic binding", model))
			}
			p.ModelSchemas[model] = m.Schema
			p.MechanicByModel[model] = m
		}
	}
	if p.UnknownModels != "report" && p.UnknownModels != "error" {
		return p, nil, fmt.Errorf("unknown model policy %q", p.UnknownModels)
	}
	for _, ref := range p.References {
		if !names[ref.Target] {
			return p, nil, documentError(p.Manifest.Documents["references"].File, fmt.Errorf("unknown reference target %q", ref.Target))
		}
		for symbol, reason := range ref.ExternalSymbols {
			if symbol == "" || reason == "" {
				return p, nil, fmt.Errorf("external symbols require a value and reason")
			}
		}
	}
	for _, s := range p.Scopes {
		if s.RootSchema != "" {
			if err := resolve(s.RootSchema); err != nil {
				return p, nil, err
			}
		}
		if s.MemberSchema != "" {
			if err := resolve(s.MemberSchema); err != nil {
				return p, nil, err
			}
		}
	}
	if err := validateRules(p); err != nil {
		return p, nil, documentError(p.Manifest.Documents["rules"].File, err)
	}
	for _, group := range p.Classifications.Groups {
		for _, kind := range group.Types {
			if kind.SelectorSchema != "" {
				if err := resolve(kind.SelectorSchema); err != nil {
					return p, nil, err
				}
			}
		}
	}
	if err := validateClassifications(p); err != nil {
		return p, nil, documentError(p.Manifest.Documents["classifications"].File, err)
	}
	unitIDs := map[string]bool{}
	for _, unit := range p.Units.Units {
		if unitIDs[unit.ID] {
			return p, nil, documentError(p.Manifest.Documents["numericalUnits"].File, fmt.Errorf("duplicate unit %q", unit.ID))
		}
		unitIDs[unit.ID] = true
	}
	bindings := map[string]bool{}
	for _, binding := range p.Units.Bindings {
		if !unitIDs[binding.Unit] {
			return p, nil, documentError(p.Manifest.Documents["numericalUnits"].File, fmt.Errorf("unknown numerical unit %q", binding.Unit))
		}
		for _, model := range binding.Models {
			key := model + "/" + binding.Field
			if bindings[key] {
				return p, nil, documentError(p.Manifest.Documents["numericalUnits"].File, fmt.Errorf("duplicate unit binding %q", key))
			}
			bindings[key] = true
		}
	}
	if err := resolve(p.Manifest.ProposalSchema); err != nil {
		return p, nil, err
	}
	p.Identity = loader.identity(p.Manifest)
	return p, schemas, nil
}
