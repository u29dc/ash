#!/usr/bin/env bash
set -euo pipefail

if ! command -v cargo >/dev/null 2>&1; then
	echo "cargo is required" >&2
	exit 1
fi

PLAN_FILE="$(mktemp)"
trap 'rm -f "$PLAN_FILE"' EXIT

cargo run -q -p ash-cli -- tools --json >/dev/null
cargo run -q -p ash-cli -- health --json >/dev/null
cargo run -q -p ash-cli -- config show --json >/dev/null
cargo run -q -p ash-cli -- config validate --json >/dev/null
cargo run -q -p ash-cli -- scan --profile safe --output "$PLAN_FILE" --json >/dev/null
cargo run -q -p ash-cli -- apply --plan "$PLAN_FILE" --dry-run --json >/dev/null
