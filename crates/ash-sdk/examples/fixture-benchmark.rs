#![forbid(unsafe_code)]

use std::env;

use ash_sdk::{ScanProfile, benchmark_fixture_scan};
use tempfile::TempDir;

fn parse_profile() -> ScanProfile {
    let mut args = env::args().skip(1);
    while let Some(argument) = args.next() {
        if argument == "--profile" {
            return match args.next().as_deref() {
                Some("safe") | None => ScanProfile::Safe,
                Some("full") => ScanProfile::Full,
                Some(other) => panic!("unsupported fixture benchmark profile: {other}"),
            };
        }
    }
    ScanProfile::Safe
}

fn main() {
    let temp = TempDir::new().expect("fixture tempdir");
    let result = benchmark_fixture_scan(temp.path(), parse_profile()).expect("fixture benchmark");
    println!(
        "{}",
        serde_json::to_string(&result).expect("benchmark result JSON")
    );
}
