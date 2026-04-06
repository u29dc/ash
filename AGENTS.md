> `ash` is a macOS-only Rust cleanup toolkit with a thin JSON-first CLI over a reusable SDK. It plans cleanup work first, executes only explicit plans, and keeps safe temporary cleanup separate from dangerous app-state cleanup.

## 1. Documentation

- Primary external references: [Bun](https://bun.sh/docs/llms.txt), [Clap](https://docs.rs/clap/latest/clap/), [Rust Book](https://doc.rust-lang.org/book/), [Rust Reference](https://doc.rust-lang.org/reference/), [Rust API Guidelines](https://rust-lang.github.io/api-guidelines/)
- Local source-of-truth files: [`Cargo.toml`](Cargo.toml), [`package.json`](package.json), [`crates/ash-sdk/src/lib.rs`](crates/ash-sdk/src/lib.rs), [`crates/ash-cli/src/main.rs`](crates/ash-cli/src/main.rs), [`README.md`](README.md)
- This file is the canonical repo-level agent document. [`CLAUDE.md`](CLAUDE.md) and [`README.md`](README.md) mirror it for tool compatibility.

## 2. Repository Structure

```text
.
├── crates/
│   ├── ash-sdk/             core contracts, config, health, planning, execution, trash, maintenance
│   └── ash-cli/             thin clap CLI and JSON envelope surface
├── scripts/                 benchmark and smoke-check helpers
├── .husky/                  local commit hooks
├── README.md                symlink mirror of AGENTS.md
└── AGENTS.md                canonical repo-level agent instructions
```

- Start in [`crates/ash-sdk/src/`](crates/ash-sdk/src/) for cleanup behavior, risk policy, and plan/execution logic.
- Start in [`crates/ash-cli/src/main.rs`](crates/ash-cli/src/main.rs) for command-surface changes and envelope wiring.
- Treat `target/` and `node_modules/` as generated or local runtime state.

## 3. Stack

| Layer | Choice | Notes |
| --- | --- | --- |
| Runtime | Rust 2024 workspace | `unsafe` forbidden, strict clippy |
| CLI | `clap` + `serde_json` | JSON-first, non-interactive, one-envelope stdout contract |
| Filesystem | `walkdir` + `rayon` | parallel read-only planning over macOS user directories |
| Config | TOML under `ASH_HOME` | resolved via env precedence |
| Tooling | Bun + Husky + Biome | JS/config tooling only; product logic is Rust |

## 4. Commands

- `cargo run -p ash-cli -- tools --json` - inspect the live command contract
- `cargo run -p ash-cli -- health --json` - inspect readiness and remediation steps
- `cargo run -p ash-cli -- config show --json` - inspect effective config and paths
- `cargo run -p ash-cli -- scan --profile safe --json` - produce a read-only cleanup plan
- `cargo run -p ash-cli -- apply --dry-run --plan <file> --json` - validate a plan without mutating state
- `bun run util:check` - required completion gate

## 5. Architecture

- [`crates/ash-cli/src/main.rs`](crates/ash-cli/src/main.rs): parses flags, resolves config, dispatches commands, and emits JSON envelopes
- [`crates/ash-sdk/src/contracts.rs`](crates/ash-sdk/src/contracts.rs): source of truth for envelope and tool metadata
- [`crates/ash-sdk/src/planner.rs`](crates/ash-sdk/src/planner.rs): read-only candidate discovery and cleanup plan generation
- [`crates/ash-sdk/src/executor.rs`](crates/ash-sdk/src/executor.rs): plan verification and trash moves
- [`crates/ash-sdk/src/policy.rs`](crates/ash-sdk/src/policy.rs): cleanup classes, risk tiers, and hard safety boundaries
- Contract invariant: JSON commands emit exactly one stdout envelope with `{ ok, data | error, meta }`
- Safety invariant: `scan` is read-only, `apply` executes only explicit plans, and dangerous app-state cleanup never rides along with generic safe cleanup

## 6. Runtime and State

- Home precedence: `ASH_HOME` -> `TOOLS_HOME/ash` -> `$HOME/.tools/ash`
- Config path: `$ASH_HOME/config.toml`
- Cache path: `$ASH_HOME/cache/`
- Installed binary path: `$ASH_HOME/ash`
- Read-only plans may be written anywhere by the caller; runtime-owned cache stays under `$ASH_HOME/cache`
- Environment variables that materially affect behavior: `ASH_HOME`, `TOOLS_HOME`

## 7. Conventions

- Keep CLI commands thin and move cleanup logic into the SDK.
- Keep command metadata registry-backed; `tools` output must not drift from real behavior.
- Prefer exact ownership evidence over heuristics for stateful app cleanup.
- Keep safe temp/cache cleanup and dangerous app-state cleanup distinct in policy, planning, and execution.

## 8. Constraints

- Never implement permanent delete behavior for normal cleanup; move to Trash only.
- Never treat `Application Support`, `Preferences`, `Containers`, `Group Containers`, `Cookies`, `HTTPStorages`, `WebKit`, or `LaunchAgents` as generic safe junk.
- Never let JSON mode leak logs or tables to stdout.
- Treat protected paths, symlink escapes, plan drift, and installed-app state as hard safety gates.

## 9. Validation

- Required gate: `bun run util:check`
- Required Rust checks: `cargo fmt --all --check`, `cargo clippy --workspace --all-targets --all-features -- -D warnings`, `cargo test --workspace`
- Required smoke path: `tools`, `health`, `config show`, `scan`, and `apply --dry-run`
- If you change the command surface or tool metadata, update the SDK registry and the CLI contract tests together
