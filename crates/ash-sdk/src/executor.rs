use std::fs;
use std::path::Path;

use serde::{Deserialize, Serialize};

use crate::error::{AshError, ErrorCode, Result};
use crate::paths::ResolvedPaths;
use crate::planner::{CleanupPlan, verify_plan_hash};
use crate::policy::{RiskLevel, is_protected_absolute_path};
use crate::trash::move_to_trash;

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

pub fn apply_plan(paths: &ResolvedPaths, request: ApplyRequest) -> Result<ExecutionReport> {
    verify_plan_hash(&request.plan)?;
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

#[cfg(test)]
mod tests {
    use std::fs;

    use tempfile::TempDir;

    use super::{ApplyRequest, MaxRisk, apply_plan};
    use crate::paths::ResolvedPaths;
    use crate::planner::{ScanOptions, ScanProfile, Scope, scan};

    #[test]
    fn dry_run_reports_would_move_items() {
        let temp = TempDir::new().expect("tempdir");
        let home = temp.path();
        fs::create_dir_all(home.join("tmp")).expect("tmp");
        fs::write(home.join("tmp/file.tmp"), "temp").expect("temp file");
        let paths = ResolvedPaths::for_test_home(home);
        let plan = scan(
            &paths,
            ScanOptions {
                app_target: None,
                profile: ScanProfile::Safe,
                scopes: vec![Scope::Temp],
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
}
