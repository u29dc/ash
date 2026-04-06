#![forbid(unsafe_code)]

use std::env;
use std::fs;
use std::path::PathBuf;

use ash_sdk::{
    FixtureSeedStats, ScanProfile, benchmark_fixture_scan, benchmark_seeded_fixture_scan,
    seed_benchmark_fixture,
};
use tempfile::TempDir;

const SEED_STATS_FILE: &str = ".ash-fixture-seed.json";

struct Args {
    profile: ScanProfile,
    home: Option<PathBuf>,
    seed_only: bool,
}

fn parse_args() -> Args {
    let mut args = env::args().skip(1);
    let mut profile = ScanProfile::Safe;
    let mut home = None;
    let mut seed_only = false;
    while let Some(argument) = args.next() {
        if argument == "--profile" {
            profile = match args.next().as_deref() {
                Some("safe") | None => ScanProfile::Safe,
                Some("full") => ScanProfile::Full,
                Some(other) => panic!("unsupported fixture benchmark profile: {other}"),
            };
        } else if argument == "--home" {
            home = Some(PathBuf::from(
                args.next().expect("fixture benchmark home path"),
            ));
        } else if argument == "--seed-only" {
            seed_only = true;
        }
    }
    Args {
        profile,
        home,
        seed_only,
    }
}

fn main() {
    let args = parse_args();
    let temp = TempDir::new().expect("fixture tempdir");
    let result = if let Some(home) = args.home {
        fs::create_dir_all(&home).expect("fixture home");
        let seed_stats_path = home.join(SEED_STATS_FILE);
        if args.seed_only {
            let seed = seed_benchmark_fixture(&home).expect("seed fixture");
            fs::write(
                &seed_stats_path,
                serde_json::to_vec(&seed).expect("fixture seed stats JSON"),
            )
            .expect("persist fixture seed stats");
            println!(
                "{}",
                serde_json::to_string(&seed).expect("fixture seed stats JSON")
            );
            return;
        }

        let seed: FixtureSeedStats = serde_json::from_slice(
            &fs::read(&seed_stats_path)
                .expect("fixture seed stats are missing; run with --seed-only first"),
        )
        .expect("fixture seed stats");
        benchmark_seeded_fixture_scan(&home, args.profile, seed).expect("fixture benchmark")
    } else {
        benchmark_fixture_scan(temp.path(), args.profile).expect("fixture benchmark")
    };
    println!(
        "{}",
        serde_json::to_string(&result).expect("benchmark result JSON")
    );
}
