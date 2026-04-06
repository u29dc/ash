use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

use chrono::{DateTime, Utc};
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

pub fn scan(paths: &ResolvedPaths, options: ScanOptions) -> Result<CleanupPlan> {
    let config = load_config(paths)?;
    let scopes = expand_scopes(options.profile, &options.scopes);
    let needs_inventory = scopes.contains(&Scope::Apps) || options.app_target.is_some();
    let inventory = if needs_inventory {
        load_inventory(paths, &config)?
    } else {
        Vec::new()
    };
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
            .map(|scope| scan_scope(paths, scope, &inventory, target_app.as_ref()))
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
    scope: Scope,
    inventory: &[AppRecord],
    target_app: Option<&TargetApp>,
) -> Result<Vec<CleanupCandidate>> {
    match scope {
        Scope::Temp => scan_entries(
            &paths.user_temp_dir,
            CandidateClass::TempPath,
            paths,
            |name| risk_for_cache_like_name(name, CandidateClass::TempPath),
            |_| None,
        ),
        Scope::Caches => scan_caches(paths),
        Scope::Logs => scan_entries(
            &paths.user_home.join("Library").join("Logs"),
            CandidateClass::UserLog,
            paths,
            risk_for_log_name,
            |_| None,
        ),
        Scope::Xcode => scan_xcode(paths),
        Scope::Homebrew => scan_homebrew(paths),
        Scope::Browsers => scan_browsers(paths),
        Scope::Apps => scan_app_state(paths, inventory, target_app),
        Scope::All => Ok(Vec::new()),
    }
}

fn scan_entries<F, G>(
    base: &Path,
    class: CandidateClass,
    paths: &ResolvedPaths,
    risk_fn: F,
    reason_fn: G,
) -> Result<Vec<CleanupCandidate>>
where
    F: Fn(&str) -> RiskLevel,
    G: Fn(&Path) -> Option<String>,
{
    let mut candidates = Vec::new();
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
        let blocked_reason = reason_fn(&path);
        let size_bytes = calculate_size(&path)?;
        candidates.push(build_candidate(CandidateDraft {
            canonical_path: canonical,
            class,
            risk: risk_fn(&name),
            size_bytes,
            owner_status: OwnerStatus::Unknown,
            evidence: vec![
                Evidence::new("scope", class.category()),
                Evidence::new("basePath", base_display.clone()),
            ],
            running_processes: Vec::new(),
            eligible: blocked_reason.is_none(),
            blocked_reason,
        }));
    }
    Ok(candidates)
}

fn scan_caches(paths: &ResolvedPaths) -> Result<Vec<CleanupCandidate>> {
    let browser_roots = browser_paths(paths);
    let homebrew_root = paths.user_cache_dir.join("Homebrew");
    let mut candidates = Vec::new();
    let Ok(entries) = fs::read_dir(&paths.user_cache_dir) else {
        return Ok(candidates);
    };
    for entry in entries.flatten() {
        let path = entry.path();
        if path == homebrew_root || browser_roots.iter().any(|root| root == &path) {
            continue;
        }
        let canonical = canonical_string(&path);
        if is_protected_absolute_path(&canonical, &paths.user_home.display().to_string()) {
            continue;
        }
        let name = entry.file_name().to_string_lossy().to_string();
        let size_bytes = calculate_size(&path)?;
        let risk = risk_for_cache_like_name(&name, CandidateClass::UserCache);
        candidates.push(build_candidate(CandidateDraft {
            canonical_path: canonical,
            class: CandidateClass::UserCache,
            risk,
            size_bytes,
            owner_status: OwnerStatus::Unknown,
            evidence: vec![
                Evidence::new("scope", "caches"),
                Evidence::new("cacheName", name),
            ],
            running_processes: Vec::new(),
            eligible: true,
            blocked_reason: None,
        }));
    }
    Ok(candidates)
}

fn scan_homebrew(paths: &ResolvedPaths) -> Result<Vec<CleanupCandidate>> {
    let path = paths.user_cache_dir.join("Homebrew");
    if !path.exists() {
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
            Evidence::new("strategy", "cache-directory"),
        ],
        running_processes: Vec::new(),
        eligible: true,
        blocked_reason: None,
    })])
}

fn scan_browsers(paths: &ResolvedPaths) -> Result<Vec<CleanupCandidate>> {
    let mut candidates = Vec::new();
    for path in browser_paths(paths) {
        if !path.exists() {
            continue;
        }
        let size_bytes = calculate_size(&path)?;
        candidates.push(build_candidate(CandidateDraft {
            canonical_path: canonical_string(&path),
            class: CandidateClass::BrowserDiskCache,
            risk: CandidateClass::BrowserDiskCache.default_risk(),
            size_bytes,
            owner_status: OwnerStatus::Unknown,
            evidence: vec![
                Evidence::new("scope", "browsers"),
                Evidence::new("path", path.display().to_string()),
            ],
            running_processes: Vec::new(),
            eligible: true,
            blocked_reason: None,
        }));
    }
    Ok(candidates)
}

fn browser_paths(paths: &ResolvedPaths) -> Vec<PathBuf> {
    let cache = &paths.user_cache_dir;
    vec![
        cache.join("com.apple.Safari"),
        cache.join("Google").join("Chrome"),
        cache.join("Firefox"),
        cache.join("org.mozilla.firefox"),
        cache.join("com.brave.Browser"),
        cache.join("com.microsoft.edgemac"),
        cache.join("com.operasoftware.Opera"),
        cache.join("company.thebrowser.Browser"),
    ]
}

fn scan_xcode(paths: &ResolvedPaths) -> Result<Vec<CleanupCandidate>> {
    let developer = paths.user_home.join("Library").join("Developer");
    let roots = [
        (
            developer.join("Xcode").join("DerivedData"),
            CandidateClass::XcodeDerivedData,
            None,
        ),
        (
            developer.join("Xcode").join("iOS DeviceSupport"),
            CandidateClass::XcodeDeviceSupport,
            None,
        ),
        (
            developer.join("Xcode").join("Archives"),
            CandidateClass::XcodeArchives,
            None,
        ),
        (
            developer.join("CoreSimulator").join("Devices"),
            CandidateClass::SimulatorDeviceSet,
            Some(
                "simulator device trees are reported but not eligible in generic plans".to_string(),
            ),
        ),
    ];

    let mut candidates = Vec::new();
    for (base, class, blocked_reason) in roots {
        let Ok(entries) = fs::read_dir(&base) else {
            continue;
        };
        for entry in entries.flatten() {
            let path = entry.path();
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
                let name = entry.file_name().to_string_lossy().to_string();
                if let Some(bundle_id) = extract_bundle_id(&name)
                    && bundle_id == target.bundle_id
                {
                    let size_bytes = calculate_size(&path)?;
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
                        eligible: true,
                        blocked_reason: None,
                    }));
                }
            }
        }
        return Ok(candidates);
    }

    for (base, class) in locations {
        let Ok(entries) = fs::read_dir(&base) else {
            continue;
        };
        for entry in entries.flatten() {
            let path = entry.path();
            let name = entry.file_name().to_string_lossy().to_string();
            let Some(bundle_id) = extract_bundle_id(&name) else {
                continue;
            };
            if bundle_id.starts_with("com.apple.") {
                continue;
            }
            if find_app_by_bundle_id(inventory, &bundle_id).is_some() {
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
    let output = match Command::new("/usr/bin/pgrep")
        .args(["-fal", &app_path.display().to_string()])
        .output()
    {
        Ok(output) if output.status.success() || !output.stdout.is_empty() => output,
        _ => return Vec::new(),
    };
    String::from_utf8_lossy(&output.stdout)
        .lines()
        .filter_map(|line| {
            let mut parts = line.splitn(2, ' ');
            let pid = parts.next()?.parse::<u32>().ok()?;
            let command = parts.next().unwrap_or_default().to_string();
            Some(RunningProcess { pid, command })
        })
        .collect()
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

    use super::{ScanOptions, ScanProfile, Scope, scan};
    use crate::paths::ResolvedPaths;

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
}
