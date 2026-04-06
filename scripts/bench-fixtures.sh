#!/usr/bin/env bash
set -euo pipefail

if ! command -v hyperfine >/dev/null 2>&1; then
	echo "hyperfine is required for benchmark runs" >&2
	exit 2
fi

cargo build --release -p ash-sdk --example fixture-benchmark >/dev/null
FIXTURE_HOME="$(mktemp -d)"
trap 'rm -rf "$FIXTURE_HOME"' EXIT
target/release/examples/fixture-benchmark --home "$FIXTURE_HOME" --seed-only >/dev/null

hyperfine --warmup 1 \
	"target/release/examples/fixture-benchmark --home '$FIXTURE_HOME' --profile safe >/dev/null" \
	"target/release/examples/fixture-benchmark --home '$FIXTURE_HOME' --profile full >/dev/null"
