use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

use chrono::{DateTime, Duration as ChronoDuration, Utc};
use rayon::prelude::*;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use walkdir::WalkDir;

use crate::config::load_config;
use crate::error::{AshError, ErrorCode, Result};
use crate::inventory::{AppRecord, find_app_by_bundle_id, load_inventory, read_app_groups};
use crate::paths::ResolvedPaths;
use crate::policy::{CandidateClass, Evidence, RiskLevel, is_protected_absolute_path};

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum ScanProfile {
    Safe,
    Full,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, PartialOrd, Ord)]
#[serde(rename_all = "lowercase")]
pub enum Scope {
    Temp,
    Caches,
    Logs,
    Xcode,
    Homebrew,
    Browsers,
    Apps,
    All,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ScanOptions {
    pub profile: ScanProfile,
    pub scopes: Vec<Scope>,
    pub app_target: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub enum OwnerStatus {
    Exclusive,
    Shared,
    InstalledApp,
    UninstalledApp,
    Unknown,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct RunningProcess {
    pub pid: u32,
    pub command: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct TargetApp {
    pub selector: String,
    pub bundle_id: String,
    pub name: String,
    pub display_name: String,
    pub app_path: Option<String>,
    pub installed: bool,
    pub app_groups: Vec<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct CleanupCandidate {
    pub id: String,
    pub path: String,
    pub canonical_path: String,
    pub class: CandidateClass,
    pub risk: RiskLevel,
    pub size_bytes: u64,
    pub owner_status: OwnerStatus,
    pub evidence: Vec<Evidence>,
    pub running_processes: Vec<RunningProcess>,
    pub eligible: bool,
    pub blocked_reason: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct PlanSummary {
    pub total_candidates: usize,
    pub eligible_candidates: usize,
    pub blocked_candidates: usize,
    pub total_bytes: u64,
    pub eligible_bytes: u64,
    pub by_risk: BTreeMap<String, usize>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct CleanupPlan {
    pub schema_version: String,
    pub created_at: DateTime<Utc>,
    pub profile: ScanProfile,
    pub scopes: Vec<Scope>,
    pub target_app: Option<TargetApp>,
    pub summary: PlanSummary,
    pub candidates: Vec<CleanupCandidate>,
    pub plan_hash: String,
}

#[derive(Serialize)]
#[serde(rename_all = "camelCase")]
struct PlanHashMaterial<'a> {
    schema_version: &'a str,
    created_at: DateTime<Utc>,
    profile: ScanProfile,
    scopes: &'a [Scope],
    target_app: &'a Option<TargetApp>,
    summary: &'a PlanSummary,
    candidates: &'a [CleanupCandidate],
}

struct CandidateDraft {
    canonical_path: String,
    class: CandidateClass,
    risk: RiskLevel,
    size_bytes: u64,
    owner_status: OwnerStatus,
    evidence: Vec<Evidence>,
    running_processes: Vec<RunningProcess>,
    eligible: bool,
    blocked_reason: Option<String>,
}

const TEMP_MIN_AGE_HOURS: i64 = 24;
const CACHE_MIN_AGE_HOURS: i64 = 72;
const LOG_MIN_AGE_HOURS: i64 = 168;
const BROWSER_CACHE_MIN_AGE_HOURS: i64 = 24;

pub fn scan(paths: &ResolvedPaths, options: ScanOptions) -> Result<CleanupPlan> {
    let config = load_config(paths)?;
    let scopes = expand_scopes(options.profile, &options.scopes);
    let inventory = load_inventory(paths, &config)?;
    let target_app = resolve_target_app(paths, &inventory, options.app_target.as_deref())?;
    let mut by_path = BTreeMap::<String, CleanupCandidate>::new();

    let pool = rayon::ThreadPoolBuilder::new()
        .num_threads(config.parallelism)
        .build()
        .map_err(|error| AshError::runtime(format!("failed to build scan thread pool: {error}")))?;
    let scope_results = pool.install(|| {
        scopes
            .par_iter()
            .copied()
            .map(|scope| {
                scan_scope(
                    paths,
                    options.profile,
                    scope,
                    &inventory,
                    target_app.as_ref(),
                )
            })
            .collect::<Vec<_>>()
    });

    for candidates in scope_results {
        for candidate in candidates? {
            by_path
                .entry(candidate.canonical_path.clone())
                .and_modify(|existing| merge_candidate(existing, candidate.clone()))
                .or_insert(candidate);
        }
    }

    let mut candidates = by_path.into_values().collect::<Vec<_>>();
    candidates.sort_by(|left, right| {
        left.risk
            .cmp(&right.risk)
            .then_with(|| right.size_bytes.cmp(&left.size_bytes))
            .then_with(|| left.path.cmp(&right.path))
    });
    let summary = build_summary(&candidates);
    let created_at = Utc::now();
    let mut plan = CleanupPlan {
        schema_version: "1".to_string(),
        created_at,
        profile: options.profile,
        scopes,
        target_app,
        summary,
        candidates,
        plan_hash: String::new(),
    };
    plan.plan_hash = hash_plan(&plan)?;
    Ok(plan)
}

fn expand_scopes(profile: ScanProfile, requested: &[Scope]) -> Vec<Scope> {
    let base = if requested.is_empty() {
        match profile {
            ScanProfile::Safe => vec![
                Scope::Temp,
                Scope::Caches,
                Scope::Logs,
                Scope::Xcode,
                Scope::Homebrew,
                Scope::Browsers,
            ],
            ScanProfile::Full => vec![
                Scope::Temp,
                Scope::Caches,
                Scope::Logs,
                Scope::Xcode,
                Scope::Homebrew,
                Scope::Browsers,
                Scope::Apps,
            ],
        }
    } else {
        requested.to_vec()
    };
    let mut seen = BTreeSet::new();
    let mut scopes = Vec::new();
    for scope in base {
        if scope == Scope::All {
            for expanded in [
                Scope::Temp,
                Scope::Caches,
                Scope::Logs,
                Scope::Xcode,
                Scope::Homebrew,
                Scope::Browsers,
                Scope::Apps,
            ] {
                if seen.insert(expanded) {
                    scopes.push(expanded);
                }
            }
            continue;
        }
        if seen.insert(scope) {
            scopes.push(scope);
        }
    }
    scopes
}

fn resolve_target_app(
    _paths: &ResolvedPaths,
    inventory: &[AppRecord],
    selector: Option<&str>,
) -> Result<Option<TargetApp>> {
    let Some(selector) = selector else {
        return Ok(None);
    };
    if selector.trim().is_empty() {
        return Err(AshError::new(
            ErrorCode::PlanInvalid,
            "empty app selector",
            "pass a bundle id or a concrete .app path",
        ));
    }

    if selector.ends_with(".app") || Path::new(selector).exists() {
        let app_path = PathBuf::from(selector);
        let record = read_app_record_from_path(&app_path).ok_or_else(|| {
            AshError::new(
                ErrorCode::PlanInvalid,
                format!("failed to read app metadata from {}", app_path.display()),
                "pass a valid .app bundle path or a bundle id",
            )
        })?;
        let app_groups = read_app_groups(&app_path);
        return Ok(Some(TargetApp {
            app_groups,
            app_path: Some(app_path.display().to_string()),
            bundle_id: record.bundle_id.clone(),
            display_name: record.display_name.clone(),
            installed: inventory
                .iter()
                .any(|app| app.bundle_id == record.bundle_id),
            name: record.name.clone(),
            selector: selector.to_string(),
        }));
    }

    let bundle_id = selector.trim().to_string();
    if bundle_id.starts_with("com.apple.") {
        return Ok(Some(TargetApp {
            app_groups: Vec::new(),
            app_path: None,
            bundle_id: bundle_id.clone(),
            display_name: derive_display_name(&bundle_id),
            installed: true,
            name: derive_display_name(&bundle_id),
            selector: selector.to_string(),
        }));
    }

    if let Some(app) = find_app_by_bundle_id(inventory, &bundle_id) {
        let app_path = PathBuf::from(&app.app_path);
        return Ok(Some(TargetApp {
            app_groups: read_app_groups(&app_path),
            app_path: Some(app.app_path),
            bundle_id: app.bundle_id,
            display_name: app.display_name,
            installed: true,
            name: app.name,
            selector: selector.to_string(),
        }));
    }

    Ok(Some(TargetApp {
        app_groups: Vec::new(),
        app_path: None,
        bundle_id: bundle_id.clone(),
        display_name: derive_display_name(&bundle_id),
        installed: false,
        name: derive_display_name(&bundle_id),
        selector: selector.to_string(),
    }))
}

fn scan_scope(
    paths: &ResolvedPaths,
    profile: ScanProfile,
    scope: Scope,
    inventory: &[AppRecord],
    target_app: Option<&TargetApp>,
) -> Result<Vec<CleanupCandidate>> {
    match scope {
        Scope::Temp => scan_temp(paths, inventory),
        Scope::Caches => scan_caches(paths, inventory),
        Scope::Logs => scan_logs(paths),
        Scope::Xcode => scan_xcode(paths, profile),
        Scope::Homebrew => scan_homebrew(paths),
        Scope::Browsers => scan_browsers(paths, inventory),
        Scope::Apps => scan_app_state(paths, inventory, target_app),
        Scope::All => Ok(Vec::new()),
    }
}

fn scan_temp(paths: &ResolvedPaths, inventory: &[AppRecord]) -> Result<Vec<CleanupCandidate>> {
    let mut candidates = Vec::new();
    let base = &paths.user_temp_dir;
    let base_display = base.display().to_string();
    let Ok(entries) = fs::read_dir(base) else {
        return Ok(candidates);
    };
    for entry in entries.flatten() {
        let path = entry.path();
        let name = entry.file_name().to_string_lossy().to_string();
        let canonical = canonical_string(&path);
        if path.is_symlink_safe()
            || is_protected_absolute_path(&canonical, &paths.user_home.display().to_string())
        {
            continue;
        }
        let owner = installed_owner_for_name(&name, inventory);
        let blocked_reason = generic_temp_block_reason(&name, &path, owner.as_ref());
        let size_bytes = calculate_size(&path)?;
        candidates.push(build_candidate(CandidateDraft {
            canonical_path: canonical,
            class: CandidateClass::TempPath,
            risk: risk_for_temp_name(&name),
            size_bytes,
            owner_status: owner
                .as_ref()
                .map(|_| OwnerStatus::InstalledApp)
                .unwrap_or(OwnerStatus::Unknown),
            evidence: vec![
                Evidence::new("scope", CandidateClass::TempPath.category()),
                Evidence::new("basePath", base_display.clone()),
            ],
            running_processes: Vec::new(),
            eligible: blocked_reason.is_none(),
            blocked_reason,
        }));
    }
    Ok(candidates)
}

fn scan_logs(paths: &ResolvedPaths) -> Result<Vec<CleanupCandidate>> {
    let mut candidates = Vec::new();
    let base = paths.user_home.join("Library").join("Logs");
    let base_display = base.display().to_string();
    let Ok(entries) = fs::read_dir(&base) else {
        return Ok(candidates);
    };
    for entry in entries.flatten() {
        let path = entry.path();
        let name = entry.file_name().to_string_lossy().to_string();
        let canonical = canonical_string(&path);
        if path.is_symlink_safe()
            || is_protected_absolute_path(&canonical, &paths.user_home.display().to_string())
        {
            continue;
        }
        let blocked_reason = min_age_block_reason(&path, LOG_MIN_AGE_HOURS, "log");
        let size_bytes = calculate_size(&path)?;
        candidates.push(build_candidate(CandidateDraft {
            canonical_path: canonical,
            class: CandidateClass::UserLog,
            risk: risk_for_log_name(&name),
            size_bytes,
            owner_status: OwnerStatus::Unknown,
            evidence: vec![
                Evidence::new("scope", CandidateClass::UserLog.category()),
                Evidence::new("basePath", base_display.clone()),
            ],
            running_processes: Vec::new(),
            eligible: blocked_reason.is_none(),
            blocked_reason,
        }));
    }
    Ok(candidates)
}

fn scan_caches(paths: &ResolvedPaths, inventory: &[AppRecord]) -> Result<Vec<CleanupCandidate>> {
    let browser_roots = browser_paths(paths);
    let homebrew_root = paths.user_cache_dir.join("Homebrew");
    let mut candidates = Vec::new();
    let Ok(entries) = fs::read_dir(&paths.user_cache_dir) else {
        return Ok(candidates);
    };
    for entry in entries.flatten() {
        let path = entry.path();
        if path.is_symlink_safe() {
            continue;
        }
        if path == homebrew_root || browser_roots.iter().any(|root| root.path == path) {
            continue;
        }
        let canonical = canonical_string(&path);
        if is_protected_absolute_path(&canonical, &paths.user_home.display().to_string()) {
            continue;
        }
        let name = entry.file_name().to_string_lossy().to_string();
        let size_bytes = calculate_size(&path)?;
        let risk = risk_for_cache_like_name(&name, CandidateClass::UserCache);
        let owner = installed_owner_for_name(&name, inventory);
        let blocked_reason = generic_cache_block_reason(&name, &path, owner.as_ref());
        candidates.push(build_candidate(CandidateDraft {
            canonical_path: canonical,
            class: CandidateClass::UserCache,
            risk,
            size_bytes,
            owner_status: classify_owner_status_from_name(&name, owner.as_ref()),
            evidence: vec![
                Evidence::new("scope", "caches"),
                Evidence::new("cacheName", name),
            ],
            running_processes: Vec::new(),
            eligible: blocked_reason.is_none(),
            blocked_reason,
        }));
    }
    Ok(candidates)
}

fn scan_homebrew(paths: &ResolvedPaths) -> Result<Vec<CleanupCandidate>> {
    let path = paths.user_cache_dir.join("Homebrew");
    if !path.exists() || path.is_symlink_safe() {
        return Ok(Vec::new());
    }
    let size_bytes = calculate_size(&path)?;
    Ok(vec![build_candidate(CandidateDraft {
        canonical_path: canonical_string(&path),
        class: CandidateClass::HomebrewCache,
        risk: CandidateClass::HomebrewCache.default_risk(),
        size_bytes,
        owner_status: OwnerStatus::Unknown,
        evidence: vec![
            Evidence::new("scope", "homebrew"),
            Evidence::new("strategy", "report-only until brew cleanup execution is implemented"),
        ],
        running_processes: Vec::new(),
        eligible: false,
        blocked_reason: Some(
            "Homebrew cleanup should execute via `brew cleanup --dry-run` or `brew cleanup`, not by trashing the cache directory directly".to_string(),
        ),
    })])
}

fn scan_browsers(paths: &ResolvedPaths, inventory: &[AppRecord]) -> Result<Vec<CleanupCandidate>> {
    let mut candidates = Vec::new();
    for browser in browser_paths(paths) {
        if !browser.path.exists() || browser.path.is_symlink_safe() {
            continue;
        }
        let size_bytes = calculate_size(&browser.path)?;
        let owner = installed_owner_for_bundle_id(browser.bundle_id, inventory);
        let running_processes = owner
            .as_ref()
            .map(detect_running_processes_for_app)
            .unwrap_or_default();
        let blocked_reason =
            browser_cache_block_reason(&browser.path, owner.as_ref(), &running_processes);
        candidates.push(build_candidate(CandidateDraft {
            canonical_path: canonical_string(&browser.path),
            class: CandidateClass::BrowserDiskCache,
            risk: CandidateClass::BrowserDiskCache.default_risk(),
            size_bytes,
            owner_status: owner
                .as_ref()
                .map(|_| OwnerStatus::InstalledApp)
                .unwrap_or(OwnerStatus::Unknown),
            evidence: vec![
                Evidence::new("scope", "browsers"),
                Evidence::new("bundleId", browser.bundle_id),
                Evidence::new("path", browser.path.display().to_string()),
            ],
            running_processes,
            eligible: blocked_reason.is_none(),
            blocked_reason,
        }));
    }
    Ok(candidates)
}

#[derive(Clone)]
struct BrowserRoot {
    path: PathBuf,
    bundle_id: &'static str,
}

fn browser_paths(paths: &ResolvedPaths) -> Vec<BrowserRoot> {
    let cache = &paths.user_cache_dir;
    vec![
        BrowserRoot {
            path: cache.join("com.apple.Safari"),
            bundle_id: "com.apple.Safari",
        },
        BrowserRoot {
            path: cache.join("Google").join("Chrome"),
            bundle_id: "com.google.Chrome",
        },
        BrowserRoot {
            path: cache.join("Firefox"),
            bundle_id: "org.mozilla.firefox",
        },
        BrowserRoot {
            path: cache.join("org.mozilla.firefox"),
            bundle_id: "org.mozilla.firefox",
        },
        BrowserRoot {
            path: cache.join("com.brave.Browser"),
            bundle_id: "com.brave.Browser",
        },
        BrowserRoot {
            path: cache.join("com.microsoft.edgemac"),
            bundle_id: "com.microsoft.edgemac",
        },
        BrowserRoot {
            path: cache.join("com.operasoftware.Opera"),
            bundle_id: "com.operasoftware.Opera",
        },
    ]
}

fn scan_xcode(paths: &ResolvedPaths, profile: ScanProfile) -> Result<Vec<CleanupCandidate>> {
    let developer = paths.user_home.join("Library").join("Developer");
    let roots = match profile {
        ScanProfile::Safe => vec![(
            developer.join("Xcode").join("DerivedData"),
            CandidateClass::XcodeDerivedData,
            None,
        )],
        ScanProfile::Full => vec![
            (
                developer.join("Xcode").join("DerivedData"),
                CandidateClass::XcodeDerivedData,
                None,
            ),
            (
                developer.join("Xcode").join("iOS DeviceSupport"),
                CandidateClass::XcodeDeviceSupport,
                Some(
                    "device support cleanup is review-only and requires manual confirmation in a future execution path".to_string(),
                ),
            ),
            (
                developer.join("Xcode").join("Archives"),
                CandidateClass::XcodeArchives,
                Some(
                    "Xcode archives are report-only in v1 because they may be required for symbolication or release history".to_string(),
                ),
            ),
            (
                developer.join("CoreSimulator").join("Devices"),
                CandidateClass::SimulatorDeviceSet,
                Some(
                    "simulator device trees are reported but not eligible in generic plans".to_string(),
                ),
            ),
        ],
    };

    let mut candidates = Vec::new();
    for (base, class, blocked_reason) in roots {
        let Ok(entries) = fs::read_dir(&base) else {
            continue;
        };
        for entry in entries.flatten() {
            let path = entry.path();
            if path.is_symlink_safe() {
                continue;
            }
            let size_bytes = calculate_size(&path)?;
            let reason = blocked_reason.clone();
            candidates.push(build_candidate(CandidateDraft {
                canonical_path: canonical_string(&path),
                class,
                risk: class.default_risk(),
                size_bytes,
                owner_status: OwnerStatus::Unknown,
                evidence: vec![
                    Evidence::new("scope", "xcode"),
                    Evidence::new("basePath", base.display().to_string()),
                ],
                running_processes: Vec::new(),
                eligible: reason.is_none(),
                blocked_reason: reason,
            }));
        }
    }
    Ok(candidates)
}

fn scan_app_state(
    paths: &ResolvedPaths,
    inventory: &[AppRecord],
    target_app: Option<&TargetApp>,
) -> Result<Vec<CleanupCandidate>> {
    let mut candidates = Vec::new();
    let home = paths.user_home.display().to_string();
    let locations = vec![
        (
            paths.user_home.join("Library").join("Application Support"),
            CandidateClass::ApplicationSupport,
        ),
        (
            paths.user_home.join("Library").join("Preferences"),
            CandidateClass::PreferencePlist,
        ),
        (
            paths.user_home.join("Library").join("Containers"),
            CandidateClass::SandboxContainer,
        ),
        (
            paths.user_home.join("Library").join("Group Containers"),
            CandidateClass::GroupContainer,
        ),
        (
            paths
                .user_home
                .join("Library")
                .join("Saved Application State"),
            CandidateClass::SavedApplicationState,
        ),
        (
            paths.user_home.join("Library").join("WebKit"),
            CandidateClass::WebKitData,
        ),
        (
            paths.user_home.join("Library").join("HTTPStorages"),
            CandidateClass::HttpStorage,
        ),
        (
            paths.user_home.join("Library").join("Cookies"),
            CandidateClass::CookieStore,
        ),
        (
            paths.user_home.join("Library").join("LaunchAgents"),
            CandidateClass::LaunchAgent,
        ),
    ];

    if let Some(target) = target_app {
        let running = target_running_processes(target);
        for (base, class) in locations {
            let Ok(entries) = fs::read_dir(&base) else {
                continue;
            };
            for entry in entries.flatten() {
                let name = entry.file_name().to_string_lossy().to_string();
                if let Some(candidate) = targeted_state_candidate(
                    paths,
                    target,
                    class,
                    &base,
                    &entry.path(),
                    &name,
                    &running,
                )? {
                    candidates.push(candidate);
                }
            }
        }

        for (base, class) in [
            (
                paths.user_cache_dir.clone(),
                CandidateClass::AppCacheLeftover,
            ),
            (
                paths.user_home.join("Library").join("Logs"),
                CandidateClass::AppLogLeftover,
            ),
        ] {
            let Ok(entries) = fs::read_dir(&base) else {
                continue;
            };
            for entry in entries.flatten() {
                let path = entry.path();
                if path.is_symlink_safe() {
                    continue;
                }
                let name = entry.file_name().to_string_lossy().to_string();
                if let Some(bundle_id) = extract_bundle_id(&name)
                    && bundle_id == target.bundle_id
                {
                    let size_bytes = calculate_size(&path)?;
                    let blocked_reason = targeted_leftover_block_reason(target, &running);
                    candidates.push(build_candidate(CandidateDraft {
                        canonical_path: canonical_string(&path),
                        class,
                        risk: class.default_risk(),
                        size_bytes,
                        owner_status: OwnerStatus::Exclusive,
                        evidence: vec![
                            Evidence::new("bundleId", target.bundle_id.clone()),
                            Evidence::new("targetedApp", target.selector.clone()),
                        ],
                        running_processes: running.clone(),
                        eligible: blocked_reason.is_none(),
                        blocked_reason,
                    }));
                }
            }
        }
        return Ok(candidates);
    }

    for (base, class) in locations {
        if class == CandidateClass::GroupContainer {
            continue;
        }
        let Ok(entries) = fs::read_dir(&base) else {
            continue;
        };
        for entry in entries.flatten() {
            let path = entry.path();
            if path.is_symlink_safe() {
                continue;
            }
            let name = entry.file_name().to_string_lossy().to_string();
            let Some(bundle_id) = extract_bundle_id(&name) else {
                continue;
            };
            if bundle_id.starts_with("com.apple.") {
                continue;
            }
            if installed_owner_for_bundle_id(&bundle_id, inventory).is_some() {
                continue;
            }
            let canonical = canonical_string(&path);
            if is_protected_absolute_path(&canonical, &home) {
                continue;
            }
            let size_bytes = calculate_size(&path)?;
            candidates.push(build_candidate(CandidateDraft {
                canonical_path: canonical,
                class,
                risk: class.default_risk(),
                size_bytes,
                owner_status: OwnerStatus::UninstalledApp,
                evidence: vec![
                    Evidence::new("bundleId", bundle_id),
                    Evidence::new(
                        "installedStatus",
                        "no installed app with this bundle id was found",
                    ),
                ],
                running_processes: Vec::new(),
                eligible: false,
                blocked_reason: Some(
                    "stateful app cleanup requires a targeted app plan in v1".to_string(),
                ),
            }));
        }
    }
    Ok(candidates)
}

fn targeted_state_candidate(
    paths: &ResolvedPaths,
    target: &TargetApp,
    class: CandidateClass,
    base: &Path,
    path: &Path,
    name: &str,
    running: &[RunningProcess],
) -> Result<Option<CleanupCandidate>> {
    if path.is_symlink_safe() {
        return Ok(None);
    }
    let canonical = canonical_string(path);
    if is_protected_absolute_path(&canonical, &paths.user_home.display().to_string()) {
        return Ok(None);
    }

    let exact_bundle_match =
        extract_bundle_id(name).is_some_and(|bundle_id| bundle_id == target.bundle_id);
    let exact_name_match = normalize(name) == normalize(&target.display_name)
        || normalize(name) == normalize(&target.name);
    let matched = exact_bundle_match || exact_name_match;
    if !matched {
        return Ok(None);
    }

    let size_bytes = calculate_size(path)?;
    let mut eligible = exact_bundle_match && !target.installed && running.is_empty();
    let mut owner_status = if exact_bundle_match {
        OwnerStatus::Exclusive
    } else {
        OwnerStatus::Unknown
    };
    let mut blocked_reason = if exact_bundle_match {
        None
    } else {
        Some("bundle id evidence is required for destructive app-state cleanup".to_string())
    };

    if target.bundle_id.starts_with("com.apple.") {
        eligible = false;
        blocked_reason = Some("Apple app state is blocked from targeted cleanup".to_string());
        owner_status = OwnerStatus::InstalledApp;
    } else if target.installed {
        eligible = false;
        blocked_reason = Some("installed app state purge is disabled in v1".to_string());
        owner_status = OwnerStatus::InstalledApp;
    } else if !running.is_empty() {
        eligible = false;
        blocked_reason = Some("the targeted app still appears to be running".to_string());
    }

    if class == CandidateClass::GroupContainer {
        if target.app_groups.iter().any(|group| group == name) {
            owner_status = OwnerStatus::Exclusive;
            blocked_reason = if target.installed || !running.is_empty() {
                blocked_reason
            } else {
                None
            };
            eligible = !target.installed && running.is_empty();
        } else {
            owner_status = OwnerStatus::Shared;
            eligible = false;
            blocked_reason = Some(
                "group container ownership is not proven by app-group entitlements".to_string(),
            );
        }
    }

    Ok(Some(build_candidate(CandidateDraft {
        canonical_path: canonical,
        class,
        risk: class.default_risk(),
        size_bytes,
        owner_status,
        evidence: vec![
            Evidence::new("targetedApp", target.selector.clone()),
            Evidence::new("bundleId", target.bundle_id.clone()),
            Evidence::new("basePath", base.display().to_string()),
        ],
        running_processes: running.to_vec(),
        eligible,
        blocked_reason,
    })))
}

fn build_candidate(draft: CandidateDraft) -> CleanupCandidate {
    let mut hasher = Sha256::new();
    hasher.update(draft.canonical_path.as_bytes());
    hasher.update(draft.class.category().as_bytes());
    let id = hex::encode(hasher.finalize());
    CleanupCandidate {
        id,
        path: draft.canonical_path.clone(),
        canonical_path: draft.canonical_path,
        class: draft.class,
        risk: draft.risk,
        size_bytes: draft.size_bytes,
        owner_status: draft.owner_status,
        evidence: draft.evidence,
        running_processes: draft.running_processes,
        eligible: draft.eligible,
        blocked_reason: draft.blocked_reason,
    }
}

fn risk_for_cache_like_name(name: &str, default_class: CandidateClass) -> RiskLevel {
    let lower = name.to_ascii_lowercase();
    if lower.starts_with("com.apple.") || lower == "cloudkit" || lower == "geoservices" {
        RiskLevel::Review
    } else {
        default_class.default_risk()
    }
}

fn risk_for_log_name(name: &str) -> RiskLevel {
    let lower = name.to_ascii_lowercase();
    if lower.contains("diagnostic") || lower.contains("crash") {
        RiskLevel::Review
    } else {
        CandidateClass::UserLog.default_risk()
    }
}

fn canonical_string(path: &Path) -> String {
    fs::canonicalize(path)
        .unwrap_or_else(|_| path.to_path_buf())
        .display()
        .to_string()
}

fn calculate_size(path: &Path) -> Result<u64> {
    if path.is_symlink_safe() {
        return Ok(0);
    }
    let metadata = match fs::metadata(path) {
        Ok(metadata) => metadata,
        Err(error)
            if matches!(
                error.kind(),
                std::io::ErrorKind::PermissionDenied | std::io::ErrorKind::NotFound
            ) =>
        {
            return Ok(0);
        }
        Err(error) => return Err(error.into()),
    };
    if metadata.is_file() {
        return Ok(metadata.len());
    }
    let mut total = 0u64;
    for entry in WalkDir::new(path).follow_links(false).into_iter().flatten() {
        if entry.file_type().is_file() {
            total =
                total.saturating_add(entry.metadata().map(|metadata| metadata.len()).unwrap_or(0));
        }
    }
    Ok(total)
}

fn hash_plan(plan: &CleanupPlan) -> Result<String> {
    let material = PlanHashMaterial {
        schema_version: &plan.schema_version,
        created_at: plan.created_at,
        profile: plan.profile,
        scopes: &plan.scopes,
        target_app: &plan.target_app,
        summary: &plan.summary,
        candidates: &plan.candidates,
    };
    let payload = serde_json::to_vec(&material).map_err(|error| {
        AshError::runtime(format!("failed to serialize plan hash material: {error}"))
    })?;
    let digest = Sha256::digest(payload);
    Ok(hex::encode(digest))
}

pub fn verify_plan_hash(plan: &CleanupPlan) -> Result<()> {
    let expected = hash_plan(plan)?;
    if expected != plan.plan_hash {
        return Err(AshError::new(
            ErrorCode::PlanInvalid,
            format!(
                "plan hash mismatch: expected {expected}, got {}",
                plan.plan_hash
            ),
            "re-run `ash scan` to regenerate the plan before applying it",
        ));
    }
    Ok(())
}

fn build_summary(candidates: &[CleanupCandidate]) -> PlanSummary {
    let mut by_risk = BTreeMap::new();
    let mut total_bytes = 0u64;
    let mut eligible_bytes = 0u64;
    let mut eligible_candidates = 0usize;
    for candidate in candidates {
        *by_risk
            .entry(candidate.risk.as_str().to_string())
            .or_insert(0usize) += 1;
        total_bytes = total_bytes.saturating_add(candidate.size_bytes);
        if candidate.eligible {
            eligible_candidates += 1;
            eligible_bytes = eligible_bytes.saturating_add(candidate.size_bytes);
        }
    }
    PlanSummary {
        blocked_candidates: candidates.len().saturating_sub(eligible_candidates),
        by_risk,
        eligible_bytes,
        eligible_candidates,
        total_bytes,
        total_candidates: candidates.len(),
    }
}

fn merge_candidate(existing: &mut CleanupCandidate, candidate: CleanupCandidate) {
    let existing_specific = existing.owner_status != OwnerStatus::Unknown;
    let candidate_specific = candidate.owner_status != OwnerStatus::Unknown;
    if !existing_specific && candidate_specific {
        *existing = candidate;
        return;
    }
    if candidate.risk > existing.risk {
        *existing = candidate;
        return;
    }
    if candidate.evidence.len() > existing.evidence.len() {
        *existing = candidate;
    }
}

fn min_age_block_reason(path: &Path, min_age_hours: i64, label: &str) -> Option<String> {
    let modified = fs::metadata(path).ok()?.modified().ok()?;
    let modified_at = DateTime::<Utc>::from(modified);
    let min_age = ChronoDuration::hours(min_age_hours);
    let age = Utc::now().signed_duration_since(modified_at);
    if age < min_age {
        return Some(format!(
            "{label} entries must be older than {min_age_hours} hours before they are eligible"
        ));
    }
    None
}

fn installed_owner_for_name(name: &str, inventory: &[AppRecord]) -> Option<AppRecord> {
    extract_bundle_id(name)
        .and_then(|bundle_id| installed_owner_for_bundle_id(&bundle_id, inventory))
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

fn classify_owner_status_from_name(name: &str, installed_owner: Option<&AppRecord>) -> OwnerStatus {
    if installed_owner.is_some() {
        return OwnerStatus::InstalledApp;
    }
    if extract_bundle_id(name).is_some() {
        return OwnerStatus::UninstalledApp;
    }
    OwnerStatus::Unknown
}

fn generic_temp_block_reason(
    name: &str,
    path: &Path,
    installed_owner: Option<&AppRecord>,
) -> Option<String> {
    if name.starts_with("com.apple.") {
        return Some("Apple-owned Darwin temp entries are report-only in v1".to_string());
    }
    if let Some(owner) = installed_owner {
        return Some(format!(
            "installed app temp entries are not eligible in generic scans: {}",
            owner.bundle_id
        ));
    }
    min_age_block_reason(path, TEMP_MIN_AGE_HOURS, "temp")
}

fn generic_cache_block_reason(
    name: &str,
    path: &Path,
    installed_owner: Option<&AppRecord>,
) -> Option<String> {
    if name.starts_with("com.apple.") {
        return Some("Apple-owned cache entries are report-only in generic scans".to_string());
    }
    if let Some(owner) = installed_owner {
        return Some(format!(
            "installed app caches are not eligible in generic scans: {}",
            owner.bundle_id
        ));
    }
    min_age_block_reason(path, CACHE_MIN_AGE_HOURS, "cache")
}

fn browser_cache_block_reason(
    path: &Path,
    installed_owner: Option<&AppRecord>,
    running_processes: &[RunningProcess],
) -> Option<String> {
    if installed_owner.is_some() && !running_processes.is_empty() {
        return Some(
            "browser cache cleanup is blocked while the browser appears to be running".to_string(),
        );
    }
    min_age_block_reason(path, BROWSER_CACHE_MIN_AGE_HOURS, "browser cache")
}

fn targeted_leftover_block_reason(
    target: &TargetApp,
    running_processes: &[RunningProcess],
) -> Option<String> {
    if target.bundle_id.starts_with("com.apple.") {
        return Some("Apple app cache cleanup is blocked from targeted cleanup".to_string());
    }
    if target.installed {
        return Some("installed app cache cleanup is disabled in v1".to_string());
    }
    if !running_processes.is_empty() {
        return Some("the targeted app still appears to be running".to_string());
    }
    None
}

fn normalize(value: &str) -> String {
    value
        .trim()
        .to_ascii_lowercase()
        .replace(['-', '_', '.'], " ")
        .split_whitespace()
        .collect::<Vec<_>>()
        .join(" ")
}

fn detect_running_processes(app_path: &Path) -> Vec<RunningProcess> {
    let executable_root = app_path.join("Contents").join("MacOS");
    if !executable_root.exists() {
        return Vec::new();
    }
    let executable_root = executable_root.display().to_string();
    let output = match Command::new("/bin/ps")
        .args(["-axo", "pid=,command="])
        .output()
    {
        Ok(output) if output.status.success() || !output.stdout.is_empty() => output,
        _ => return Vec::new(),
    };
    String::from_utf8_lossy(&output.stdout)
        .lines()
        .filter_map(parse_ps_line)
        .filter(|(_, command)| {
            command == &executable_root || command.starts_with(&format!("{executable_root}/"))
        })
        .map(|(pid, command)| RunningProcess {
            pid,
            command: command.to_string(),
        })
        .collect()
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

fn detect_running_processes_for_app(app: &AppRecord) -> Vec<RunningProcess> {
    detect_running_processes(Path::new(&app.app_path))
}

fn target_running_processes(target: &TargetApp) -> Vec<RunningProcess> {
    target
        .app_path
        .as_ref()
        .map(|path| detect_running_processes(Path::new(path)))
        .unwrap_or_default()
}

fn extract_bundle_id(name: &str) -> Option<String> {
    let trimmed = strip_known_suffixes(name);
    if trimmed.starts_with("group.") {
        let candidate = trimmed.trim_start_matches("group.");
        if looks_like_bundle_id(candidate) {
            return Some(candidate.to_string());
        }
    }

    let parts = trimmed.split('.').collect::<Vec<_>>();
    if parts.len() >= 4 && looks_like_team_id(parts[0]) {
        if parts[1] == "groups" {
            let candidate = parts[2..].join(".");
            if looks_like_bundle_id(&candidate) {
                return Some(candidate);
            }
        }
        let candidate = parts[1..].join(".");
        if looks_like_bundle_id(&candidate) {
            return Some(candidate);
        }
    }

    if looks_like_bundle_id(trimmed) {
        return Some(trimmed.to_string());
    }
    for index in 1..parts.len() {
        let candidate = parts[index..].join(".");
        if looks_like_bundle_id(&candidate) {
            return Some(candidate);
        }
    }
    None
}

fn strip_known_suffixes(name: &str) -> &str {
    for suffix in [".plist", ".savedstate", ".savedState", ".binarycookies"] {
        if let Some(value) = name.strip_suffix(suffix) {
            return value;
        }
    }
    name
}

fn looks_like_bundle_id(value: &str) -> bool {
    let parts = value.split('.').collect::<Vec<_>>();
    if parts.len() < 3
        || parts
            .first()
            .is_none_or(|first| first.to_ascii_lowercase() != *first)
    {
        return false;
    }
    parts.iter().all(|part| {
        !part.is_empty()
            && part.chars().all(|character| {
                character.is_ascii_alphanumeric() || character == '-' || character == '_'
            })
    })
}

fn looks_like_team_id(value: &str) -> bool {
    value.len() >= 4
        && value
            .chars()
            .all(|character| character.is_ascii_uppercase() || character.is_ascii_digit())
}

fn derive_display_name(bundle_id: &str) -> String {
    bundle_id
        .split('.')
        .next_back()
        .unwrap_or(bundle_id)
        .replace(['-', '_'], " ")
}

fn risk_for_temp_name(name: &str) -> RiskLevel {
    let lower = name.to_ascii_lowercase();
    if lower.starts_with("com.apple.") {
        RiskLevel::Review
    } else {
        risk_for_cache_like_name(name, CandidateClass::TempPath)
    }
}

fn read_app_record_from_path(path: &Path) -> Option<AppRecord> {
    let info_path = path.join("Contents").join("Info.plist");
    let value = plist::Value::from_file(info_path).ok()?;
    let dict = value.as_dictionary()?;
    let bundle_id = dict.get("CFBundleIdentifier")?.as_string()?.to_string();
    let name = dict
        .get("CFBundleName")
        .and_then(plist::Value::as_string)
        .map(ToString::to_string)
        .unwrap_or_else(|| derive_display_name(&bundle_id));
    let display_name = dict
        .get("CFBundleDisplayName")
        .and_then(plist::Value::as_string)
        .map(ToString::to_string)
        .unwrap_or_else(|| name.clone());
    Some(AppRecord {
        bundle_id,
        name,
        display_name,
        app_path: path.display().to_string(),
    })
}

trait PathSafetyExt {
    fn is_symlink_safe(&self) -> bool;
}

impl PathSafetyExt for Path {
    fn is_symlink_safe(&self) -> bool {
        fs::symlink_metadata(self)
            .map(|metadata| metadata.file_type().is_symlink())
            .unwrap_or(false)
    }
}

#[cfg(test)]
mod tests {
    use std::fs;

    use tempfile::TempDir;

    use super::{OwnerStatus, ScanOptions, ScanProfile, Scope, scan};
    use crate::paths::ResolvedPaths;
    use crate::policy::CandidateClass;

    #[test]
    fn scan_fixture_safe_profile() {
        let temp = TempDir::new().expect("tempdir");
        let home = temp.path();
        fs::create_dir_all(home.join("tmp")).expect("tmp");
        fs::create_dir_all(home.join("Library/Caches/com.example.cache")).expect("cache");
        fs::create_dir_all(home.join("Library/Logs/Example")).expect("logs");
        fs::write(home.join("tmp/file.tmp"), "temp").expect("temp file");
        fs::write(home.join("Library/Caches/com.example.cache/blob"), "cache").expect("cache blob");
        fs::write(home.join("Library/Logs/Example/app.log"), "log").expect("log file");

        let paths = ResolvedPaths::for_test_home(home);
        let plan = scan(
            &paths,
            ScanOptions {
                app_target: None,
                profile: ScanProfile::Safe,
                scopes: vec![Scope::Temp, Scope::Caches, Scope::Logs],
            },
        )
        .expect("safe scan");
        assert!(plan.summary.total_candidates >= 3);
        assert!(!plan.plan_hash.is_empty());
    }

    #[test]
    fn targeted_app_scan_reports_dangerous_state_as_blocked_when_installed() {
        let temp = TempDir::new().expect("tempdir");
        let home = temp.path();
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
        fs::create_dir_all(home.join("Library/Preferences")).expect("prefs dir");
        fs::write(
            home.join("Library/Preferences/com.example.test.plist"),
            "plist",
        )
        .expect("prefs");

        let paths = ResolvedPaths::for_test_home(home);
        let plan = scan(
            &paths,
            ScanOptions {
                app_target: Some("com.example.test".to_string()),
                profile: ScanProfile::Full,
                scopes: vec![Scope::Apps],
            },
        )
        .expect("app scan");
        assert!(plan.candidates.iter().any(|candidate| !candidate.eligible));
    }

    #[test]
    fn targeted_full_scan_dedupes_cache_and_app_candidates_by_canonical_path() {
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
                scopes: vec![Scope::Caches, Scope::Apps],
            },
        )
        .expect("deduped scan");

        let unique_paths = plan
            .candidates
            .iter()
            .map(|candidate| candidate.canonical_path.clone())
            .collect::<std::collections::BTreeSet<_>>();
        assert_eq!(unique_paths.len(), plan.candidates.len());
    }

    #[test]
    fn targeted_app_cache_leftovers_are_blocked_when_app_is_installed() {
        let temp = TempDir::new().expect("tempdir");
        let home = temp.path();
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
        .expect("app scan");

        let candidate = plan
            .candidates
            .iter()
            .find(|candidate| candidate.class == CandidateClass::AppCacheLeftover)
            .expect("cache leftover candidate");
        assert!(!candidate.eligible);
        assert_eq!(candidate.owner_status, OwnerStatus::Exclusive);
    }

    #[test]
    fn generic_group_containers_are_not_reported() {
        let temp = TempDir::new().expect("tempdir");
        let home = temp.path();
        fs::create_dir_all(home.join("Library/Group Containers/group.com.example.fixture"))
            .expect("group container");
        fs::write(
            home.join("Library/Group Containers/group.com.example.fixture/shared.db"),
            "shared",
        )
        .expect("shared data");

        let paths = ResolvedPaths::for_test_home(home);
        let plan = scan(
            &paths,
            ScanOptions {
                app_target: None,
                profile: ScanProfile::Full,
                scopes: vec![Scope::Apps],
            },
        )
        .expect("full scan");

        assert!(
            !plan
                .candidates
                .iter()
                .any(|candidate| candidate.class == CandidateClass::GroupContainer)
        );
    }

    #[test]
    fn generic_installed_app_cache_is_reported_but_not_eligible() {
        let temp = TempDir::new().expect("tempdir");
        let home = temp.path();
        let app_dir = home.join("Applications/Cache Test.app/Contents");
        fs::create_dir_all(&app_dir).expect("app dir");
        fs::write(
            app_dir.join("Info.plist"),
            r#"<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>com.example.cache</string>
<key>CFBundleName</key><string>Cache Test</string>
</dict></plist>"#,
        )
        .expect("plist");
        fs::create_dir_all(home.join("Library/Caches/com.example.cache")).expect("cache dir");
        fs::write(home.join("Library/Caches/com.example.cache/blob"), "cache").expect("cache file");

        let paths = ResolvedPaths::for_test_home(home);
        let plan = scan(
            &paths,
            ScanOptions {
                app_target: None,
                profile: ScanProfile::Safe,
                scopes: vec![Scope::Caches],
            },
        )
        .expect("cache scan");

        let candidate = plan
            .candidates
            .iter()
            .find(|candidate| candidate.class == CandidateClass::UserCache)
            .expect("cache candidate");
        assert!(!candidate.eligible);
        assert_eq!(candidate.owner_status, OwnerStatus::InstalledApp);
    }

    #[test]
    fn safe_xcode_scan_only_reports_derived_data() {
        let temp = TempDir::new().expect("tempdir");
        let home = temp.path();
        fs::create_dir_all(home.join("Library/Developer/Xcode/DerivedData/Fixture"))
            .expect("derived data");
        fs::create_dir_all(home.join("Library/Developer/Xcode/iOS DeviceSupport/Fixture"))
            .expect("device support");
        fs::write(
            home.join("Library/Developer/Xcode/DerivedData/Fixture/blob"),
            "data",
        )
        .expect("derived data file");
        fs::write(
            home.join("Library/Developer/Xcode/iOS DeviceSupport/Fixture/blob"),
            "data",
        )
        .expect("device support file");

        let paths = ResolvedPaths::for_test_home(home);
        let plan = scan(
            &paths,
            ScanOptions {
                app_target: None,
                profile: ScanProfile::Safe,
                scopes: vec![Scope::Xcode],
            },
        )
        .expect("xcode scan");

        assert_eq!(plan.candidates.len(), 1);
        assert_eq!(plan.candidates[0].class, CandidateClass::XcodeDerivedData);
    }

    #[test]
    fn apple_temp_paths_are_report_only() {
        let temp = TempDir::new().expect("tempdir");
        let home = temp.path();
        fs::create_dir_all(home.join("tmp/com.apple.tccd")).expect("apple temp dir");
        fs::write(home.join("tmp/com.apple.tccd/blob"), "temp").expect("temp file");

        let paths = ResolvedPaths::for_test_home(home);
        let plan = scan(
            &paths,
            ScanOptions {
                app_target: None,
                profile: ScanProfile::Safe,
                scopes: vec![Scope::Temp],
            },
        )
        .expect("temp scan");

        let candidate = plan.candidates.first().expect("temp candidate");
        assert!(!candidate.eligible);
        assert_eq!(candidate.class, CandidateClass::TempPath);
    }
}
