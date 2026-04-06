use serde::{Deserialize, Serialize};

use crate::error::Result;
use crate::executor::{ApplyRequest, ExecutionReport, MaxRisk, apply_plan};
use crate::paths::ResolvedPaths;
use crate::planner::{CleanupPlan, ScanOptions, scan};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct CleanRequest {
    pub scan: ScanOptions,
    pub max_risk: MaxRisk,
    pub dry_run: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct CleanRun {
    pub plan: CleanupPlan,
    pub execution: ExecutionReport,
}

pub fn run_clean(paths: &ResolvedPaths, request: CleanRequest) -> Result<CleanRun> {
    let plan = scan(paths, request.scan)?;
    let execution = apply_plan(
        paths,
        ApplyRequest {
            plan: plan.clone(),
            max_risk: request.max_risk,
            dry_run: request.dry_run,
        },
    )?;
    Ok(CleanRun { plan, execution })
}

#[cfg(test)]
mod tests {
    use std::fs;

    use tempfile::TempDir;

    use super::{CleanRequest, run_clean};
    use crate::executor::MaxRisk;
    use crate::paths::ResolvedPaths;
    use crate::planner::{ScanOptions, ScanProfile, Scope};

    #[test]
    fn clean_dry_run_uses_scan_and_apply_without_mutating() {
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

        let result = run_clean(
            &paths,
            CleanRequest {
                scan: ScanOptions {
                    app_target: None,
                    profile: ScanProfile::Safe,
                    scopes: vec![Scope::Xcode],
                },
                max_risk: MaxRisk::Safe,
                dry_run: true,
            },
        )
        .expect("clean dry run");

        assert!(result.plan.summary.eligible_candidates >= 1);
        assert!(result.execution.dry_run);
        assert!(result.execution.moved_count >= 1);
    }
}
