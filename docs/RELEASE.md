# Releases

Run the packager from a clean, committed checkout with Go and Git installed:

```sh
go test ./...
go vet ./...
go test -race ./...
go run ./scripts/release.go -version v1.0.0 -out dist
```

The script builds the current machine's target with `CGO_ENABLED=0`. It rejects cross-target environment variables. Use `-target linux-amd64`, `-target windows-amd64` or `-target darwin-arm64` to require a particular native machine. A successful build on one target does not prove that another target runs.

Each archive is named `td-profile-<release>-<os>-<arch>`. Linux and macOS use `.tar.gz`; Windows uses `.zip`. The archive contains one directory with `validator` or `validator.exe`, the self-contained `profile/`, an empty `game-data/`, `README.md`, `LICENSE`, `NOTICE.md`, `licenses/`, `docs/`, `AGENTS.md`, `examples/minimal-game/` and `release.json`. The example contains only synthetic data. It contains no captured game-data. Compiled binaries and archives stay outside Git.

The script extracts the archive into a temporary directory outside the checkout. It verifies that the empty `game-data/` exists and passes with zero files. It verifies the included synthetic example and requires a complete score of 100. It then deletes `profile/mechanics.json` and requires a failure identifying that missing dependency. These checks never change the archived Profile or empty data directory.

Git checks out text with LF line endings on every platform. The packager rejects CRLF in bundled JSON instead of changing its bytes. This keeps the matching Profile's dependency digest identical across native archives.

`release.json` records the release tag, checker version and interface, Profile id, revision, format version and dependency SHA256, source commit, native target, Go version and disabled CGO. The Profile digest comes from the checker report. It hashes the resolved Profile dependencies, not the archive. The release tag, checker version and Profile revision are independent versions.

The adjacent `.sha256` file records the SHA256 of the archive bytes. Verify it before extracting on Linux:

```sh
sha256sum -c dist/td-profile-v1.0.0-linux-amd64.tar.gz.sha256
```

After extraction, run the checker from the bundle directory:

```sh
./validator --data game-data --profile profile
```

On Windows PowerShell, run `./validator.exe --data game-data --profile profile`. Users need no Go installation to run the bundled binary.

The Checks workflow runs Go tests, vet, race tests and a build. Pushing a `v*` tag starts the Release workflow. Each package job runs on its native Linux amd64, Windows amd64 or macOS arm64 runner, runs tests and vet, and executes the packager with the expected target. Only the final job has permission to create the GitHub release and upload the archives and checksums. A runner with a different architecture fails packaging. Review the tag and source commit before pushing the release tag.
