#!/usr/bin/env bash
set -euo pipefail

if ! command -v hyperfine >/dev/null 2>&1; then
	echo "hyperfine is required for benchmark runs" >&2
	exit 2
fi

cargo build --release -p ash-cli >/dev/null

hyperfine --warmup 1 'target/release/ash scan --profile full --json >/dev/null'
