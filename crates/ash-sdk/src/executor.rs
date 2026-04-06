use std::fs;
use std::path::Path;
use std::process::Command;
use std::time::{Duration, SystemTime};

use serde::{Deserialize, Serialize};

use crate::config::load_config;
use crate::error::{AshError, ErrorCode, Result};
use crate::inventory::{
    AppRecord, extract_bundle_id, find_app_by_bundle_id, find_unique_app_by_name, load_inventory,
};
use crate::paths::ResolvedPaths;
use crate::planner::{CleanupPlan, verify_plan_hash};
use crate::policy::{
    CandidateClass, RiskLevel, is_protected_absolute_path, is_safe_tool_cache_name,
};
use crate::trash::move_to_trash;

const TEMP_MIN_AGE: Duration = Duration::from_secs(24 * 60 * 60);
const CACHE_MIN_AGE: Duration = Duration::from_secs(72 * 60 * 60);
const LOG_MIN_AGE: Duration = Duration::from_secs(7 * 24 * 60 * 60);
const BROWSER_CACHE_MIN_AGE: Duration = Duration::from_secs(24 * 60 * 60);

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum MaxRisk {
    Safe,
    Review,
    Dangerous,
}

impl MaxRisk {
    fn as_risk(self) -> RiskLevel {
        match self {
            Self::Safe => RiskLevel::Safe,
            Self::Review => RiskLevel::Review,
            Self::Dangerous => RiskLevel::Dangerous,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ApplyRequest {
    pub plan: CleanupPlan,
    pub max_risk: MaxRisk,
    pub dry_run: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ApplyResultItem {
    pub candidate_id: String,
    pub path: String,
    pub outcome: String,
    pub detail: String,
    pub backend: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ExecutionReport {
    pub plan_hash: String,
    pub dry_run: bool,
    pub max_risk: MaxRisk,
    pub moved_count: usize,
    pub blocked_count: usize,
    pub failed_count: usize,
    pub items: Vec<ApplyResultItem>,
}

pub fn parse_cleanup_plan_payload(payload: &str) -> Result<CleanupPlan> {
    if let Ok(plan) = serde_json::from_str::<CleanupPlan>(payload) {
        return Ok(plan);
    }

    let value = serde_json::from_str::<serde_json::Value>(payload).map_err(|error| {
        AshError::new(
            ErrorCode::PlanInvalid,
            format!("failed to parse cleanup plan JSON: {error}"),
            "re-run `ash scan` and pass the generated plan JSON to `ash apply`",
        )
    })?;

    let plan_value = value
        .get("data")
        .and_then(|data| data.get("plan"))
        .cloned()
        .or_else(|| value.get("plan").cloned())
        .ok_or_else(|| {
            AshError::new(
                ErrorCode::PlanInvalid,
                "the provided JSON does not contain a cleanup plan",
                "pass a raw CleanupPlan JSON document or pipe the `ash scan --json` envelope to `ash apply`",
            )
        })?;

    serde_json::from_value(plan_value).map_err(|error| {
        AshError::new(
            ErrorCode::PlanInvalid,
            format!("failed to decode cleanup plan JSON: {error}"),
            "pass a raw CleanupPlan JSON document or pipe the `ash scan --json` envelope to `ash apply`",
        )
    })
}

pub fn apply_plan(paths: &ResolvedPaths, request: ApplyRequest) -> Result<ExecutionReport> {
    verify_plan_hash(&request.plan)?;
    let config = load_config(paths)?;
    let inventory = load_inventory(paths, &config)?;
    let mut moved_count = 0usize;
    let mut blocked_count = 0usize;
    let mut failed_count = 0usize;
    let mut items = Vec::new();
    let max_risk = request.max_risk.as_risk();

    for candidate in &request.plan.candidates {
        if !candidate.eligible {
            blocked_count += 1;
            items.push(ApplyResultItem {
                backend: None,
                candidate_id: candidate.id.clone(),
                detail: candidate
                    .blocked_reason
                    .clone()
                    .unwrap_or_else(|| "candidate is not eligible in this plan".to_string()),
                outcome: "blocked".to_string(),
                path: candidate.path.clone(),
            });
            continue;
        }

        if !max_risk.allows(candidate.risk) {
            blocked_count += 1;
            items.push(ApplyResultItem {
                backend: None,
                candidate_id: candidate.id.clone(),
                detail: format!(
                    "candidate risk {} exceeds max-risk {}",
                    candidate.risk.as_str(),
                    max_risk.as_str()
                ),
                outcome: "blocked".to_string(),
                path: candidate.path.clone(),
            });
            continue;
        }

        let path = Path::new(&candidate.path);
        if !path.exists() {
            failed_count += 1;
            items.push(ApplyResultItem {
                backend: None,
                candidate_id: candidate.id.clone(),
                detail: "candidate path no longer exists".to_string(),
                outcome: "failed".to_string(),
                path: candidate.path.clone(),
            });
            continue;
        }

        let current_canonical = fs::canonicalize(path)
            .unwrap_or_else(|_| path.to_path_buf())
            .display()
            .to_string();
        if current_canonical != candidate.canonical_path {
            return Err(AshError::new(
                ErrorCode::PlanDrift,
                format!(
                    "candidate {} drifted from {} to {}",
                    candidate.id, candidate.canonical_path, current_canonical
                ),
                "re-run `ash scan` to generate a fresh plan before applying it",
            ));
        }

        if fs::symlink_metadata(path)
            .map(|metadata| metadata.file_type().is_symlink())
            .unwrap_or(false)
        {
            return Err(AshError::new(
                ErrorCode::SafetyBlocked,
                format!("refusing to apply a symlink candidate: {}", candidate.path),
                "remove the symlink from the plan by re-running `ash scan`",
            ));
        }

        if is_protected_absolute_path(&current_canonical, &paths.user_home.display().to_string()) {
            return Err(AshError::new(
                ErrorCode::SafetyBlocked,
                format!("candidate entered a protected path: {}", candidate.path),
                "remove the protected candidate from the plan by re-running `ash scan`",
            ));
        }

        if let Some(reason) = revalidate_candidate(path, candidate, &inventory) {
            blocked_count += 1;
            items.push(ApplyResultItem {
                backend: None,
                candidate_id: candidate.id.clone(),
                detail: reason,
                outcome: "blocked".to_string(),
                path: candidate.path.clone(),
            });
            continue;
        }

        if request.dry_run {
            moved_count += 1;
            items.push(ApplyResultItem {
                backend: None,
                candidate_id: candidate.id.clone(),
                detail: "dry-run: candidate would be moved to Trash".to_string(),
                outcome: "wouldMove".to_string(),
                path: candidate.path.clone(),
            });
            continue;
        }

        match move_to_trash(path, paths) {
            Ok(backend) => {
                moved_count += 1;
                items.push(ApplyResultItem {
                    backend: Some(match backend {
                        crate::trash::TrashBackend::Native => "native".to_string(),
                        crate::trash::TrashBackend::Fallback => "fallback".to_string(),
                    }),
                    candidate_id: candidate.id.clone(),
                    detail: "candidate moved to Trash".to_string(),
                    outcome: "moved".to_string(),
                    path: candidate.path.clone(),
                });
            }
            Err(error) => {
                failed_count += 1;
                items.push(ApplyResultItem {
                    backend: None,
                    candidate_id: candidate.id.clone(),
                    detail: error.to_string(),
                    outcome: "failed".to_string(),
                    path: candidate.path.clone(),
                });
            }
        }
    }

    Ok(ExecutionReport {
        blocked_count,
        dry_run: request.dry_run,
        failed_count,
        items,
        max_risk: request.max_risk,
        moved_count,
        plan_hash: request.plan.plan_hash,
    })
}

fn revalidate_candidate(
    path: &Path,
    candidate: &crate::planner::CleanupCandidate,
    inventory: &[AppRecord],
) -> Option<String> {
    if path_has_open_handles(path) {
        return Some("candidate still has open file handles".to_string());
    }

    match candidate.class {
        CandidateClass::TempPath => {
            let name = file_name(path)?;
            if name.starts_with("com.apple.") {
                return Some("Apple-owned Darwin temp entries are report-only in v1".to_string());
            }
            if let Some(owner) = installed_owner_for_name(name, inventory) {
                return Some(format!(
                    "installed app temp entries are not eligible in generic scans: {}",
                    owner.bundle_id
                ));
            }
            min_age_block_reason(path, TEMP_MIN_AGE, "temp")
        }
        CandidateClass::UserCache => {
            let name = file_name(path)?;
            if name.starts_with("com.apple.") {
                return Some("Apple-owned cache entries are report-only in generic scans".to_string());
            }
            if let Some(owner) = installed_owner_for_cache_name(name, inventory) {
                return Some(format!(
                    "installed app caches are not eligible in generic scans: {}",
                    owner.bundle_id
                ));
            }
            if !is_safe_tool_cache_name(name) {
                return Some(
                    "generic cache cleanup is blocked unless the cache is explicitly classified as safe"
                        .to_string(),
                );
            }
            min_age_block_reason(path, CACHE_MIN_AGE, "cache")
        }
        CandidateClass::UserLog => min_age_block_reason(path, LOG_MIN_AGE, "log"),
        CandidateClass::BrowserDiskCache => {
            let bundle_id = evidence_value(candidate, "bundleId")?;
            if let Some(owner) = installed_owner_for_bundle_id(bundle_id, inventory)
                && app_has_running_processes(&owner)
            {
                return Some(
                    "browser cache cleanup is blocked while the browser appears to be running"
                        .to_string(),
                );
            }
            min_age_block_reason(path, BROWSER_CACHE_MIN_AGE, "browser cache")
        }
        CandidateClass::AppCacheLeftover
        | CandidateClass::AppLogLeftover
        | CandidateClass::ApplicationSupport
        | CandidateClass::PreferencePlist
        | CandidateClass::SandboxContainer
        | CandidateClass::GroupContainer
        | CandidateClass::SavedApplicationState
        | CandidateClass::WebKitData
        | CandidateClass::HttpStorage
        | CandidateClass::CookieStore
        | CandidateClass::LaunchAgent => {
            let bundle_id = evidence_value(candidate, "bundleId")?;
            if bundle_id.starts_with("com.apple.") {
                return Some("Apple app state is blocked from targeted cleanup".to_string());
            }
            if let Some(owner) = installed_owner_for_bundle_id(bundle_id, inventory) {
                if matches!(
                    candidate.class,
                    CandidateClass::AppCacheLeftover | CandidateClass::AppLogLeftover
                ) {
                    return Some("installed app cache cleanup is disabled in v1".to_string());
                }
                return Some(format!(
                    "stateful app cleanup is blocked because the app is installed: {}",
                    owner.bundle_id
                ));
            }
            if bundle_id_has_running_processes(bundle_id, inventory) {
                return Some("the targeted app still appears to be running".to_string());
            }
            None
        }
        CandidateClass::HomebrewCache => Some(
            "Homebrew cleanup should execute via `brew cleanup --dry-run` or `brew cleanup`, not by trashing the cache directory directly".to_string(),
        ),
        CandidateClass::XcodeDeviceSupport => Some(
            "device support cleanup is review-only and requires manual confirmation in a future execution path".to_string(),
        ),
        CandidateClass::XcodeArchives => Some(
            "Xcode archives are report-only in v1 because they may be required for symbolication or release history".to_string(),
        ),
        CandidateClass::SimulatorDeviceSet => {
            Some("simulator device trees are reported but not eligible in generic plans".to_string())
        }
        CandidateClass::SystemLog => Some("system log cleanup is not eligible in v1".to_string()),
        CandidateClass::XcodeDerivedData => None,
    }
}

fn file_name(path: &Path) -> Option<&str> {
    path.file_name().and_then(|name| name.to_str())
}

fn evidence_value<'a>(
    candidate: &'a crate::planner::CleanupCandidate,
    kind: &str,
) -> Option<&'a str> {
    candidate
        .evidence
        .iter()
        .find(|evidence| evidence.kind == kind)
        .map(|evidence| evidence.detail.as_str())
}

fn min_age_block_reason(path: &Path, min_age: Duration, label: &str) -> Option<String> {
    let modified = fs::metadata(path).ok()?.modified().ok()?;
    let age = SystemTime::now().duration_since(modified).ok()?;
    if age < min_age {
        return Some(format!(
            "{label} entries must be older than {} hours before they are eligible",
            min_age.as_secs() / 3600
        ));
    }
    None
}

fn path_has_open_handles(path: &Path) -> bool {
    let lsof_path = Path::new("/usr/sbin/lsof");
    if !lsof_path.is_file() {
        return false;
    }
    let output = if path.is_dir() {
        Command::new(lsof_path)
            .args(["-F", "p", "+D"])
            .arg(path)
            .output()
    } else {
        Command::new(lsof_path)
            .args(["-F", "p", "--"])
            .arg(path)
            .output()
    };
    let Ok(output) = output else {
        return false;
    };
    String::from_utf8_lossy(&output.stdout)
        .lines()
        .any(|line| line.starts_with('p'))
}

fn installed_owner_for_name(name: &str, inventory: &[AppRecord]) -> Option<AppRecord> {
    extract_bundle_id(name)
        .and_then(|bundle_id| installed_owner_for_bundle_id(&bundle_id, inventory))
        .or_else(|| find_unique_app_by_name(inventory, name))
}

fn installed_owner_for_cache_name(name: &str, inventory: &[AppRecord]) -> Option<AppRecord> {
    installed_owner_for_name(name, inventory)
}

fn installed_owner_for_bundle_id(bundle_id: &str, inventory: &[AppRecord]) -> Option<AppRecord> {
    let parts = bundle_id.split('.').collect::<Vec<_>>();
    if parts.len() < 3 {
        return None;
    }
    for end in (3..=parts.len()).rev() {
        let candidate = parts[..end].join(".");
        if let Some(app) = find_app_by_bundle_id(inventory, &candidate) {
            return Some(app);
        }
    }
    None
}

fn bundle_id_has_running_processes(bundle_id: &str, inventory: &[AppRecord]) -> bool {
    installed_owner_for_bundle_id(bundle_id, inventory)
        .map(|app| app_has_running_processes(&app))
        .unwrap_or(false)
}

fn app_has_running_processes(app: &AppRecord) -> bool {
    let executable_root = Path::new(&app.app_path).join("Contents").join("MacOS");
    if !executable_root.exists() {
        return false;
    }
    let executable_root = executable_root.display().to_string();
    let output = match Command::new("/bin/ps")
        .args(["-axo", "pid=,command="])
        .output()
    {
        Ok(output) if output.status.success() || !output.stdout.is_empty() => output,
        _ => return false,
    };
    String::from_utf8_lossy(&output.stdout)
        .lines()
        .filter_map(parse_ps_line)
        .any(|(_, command)| {
            command == executable_root || command.starts_with(&format!("{executable_root}/"))
        })
}

fn parse_ps_line(line: &str) -> Option<(u32, &str)> {
    let trimmed = line.trim_start();
    let pid_end = trimmed.find(char::is_whitespace)?;
    let pid = trimmed[..pid_end].parse::<u32>().ok()?;
    let command = trimmed[pid_end..].trim_start();
    if command.is_empty() {
        return None;
    }
    Some((pid, command))
}

#[cfg(test)]
mod tests {
    use std::fs;

    use serde_json::json;
    use tempfile::TempDir;

    use super::{ApplyRequest, MaxRisk, apply_plan, parse_cleanup_plan_payload};
    use crate::paths::ResolvedPaths;
    use crate::planner::{ScanOptions, ScanProfile, Scope, scan};

    #[test]
    fn dry_run_reports_would_move_items() {
        let temp = TempDir::new().expect("tempdir");
        let home = temp.path();
        fs::create_dir_all(home.join("Library/Developer/Xcode/DerivedData/Fixture"))
            .expect("derived data");
        fs::write(
            home.join("Library/Developer/Xcode/DerivedData/Fixture/blob"),
            "temp",
        )
        .expect("derived data file");
        let paths = ResolvedPaths::for_test_home(home);
        let plan = scan(
            &paths,
            ScanOptions {
                app_target: None,
                profile: ScanProfile::Safe,
                scopes: vec![Scope::Xcode],
            },
        )
        .expect("plan");

        let report = apply_plan(
            &paths,
            ApplyRequest {
                dry_run: true,
                max_risk: MaxRisk::Safe,
                plan,
            },
        )
        .expect("dry run");
        assert!(report.moved_count >= 1);
        assert!(report.items.iter().all(|item| item.outcome == "wouldMove"));
    }

    #[test]
    fn tampered_plan_hash_is_rejected() {
        let temp = TempDir::new().expect("tempdir");
        let home = temp.path();
        fs::create_dir_all(home.join("tmp")).expect("tmp");
        fs::write(home.join("tmp/file.tmp"), "temp").expect("temp file");
        let paths = ResolvedPaths::for_test_home(home);
        let mut plan = scan(
            &paths,
            ScanOptions {
                app_target: None,
                profile: ScanProfile::Safe,
                scopes: vec![Scope::Temp],
            },
        )
        .expect("plan");
        plan.plan_hash = "tampered".to_string();

        let error = apply_plan(
            &paths,
            ApplyRequest {
                dry_run: true,
                max_risk: MaxRisk::Safe,
                plan,
            },
        )
        .expect_err("tampered plan must fail");
        assert_eq!(error.code, crate::error::ErrorCode::PlanInvalid);
    }

    #[test]
    fn reinstalled_app_leftovers_are_blocked_at_apply_time() {
        let temp = TempDir::new().expect("tempdir");
        let home = temp.path();
        fs::create_dir_all(home.join("Library/Caches/com.example.test")).expect("cache dir");
        fs::write(home.join("Library/Caches/com.example.test/blob"), "cache").expect("cache file");
        let paths = ResolvedPaths::for_test_home(home);
        let plan = scan(
            &paths,
            ScanOptions {
                app_target: Some("com.example.test".to_string()),
                profile: ScanProfile::Full,
                scopes: vec![Scope::Apps],
            },
        )
        .expect("plan");

        let app_dir = home.join("Applications/Test.app/Contents");
        fs::create_dir_all(&app_dir).expect("app dir");
        fs::write(
            app_dir.join("Info.plist"),
            r#"<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>com.example.test</string>
<key>CFBundleName</key><string>Test</string>
</dict></plist>"#,
        )
        .expect("plist");

        let report = apply_plan(
            &paths,
            ApplyRequest {
                dry_run: true,
                max_risk: MaxRisk::Dangerous,
                plan,
            },
        )
        .expect("dry-run apply");

        assert_eq!(report.moved_count, 0);
        assert_eq!(report.blocked_count, 1);
        assert!(report.items.iter().all(|item| item.outcome == "blocked"));
    }

    #[test]
    fn parse_cleanup_plan_payload_accepts_scan_envelope() {
        let temp = TempDir::new().expect("tempdir");
        let home = temp.path();
        fs::create_dir_all(home.join("Library/Developer/Xcode/DerivedData/Fixture"))
            .expect("derived data");
        fs::write(
            home.join("Library/Developer/Xcode/DerivedData/Fixture/blob"),
            "temp",
        )
        .expect("derived data file");
        let paths = ResolvedPaths::for_test_home(home);
        let plan = scan(
            &paths,
            ScanOptions {
                app_target: None,
                profile: ScanProfile::Safe,
                scopes: vec![Scope::Xcode],
            },
        )
        .expect("plan");

        let envelope = json!({
            "ok": true,
            "data": {
                "plan": plan.clone(),
                "summary": plan.summary,
                "writtenPlanPath": null,
            },
            "error": null,
            "meta": {
                "tool": "scan.run",
                "elapsed": 1,
            },
        });
        let parsed = parse_cleanup_plan_payload(&serde_json::to_string(&envelope).expect("json"))
            .expect("parsed plan");
        assert_eq!(parsed.plan_hash, plan.plan_hash);
    }
}
