#!/usr/bin/env bash
set -euo pipefail

if ! command -v cargo >/dev/null 2>&1; then
	echo "cargo is required" >&2
	exit 1
fi

cargo run -q -p ash-cli -- tools --json >/dev/null
cargo run -q -p ash-cli -- health --json >/dev/null
cargo run -q -p ash-cli -- config show --json >/dev/null
cargo run -q -p ash-cli -- scan --profile safe --json >/dev/null
