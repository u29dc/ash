> `ash` is a macOS-only Go TUI and dry-run CLI that scans common cleanup targets, optionally performs a deeper orphaned-app pass, and moves selected paths into `~/.Trash` instead of deleting permanently.

## 1. Documentation

- Primary references: [Bubble Tea](https://pkg.go.dev/github.com/charmbracelet/bubbletea), [Bubbles](https://pkg.go.dev/github.com/charmbracelet/bubbles), [Lip Gloss](https://pkg.go.dev/github.com/charmbracelet/lipgloss), [fastwalk](https://pkg.go.dev/github.com/charlievieth/fastwalk), [golangci-lint](https://golangci-lint.run/docs/)
- Local source-of-truth files: [`cmd/ash/main.go`](cmd/ash/main.go), [`internal/app/app.go`](internal/app/app.go), [`internal/app/commands.go`](internal/app/commands.go), [`internal/cleaner/trash.go`](internal/cleaner/trash.go), [`internal/scanner/orphan.go`](internal/scanner/orphan.go), [`internal/safety/guards.go`](internal/safety/guards.go), [`package.json`](package.json), [`Makefile`](Makefile), [`.goreleaser.yml`](.goreleaser.yml)
- Edit [`AGENTS.md`](AGENTS.md) only; [`CLAUDE.md`](CLAUDE.md) and [`README.md`](README.md) are tracked symlinks to it.

## 2. Repository Structure

```text
.
├── cmd/ash/                 process entrypoint and CLI flags
├── internal/app/            Bubble Tea model, views, and orchestration
├── internal/cleaner/        Trash moves, cleanup stats, cleanup modules
├── internal/scanner/        scan types, path config, and deep orphan detection
├── internal/safety/         protected-path and permission checks
├── internal/maintenance/    external macOS maintenance commands
├── internal/config/         JSON config under ash home
├── internal/tui/            grayscale theme and Lip Gloss styles
├── internal/testutil/       filesystem fixtures for tests
├── pkg/plist/               Info.plist helpers
├── package.json             Bun runner and documented quality gate
├── Makefile                 stricter Go-oriented alternative workflow
└── AGENTS.md                canonical repo-level agent instructions
```

- Start in [`internal/app/`](internal/app/) for flow changes, [`internal/cleaner/modules/`](internal/cleaner/modules/) for scan target coverage, [`internal/scanner/orphan.go`](internal/scanner/orphan.go) for deep-scan heuristics, and [`internal/safety/`](internal/safety/) for delete guards.
- Treat the repo-root `ash`, `bin/`, `coverage.out`, and `coverage.html` as local/generated artifacts; they are not tracked.

## 3. Stack

| Layer | Choice | Notes |
| --- | --- | --- |
| Runtime | Go 1.24.2 with toolchain 1.24.7 | macOS-only binary; exits on non-Darwin |
| UI | Bubble Tea + Bubbles + Lip Gloss | alt-screen TUI with home, scan, results, confirm, auth, clean, and maintenance views |
| Filesystem scan | `fastwalk` + `os.ReadDir` | size walks do not follow symlinks |
| Automation | Bun + Make | Bun is the documented runner; Make adds `gofumpt`-based formatting |
| Release | GoReleaser | darwin `amd64` and `arm64`, draft GitHub releases, Homebrew tap publish |

## 4. Commands

- `bun run dev` or `go run ./cmd/ash` - launch the interactive TUI locally.
- `go run ./cmd/ash --dry-run` - run the non-interactive scan report without a TTY.
- `bun run build` - install a version-stamped binary to `${ASH_HOME:-${TOOLS_HOME:-$HOME/.tools}/ash}/ash`.
- `bun run build:release` - build the trimmed release binary in the same install dir with `CGO_ENABLED=0`.
- `bun run test` - run `go test -race ./...`.
- `bun run util:check` - repo's documented completion gate; aggregates format, lint, vet, test, and build failures before exiting.
- `make check` - stricter alternative gate; runs `gofumpt -w .` plus `go fmt`, lint, vet, test, and build. Use it when formatting drift matters.
- `bun run deps` - run `go mod download && go mod tidy`.

## 5. Architecture

- [`cmd/ash/main.go`](cmd/ash/main.go) is the only CLI surface: interactive TUI by default plus `--dry-run`, `--help`, and `--version`.
- [`internal/app/app.go`](internal/app/app.go) owns the Bubble Tea state machine. High-risk cleanup requires a second Enter press after issues are shown; failed cleanup items remain selected.
- [`internal/app/commands.go`](internal/app/commands.go) runs enabled cleanup modules concurrently, merges module errors into `ScanStatusPartial`, and marks the whole scan failed only when every module fails or no cleanable entries survive.
- [`internal/cleaner/`](internal/cleaner/) never deletes permanently. Batch cleanup validates every path before any move, re-validates each entry immediately before moving, then renames into `~/.Trash`.
- [`internal/cleaner/modules/`](internal/cleaner/modules/) partitions scan targets by category. `App Leftovers` is disabled by default and only enabled by deep scan; `Caches` explicitly excludes Homebrew and browser cache roots to avoid overlap.
- [`internal/scanner/orphan.go`](internal/scanner/orphan.go) builds an installed-app index from `/Applications`, `/System/Applications`, and `~/Applications`, then matches leftovers by bundle ID, app name, or company name with `high` / `medium` / `low` confidence.
- [`internal/safety/`](internal/safety/) is the hard guardrail layer. It blocks protected paths, protected bundle IDs, symlink escapes, and risky confirmation cases such as iOS backups, Xcode archives, `Application Support`, and items larger than 1 GiB.

## 6. Runtime and State

- Config directory resolution: `ASH_HOME` -> `${TOOLS_HOME}/ash` -> `~/.tools/ash`; config lives at `config.json` in that directory via [`internal/config/config.go`](internal/config/config.go).
- Current runtime only consumes `Config.Parallelism` during module scan fan-out. The rest of the config schema exists on disk but is not wired into the live scan/clean flow yet.
- Build/install scripts write the executable into the same resolved ash home, not into `bin/`.
- Full Disk Access detection probes `~/Library/Mail`, `~/Library/Messages`, and `~/Library/Safari` and reports `granted`, `denied`, or `unknown`; denied access downgrades scans to partial instead of aborting immediately.
- Maintenance commands run external macOS tools directly; commands flagged `RequiresSudo` request one `sudo -v` authorization step before execution.
- `~/.Trash` is treated as runtime state. The cleaner creates it if missing, enforces `0700`, rejects symlinked trash paths, and generates unique basenames on collision.

## 7. Conventions

- Keep behavior layered: `app` orchestrates UI, `modules` discover entries, `scanner` handles analysis and orphan heuristics, `cleaner` performs moves, and `safety` owns protection rules.
- Preserve category ownership. If you add a new scan target, decide which module owns it and adjust exclusions so the same bytes do not appear in multiple categories.
- Preserve risk semantics. `Group Containers` and ambiguous `Application Support` leftovers intentionally surface as `RiskDangerous` and manual-review candidates even when they match an uninstalled app.
- Preserve maintenance metadata in [`internal/maintenance/commands.go`](internal/maintenance/commands.go); `RequiresSudo` and `Useful` are part of the command contract, and `Purge RAM` is intentionally marked low-value.
- Prefer actual target bodies over helper text. `make install` writes to the resolved ash home even though `make help` still mentions `/usr/local/bin`.
- Edit [`AGENTS.md`](AGENTS.md) only for repo-level agent docs; the tracked symlinks for [`CLAUDE.md`](CLAUDE.md) and [`README.md`](README.md) must continue to resolve to it.

## 8. Constraints

- Never bypass [`internal/safety/guards.go`](internal/safety/guards.go) or [`internal/cleaner/trash.go`](internal/cleaner/trash.go) with direct `os.RemoveAll`, permanent delete behavior, or Finder automation for normal cleanup.
- Treat protected paths as non-negotiable: `~/.ssh`, `~/.gnupg`, `~/Library/Keychains`, `.git`, `/Applications`, `/System`, `/usr`, `/bin`, `/sbin`, `/private/var/vm`, `/private/var/db`, `/Library/Keychains`, `/Network`, and `/cores`.
- Treat bundle IDs with prefixes `com.apple.` and `com.microsoft.` as protected app data during deep scan.
- High-risk files for regressions are [`internal/scanner/orphan.go`](internal/scanner/orphan.go), [`internal/safety/guards.go`](internal/safety/guards.go), [`internal/cleaner/trash.go`](internal/cleaner/trash.go), [`internal/app/app.go`](internal/app/app.go), [`package.json`](package.json), [`Makefile`](Makefile), and [`.goreleaser.yml`](.goreleaser.yml).
- Do not assume Bun and Make are equivalent. `bun run util:format` only runs `go fmt`, while `make fmt` also requires `gofumpt`.
- Release packaging is darwin-only and expects [`README.md`](README.md) to exist; do not break the symlinked mirror layout unless you replace it with an equally reliable mirror.

## 9. Validation

- Required documented gate: `bun run util:check`.
- Run `make check` as well when you touch formatting-heavy Go code, lint config, or anything that could drift under `gofumpt`.
- Run `go test ./internal/cleaner/... ./internal/safety/... ./internal/app/...` when changing cleanup flow, confirmation logic, Trash behavior, or permission handling.
- Run `go test ./internal/scanner/... ./internal/cleaner/modules/... ./pkg/plist/...` when changing deep-scan heuristics, module coverage, plist parsing, or risk/confidence mapping.
- Smoke test on macOS when changing CLI or runtime behavior: `go run ./cmd/ash --help`, `go run ./cmd/ash --version`, and `go run ./cmd/ash --dry-run`.
- After doc changes, verify `readlink CLAUDE.md` and `readlink README.md` still point at `AGENTS.md`.
