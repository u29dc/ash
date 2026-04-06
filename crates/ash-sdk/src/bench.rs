use std::fs;
use std::path::Path;
use std::time::Instant;

use serde::{Deserialize, Serialize};

use crate::error::Result;
use crate::paths::ResolvedPaths;
use crate::planner::{PlanSummary, ScanOptions, ScanProfile, Scope, scan};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct FixtureSeedStats {
    pub directories: usize,
    pub files: usize,
    pub bytes_seeded: u64,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ScanBenchmarkResult {
    pub elapsed_ms: u64,
    pub profile: ScanProfile,
    pub scopes: Vec<Scope>,
    pub seed: FixtureSeedStats,
    pub summary: PlanSummary,
}

pub fn seed_benchmark_fixture(home: &Path) -> Result<FixtureSeedStats> {
    let mut directories = 0usize;
    let mut files = 0usize;
    let mut bytes_seeded = 0u64;

    for index in 0..48 {
        let temp_dir = home.join("tmp").join(format!("ash-temp-{index}"));
        fs::create_dir_all(&temp_dir)?;
        directories += 1;
        for file_index in 0..4 {
            let size = 2_048 + (index * 17) + file_index;
            let file_path = temp_dir.join(format!("blob-{file_index}.tmp"));
            fs::write(&file_path, vec![b't'; size])?;
            files += 1;
            bytes_seeded = bytes_seeded.saturating_add(size as u64);
        }
    }

    for index in 0..24 {
        let cache_dir = home
            .join("Library/Caches")
            .join(format!("com.example.fixture-cache-{index}"));
        fs::create_dir_all(&cache_dir)?;
        directories += 1;
        for file_index in 0..3 {
            let size = 4_096 + (index * 29) + file_index;
            let file_path = cache_dir.join(format!("cache-{file_index}.bin"));
            fs::write(&file_path, vec![b'c'; size])?;
            files += 1;
            bytes_seeded = bytes_seeded.saturating_add(size as u64);
        }
    }

    let browser_cache = home.join("Library/Caches/Google/Chrome/Default/Cache");
    fs::create_dir_all(&browser_cache)?;
    directories += 1;
    fs::write(browser_cache.join("index"), vec![b'b'; 32_768])?;
    files += 1;
    bytes_seeded = bytes_seeded.saturating_add(32_768);

    let homebrew_cache = home.join("Library/Caches/Homebrew");
    fs::create_dir_all(&homebrew_cache)?;
    directories += 1;
    fs::write(homebrew_cache.join("download.tar.gz"), vec![b'h'; 64_000])?;
    files += 1;
    bytes_seeded = bytes_seeded.saturating_add(64_000);

    for index in 0..12 {
        let log_dir = home.join("Library/Logs").join(format!("Fixture-{index}"));
        fs::create_dir_all(&log_dir)?;
        directories += 1;
        let file_path = log_dir.join("app.log");
        let size = 8_192 + (index * 13);
        fs::write(&file_path, vec![b'l'; size])?;
        files += 1;
        bytes_seeded = bytes_seeded.saturating_add(size as u64);
    }

    for (root, name) in [
        ("Library/Developer/Xcode/DerivedData", "DerivedData"),
        ("Library/Developer/Xcode/iOS DeviceSupport", "DeviceSupport"),
        ("Library/Developer/Xcode/Archives", "Archives"),
        ("Library/Developer/CoreSimulator/Devices", "Devices"),
    ] {
        let dir = home.join(root).join(format!("Fixture-{name}"));
        fs::create_dir_all(&dir)?;
        directories += 1;
        let file_path = dir.join("artifact.bin");
        fs::write(&file_path, vec![b'x'; 24_576])?;
        files += 1;
        bytes_seeded = bytes_seeded.saturating_add(24_576);
    }

    for (root, file_name, contents) in [
        (
            "Library/Application Support/com.example.fixture",
            "state.db",
            vec![b'a'; 12_288],
        ),
        (
            "Library/Preferences",
            "com.example.fixture.plist",
            vec![b'p'; 2_048],
        ),
        (
            "Library/Containers/com.example.fixture",
            "container.db",
            vec![b's'; 10_240],
        ),
        (
            "Library/Group Containers/group.com.example.fixture",
            "shared.db",
            vec![b'g'; 10_240],
        ),
        (
            "Library/WebKit/com.example.fixture",
            "site-data.db",
            vec![b'w'; 8_192],
        ),
        (
            "Library/HTTPStorages/com.example.fixture",
            "storage.bin",
            vec![b'h'; 6_144],
        ),
        (
            "Library/Cookies",
            "com.example.fixture.binarycookies",
            vec![b'k'; 4_096],
        ),
        (
            "Library/LaunchAgents",
            "com.example.fixture.agent.plist",
            vec![b'j'; 1_024],
        ),
    ] {
        let full_path = home.join(root).join(file_name);
        if let Some(parent) = full_path.parent() {
            fs::create_dir_all(parent)?;
            directories += 1;
        }
        fs::write(&full_path, contents.clone())?;
        files += 1;
        bytes_seeded = bytes_seeded.saturating_add(contents.len() as u64);
    }

    Ok(FixtureSeedStats {
        directories,
        files,
        bytes_seeded,
    })
}

pub fn benchmark_fixture_scan(home: &Path, profile: ScanProfile) -> Result<ScanBenchmarkResult> {
    let seed = seed_benchmark_fixture(home)?;
    let paths = ResolvedPaths::for_test_home(home);
    let start = Instant::now();
    let plan = scan(
        &paths,
        ScanOptions {
            app_target: None,
            profile,
            scopes: Vec::new(),
        },
    )?;

    Ok(ScanBenchmarkResult {
        elapsed_ms: start.elapsed().as_millis() as u64,
        profile,
        scopes: plan.scopes,
        seed,
        summary: plan.summary,
    })
}

#[cfg(test)]
mod tests {
    use tempfile::TempDir;

    use super::benchmark_fixture_scan;
    use crate::planner::ScanProfile;

    #[test]
    fn benchmark_fixture_scan_returns_a_summary() {
        let temp = TempDir::new().expect("tempdir");
        let result =
            benchmark_fixture_scan(temp.path(), ScanProfile::Full).expect("benchmark result");
        assert!(result.seed.files > 0);
        assert!(result.summary.total_candidates > 0);
    }
}
