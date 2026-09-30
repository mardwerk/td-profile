package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestArchiveRoundTrip(t *testing.T) {
	for _, extension := range []string{".tar.gz", ".zip"} {
		t.Run(extension, func(t *testing.T) {
			work := t.TempDir()
			source := filepath.Join(work, "td-profile-v1.0.0-test")
			if err := os.MkdirAll(filepath.Join(source, "game-data"), 0755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(source, "atlas-validator"), []byte("executable"), 0755); err != nil {
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
			contents, err := os.ReadFile(filepath.Join(bundle, "atlas-validator"))
			if err != nil || string(contents) != "executable" {
				t.Fatalf("file contents lost: %q, %v", contents, err)
			}
			info, err := os.Stat(filepath.Join(bundle, "atlas-validator"))
			if err != nil {
				t.Fatal(err)
			}
			if runtime.GOOS != "windows" && info.Mode().Perm()&0111 == 0 {
				t.Fatal("executable permission lost")
			}
		})
	}
}
