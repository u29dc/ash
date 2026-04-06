#!/usr/bin/env bash
set -euo pipefail

if ! command -v hyperfine >/dev/null 2>&1; then
	echo "hyperfine is required for benchmark runs" >&2
	exit 2
fi

hyperfine --warmup 1 'cargo run -q -p ash-cli -- scan --profile safe --json >/dev/null'
