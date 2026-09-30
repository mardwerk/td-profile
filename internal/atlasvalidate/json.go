package atlasvalidate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Decode tokens so duplicate keys are errors, rather than silently taking the last value.
func decodeJSON(raw []byte) (any, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	var read func() (any, error)
	read = func() (any, error) {
		token, err := d.Token()
		if err != nil {
			return nil, err
		}
		switch token {
		case json.Delim('{'):
			object := map[string]any{}
			for d.More() {
				keyToken, err := d.Token()
				if err != nil {
					return nil, err
				}
				key := keyToken.(string)
				if _, exists := object[key]; exists {
					return nil, fmt.Errorf("duplicate JSON key %q at byte %d", key, d.InputOffset())
				}
				value, err := read()
				if err != nil {
					return nil, err
				}
				object[key] = value
			}
			_, err := d.Token()
			return object, err
		case json.Delim('['):
			array := []any{}
			for d.More() {
				value, err := read()
				if err != nil {
					return nil, err
				}
				array = append(array, value)
			}
			_, err := d.Token()
			return array, err
		default:
			return token, nil
		}
	}
	value, err := read()
	if err != nil {
		return nil, err
	}
	if _, err := d.Token(); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("extra JSON value at byte %d", d.InputOffset())
		}
		return nil, err
	}
	return value, nil
}

func readJSON(path string) (any, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return decodeJSON(raw)
}

func typeName(value string) string {
	value = strings.SplitN(value, "[", 2)[0]
	value = strings.SplitN(value, ",", 2)[0]
	return value[strings.LastIndex(value, ".")+1:]
}

func pointerToken(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "~", "~0"), "/", "~1")
}

func localPath(root, relative string) (string, error) {
	if !filepath.IsLocal(filepath.FromSlash(relative)) {
		return "", fmt.Errorf("expected a local relative path, got %q", relative)
	}
	return filepath.Join(root, filepath.FromSlash(relative)), nil
}
