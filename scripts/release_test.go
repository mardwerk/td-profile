package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCopyTreeJSONLineEndings(t *testing.T) {
	for _, test := range []struct{ name, lineEnding string }{{"LF", "\n"}, {"CRLF", "\r\n"}} {
		t.Run(test.name, func(t *testing.T) {
			work := t.TempDir()
			source := filepath.Join(work, "manifest.json")
			if err := os.WriteFile(source, []byte("{}"+test.lineEnding), 0644); err != nil {
				t.Fatal(err)
			}
			err := copyTree(source, filepath.Join(work, "copied.json"))
			if test.lineEnding == "\r\n" {
				if err == nil || !strings.Contains(err.Error(), "LF line endings") {
					t.Fatalf("CRLF must fail with a useful diagnostic: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestArchiveRoundTrip(t *testing.T) {
	for _, extension := range []string{".tar.gz", ".zip"} {
		t.Run(extension, func(t *testing.T) {
			work := t.TempDir()
			source := filepath.Join(work, "td-profile-v1.0.0-test")
			if err := os.MkdirAll(filepath.Join(source, "game-data"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "validator"), []byte("executable"), 0755); err != nil {
				t.Fatal(err)
			}
			archive := filepath.Join(work, "release"+extension)
			if err := pack(source, archive); err != nil {
				t.Fatal(err)
			}
			extracted := filepath.Join(work, "extracted")
			if err := unpack(archive, extracted); err != nil {
				t.Fatal(err)
			}
			bundle := filepath.Join(extracted, filepath.Base(source))
			entries, err := os.ReadDir(filepath.Join(bundle, "game-data"))
			if err != nil || len(entries) != 0 {
				t.Fatalf("empty directory lost: %v, %v", entries, err)
			}
			contents, err := os.ReadFile(filepath.Join(bundle, "validator"))
			if err != nil || string(contents) != "executable" {
				t.Fatalf("file contents lost: %q, %v", contents, err)
			}
			info, err := os.Stat(filepath.Join(bundle, "validator"))
			if err != nil {
				t.Fatal(err)
			}
			if runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
				t.Fatal("executable permission lost")
			}
		})
	}
}
