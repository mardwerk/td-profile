// Build, archive and smoke-test a native release. Run from the repository root.
package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

type identity struct {
	ID            string `json:"id"`
	Revision      string `json:"revision"`
	FormatVersion int    `json:"formatVersion"`
	SHA256        string `json:"sha256"`
}
type report struct {
	Valid        bool     `json:"valid"`
	FilesChecked int      `json:"filesChecked"`
	Profile      identity `json:"profile"`
	Checker      struct {
		Version string `json:"version"`
	} `json:"checker"`
	Score *struct {
		Points   float64 `json:"points"`
		Complete bool    `json:"complete"`
	} `json:"score"`
}
type releaseInfo struct {
	Release          string   `json:"release"`
	CheckerVersion   string   `json:"checkerVersion"`
	CheckerInterface int      `json:"checkerInterface"`
	Profile          identity `json:"profile"`
	SourceCommit     string   `json:"sourceCommit"`
	GOOS             string   `json:"goos"`
	GOARCH           string   `json:"goarch"`
	GoVersion        string   `json:"goVersion"`
	CGOEnabled       bool     `json:"cgoEnabled"`
}

func main() {
	version := flag.String("version", "", "release tag, for example v1.0.0")
	output := flag.String("out", "dist", "output directory")
	target := flag.String("target", runtime.GOOS+"-"+runtime.GOARCH, "expected native OS and architecture")
	flag.Parse()
	if *target != runtime.GOOS+"-"+runtime.GOARCH {
		fmt.Fprintln(os.Stderr, "release: target must match the native OS and architecture")
		os.Exit(1)
	}
	if err := release(*version, *output); err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
}
func release(version, output string) error {
	if !regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[A-Za-z0-9.-]+)?$`).MatchString(version) {
		return errors.New("-version must be a release tag such as v1.0.0")
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	gitRoot, err := command(root, "git", "rev-parse", "--show-toplevel")
	if err != nil {
		return fmt.Errorf("cannot identify source repository: %w", err)
	}
	if filepath.Clean(strings.TrimSpace(string(gitRoot))) != root {
		return errors.New("run the script from the td-profile repository root")
	}
	commit, err := command(root, "git", "rev-parse", "--verify", "HEAD")
	if err != nil {
		return fmt.Errorf("source commit is required; commit the source before packaging: %w", err)
	}
	dirty, err := command(root, "git", "status", "--porcelain", "--untracked-files=all")
	if err != nil {
		return err
	}
	if len(strings.TrimSpace(string(dirty))) != 0 {
		return errors.New("source checkout must be clean before packaging")
	}
	for _, key := range []string{"GOOS", "GOARCH"} {
		expected := runtime.GOOS
		if key == "GOARCH" {
			expected = runtime.GOARCH
		}
		if value := os.Getenv(key); value != "" && value != expected {
			return fmt.Errorf("%s=%s is not the native target %s", key, value, expected)
		}
	}
	name := "td-profile-" + version + "-" + runtime.GOOS + "-" + runtime.GOARCH
	work, err := os.MkdirTemp("", "td-profile-release-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	stage := filepath.Join(work, name)
	if err := os.MkdirAll(filepath.Join(stage, "game-data"), 0755); err != nil {
		return err
	}
	for _, item := range []string{"profile", "README.md", "LICENSE", "NOTICE.md", "licenses", "docs", "AGENTS.md", "examples/minimal-game"} {
		if err := copyTree(filepath.Join(root, item), filepath.Join(stage, item)); err != nil {
			return fmt.Errorf("bundle %s: %w", item, err)
		}
	}
	binary := "validator"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-trimpath", "-o", filepath.Join(stage, binary), "./cmd/validator")
	build.Dir = root
	build.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS="+runtime.GOOS, "GOARCH="+runtime.GOARCH)
	if out, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("build: %w\n%s", err, out)
	}
	empty, err := check(stage, binary, "--data", "game-data", "--profile", "profile")
	if err != nil {
		return err
	}
	if !empty.Valid || empty.FilesChecked != 0 || empty.Profile.ID == "" || empty.Profile.Revision == "" || len(empty.Profile.SHA256) != 64 || empty.Checker.Version == "" {
		return fmt.Errorf("empty data check returned invalid release identity: %+v", empty)
	}
	manifestBytes, err := os.ReadFile(filepath.Join(stage, "profile", "manifest.json"))
	if err != nil {
		return err
	}
	var manifest struct {
		ValidatorFormatVersion int `json:"validatorFormatVersion"`
	}
	if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
		return err
	}
	info := releaseInfo{Release: version, CheckerVersion: empty.Checker.Version, CheckerInterface: manifest.ValidatorFormatVersion, Profile: empty.Profile, SourceCommit: strings.TrimSpace(string(commit)), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, GoVersion: runtime.Version(), CGOEnabled: false}
	metadata, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(stage, "release.json"), append(metadata, '\n'), 0644); err != nil {
		return err
	}
	if err := os.MkdirAll(output, 0755); err != nil {
		return err
	}
	suffix := ".tar.gz"
	if runtime.GOOS == "windows" {
		suffix = ".zip"
	}
	archive := filepath.Join(output, name+suffix)
	if err := pack(stage, archive); err != nil {
		return err
	}
	extracted := filepath.Join(work, "extracted")
	if err := unpack(archive, extracted); err != nil {
		return err
	}
	bundle := filepath.Join(extracted, name)
	if err := smoke(bundle, binary); err != nil {
		os.Remove(archive)
		return err
	}
	bytes, err := os.ReadFile(archive)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(bytes)
	checksum := hex.EncodeToString(digest[:]) + "  " + filepath.Base(archive) + "\n"
	if err := os.WriteFile(archive+".sha256", []byte(checksum), 0644); err != nil {
		return err
	}
	fmt.Printf("Built and tested native %s/%s archive: %s\n", runtime.GOOS, runtime.GOARCH, archive)
	return nil
}
func command(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("%s: %w: %s", name, err, out)
	}
	return out, nil
}
func check(dir, binary string, args ...string) (report, error) {
	var result report
	out, err := command(dir, filepath.Join(dir, binary), args...)
	if err != nil {
		return result, err
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return result, fmt.Errorf("decode checker report: %w", err)
	}
	return result, nil
}
func smoke(dir, binary string) error {
	entries, err := os.ReadDir(filepath.Join(dir, "game-data"))
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return errors.New("extracted game-data must be empty")
	}
	empty, err := check(dir, binary, "--data", "game-data", "--profile", "profile")
	if err != nil {
		return err
	}
	if !empty.Valid || empty.FilesChecked != 0 {
		return errors.New("extracted empty-data check failed")
	}
	example := "examples/minimal-game/game-data"
	valid, err := check(dir, binary, "--data", example, "--profile", "profile")
	if err != nil {
		return err
	}
	if !valid.Valid || valid.FilesChecked == 0 {
		return errors.New("extracted synthetic example check failed")
	}
	score, err := check(dir, binary, "score-tower", "--game-data", example, "--profile", "profile", "--tower", "Towers/Bolt/Bolt-0.json")
	if err != nil {
		return err
	}
	if score.Score == nil || !score.Score.Complete || score.Score.Points != 100 {
		return errors.New("extracted synthetic example must score 100/100")
	}
	if err := os.Remove(filepath.Join(dir, "profile", "mechanics.json")); err != nil {
		return err
	}
	cmd := exec.Command(filepath.Join(dir, binary), "--data", "game-data", "--profile", "profile")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() == 0 || !strings.Contains(string(out), "mechanics.json") {
		return fmt.Errorf("missing Profile dependency did not fail with a useful diagnostic: %s (%v)", out, err)
	}
	return nil
}
func copyTree(source, destination string) error {
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0755)
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("unsupported non-regular file: %s", path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.HasSuffix(path, ".json") && bytes.Contains(data, []byte("\r\n")) {
			return fmt.Errorf("release JSON must use LF line endings: %s", path)
		}
		return os.WriteFile(target, data, 0644)
	})
}
func pack(source, destination string) (err error) {
	file, err := os.Create(destination)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, file.Close()) }()
	var tw *tar.Writer
	var zw *zip.Writer
	var gz *gzip.Writer
	if strings.HasSuffix(destination, ".zip") {
		zw = zip.NewWriter(file)
		defer func() { err = errors.Join(err, zw.Close()) }()
	} else {
		gz = gzip.NewWriter(file)
		defer func() { err = errors.Join(err, gz.Close()) }()
		tw = tar.NewWriter(gz)
		defer func() { err = errors.Join(err, tw.Close()) }()
	}
	return filepath.Walk(source, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(filepath.Dir(source), path)
		if err != nil {
			return err
		}
		name := filepath.ToSlash(rel)
		var writer io.Writer
		if zw != nil {
			header, err := zip.FileInfoHeader(info)
			if err != nil {
				return err
			}
			header.Name = name
			if info.IsDir() {
				header.Name += "/"
			} else {
				header.Method = zip.Deflate
			}
			writer, err = zw.CreateHeader(header)
			if err != nil {
				return err
			}
		} else {
			header, err := tar.FileInfoHeader(info, "")
			if err != nil {
				return err
			}
			header.Name = name
			if err := tw.WriteHeader(header); err != nil {
				return err
			}
			writer = tw
		}
		if info.IsDir() {
			return nil
		}
		input, err := os.Open(path)
		if err != nil {
			return err
		}
		defer input.Close()
		_, err = io.Copy(writer, input)
		return err
	})
}
func unpack(source, destination string) error {
	if strings.HasSuffix(source, ".zip") {
		reader, err := zip.OpenReader(source)
		if err != nil {
			return err
		}
		defer reader.Close()
		for _, entry := range reader.File {
			if !filepath.IsLocal(filepath.FromSlash(entry.Name)) {
				return errors.New("unsafe archive path")
			}
			target := filepath.Join(destination, filepath.FromSlash(entry.Name))
			if entry.FileInfo().IsDir() {
				if err := os.MkdirAll(target, 0755); err != nil {
					return err
				}
				continue
			}
			input, err := entry.Open()
			if err != nil {
				return err
			}
			err = extractFile(target, entry.Mode(), input)
			input.Close()
			if err != nil {
				return err
			}
		}
		return nil
	}
	file, err := os.Open(source)
	if err != nil {
		return err
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if !filepath.IsLocal(filepath.FromSlash(header.Name)) {
			return errors.New("unsafe archive path")
		}
		target := filepath.Join(destination, filepath.FromSlash(header.Name))
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := extractFile(target, fs.FileMode(header.Mode), reader); err != nil {
				return err
			}
		default:
			return errors.New("unsupported archive entry")
		}
	}
}
func extractFile(path string, mode fs.FileMode, input io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode.Perm())
	if err != nil {
		return err
	}
	_, err = io.Copy(file, input)
	return errors.Join(err, file.Close())
}
