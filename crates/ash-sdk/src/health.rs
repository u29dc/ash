use serde::{Deserialize, Serialize};
use std::env;
use std::path::Path;

use crate::config::{load_config, validate_config};
use crate::contracts::HealthStatus;
use crate::paths::ResolvedPaths;
use crate::trash::trash_backend_ready;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct HealthCheck {
    pub name: String,
    pub status: HealthStatus,
    pub message: String,
    pub fix: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct HealthReport {
    pub status: HealthStatus,
    pub checks: Vec<HealthCheck>,
    pub paths: HealthPaths,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct HealthPaths {
    pub ash_home: String,
    pub config_path: String,
    pub user_home: String,
    pub user_temp_dir: String,
}

pub fn run_health_checks(paths: &ResolvedPaths) -> HealthReport {
    let mut checks = Vec::new();
    checks.push(HealthCheck {
        name: "platform".to_string(),
        status: if cfg!(target_os = "macos") {
            HealthStatus::Ready
        } else {
            HealthStatus::Blocked
        },
        message: if cfg!(target_os = "macos") {
            "ash is running on macOS".to_string()
        } else {
            "ash is supported on macOS only".to_string()
        },
        fix: "run ash on a macOS host".to_string(),
    });

    match load_config(paths) {
        Ok(_) => checks.push(HealthCheck {
            name: "config".to_string(),
            status: HealthStatus::Ready,
            message: format!("config loaded from {}", paths.config_path.display()),
            fix: "no action required".to_string(),
        }),
        Err(error) => checks.push(HealthCheck {
            name: "config".to_string(),
            status: HealthStatus::Blocked,
            message: error.to_string(),
            fix: error.hint,
        }),
    }

    let validation = validate_config(paths);
    checks.push(HealthCheck {
        name: "configValidation".to_string(),
        status: if validation.valid {
            HealthStatus::Ready
        } else {
            HealthStatus::Blocked
        },
        message: if validation.valid {
            "config validation passed".to_string()
        } else {
            "config validation failed".to_string()
        },
        fix: "run `ash config validate --json` to inspect failing checks".to_string(),
    });

    let trash_backend_available = trash_backend_ready(paths);
    checks.push(HealthCheck {
        name: "trashBackend".to_string(),
        status: if trash_backend_available {
            HealthStatus::Ready
        } else {
            HealthStatus::Blocked
        },
        message: if trash_backend_available {
            "a trash backend is ready".to_string()
        } else {
            "no usable trash backend is available".to_string()
        },
        fix: "ensure /usr/bin/trash exists or that ~/.Trash is writable and not symlinked"
            .to_string(),
    });

    let fda_status = full_disk_access_status(paths);
    checks.push(HealthCheck {
        name: "fullDiskAccess".to_string(),
        status: fda_status.0,
        message: fda_status.1,
        fix: "grant Full Disk Access to your terminal if full-profile scans need protected user data roots".to_string(),
    });

    let hyperfine_available = command_on_path("hyperfine");
    checks.push(HealthCheck {
        name: "hyperfine".to_string(),
        status: if hyperfine_available {
            HealthStatus::Ready
        } else {
            HealthStatus::Degraded
        },
        message: if hyperfine_available {
            "hyperfine is available for benchmark scripts".to_string()
        } else {
            "hyperfine is not installed; benchmark scripts will be blocked".to_string()
        },
        fix: "install hyperfine if you want to run benchmark helpers".to_string(),
    });

    let overall = checks.iter().fold(HealthStatus::Ready, |status, check| {
        match (status, check.status) {
            (_, HealthStatus::Blocked) => HealthStatus::Blocked,
            (HealthStatus::Ready, HealthStatus::Degraded) => HealthStatus::Degraded,
            (current, _) => current,
        }
    });

    HealthReport {
        checks,
        paths: HealthPaths {
            ash_home: paths.ash_home.display().to_string(),
            config_path: paths.config_path.display().to_string(),
            user_home: paths.user_home.display().to_string(),
            user_temp_dir: paths.user_temp_dir.display().to_string(),
        },
        status: overall,
    }
}

fn full_disk_access_status(paths: &ResolvedPaths) -> (HealthStatus, String) {
    let candidates = [
        paths.user_home.join("Library/Mail"),
        paths.user_home.join("Library/Messages"),
        paths.user_home.join("Library/Safari"),
    ];

    for path in candidates {
        match std::fs::read_dir(&path) {
            Ok(_) => {
                return (
                    HealthStatus::Ready,
                    format!("read access confirmed for {}", path.display()),
                );
            }
            Err(error) if error.kind() == std::io::ErrorKind::PermissionDenied => {
                return (
                    HealthStatus::Degraded,
                    format!("permission denied for {}", path.display()),
                );
            }
            Err(_) => continue,
        }
    }

    (
        HealthStatus::Degraded,
        "Full Disk Access could not be verified from the standard protected directories"
            .to_string(),
    )
}

fn command_on_path(name: &str) -> bool {
    env::var_os("PATH")
        .map(|path| {
            env::split_paths(&path)
                .map(|entry| entry.join(name))
                .any(|candidate| is_executable_file(&candidate))
        })
        .unwrap_or(false)
}

fn is_executable_file(path: &Path) -> bool {
    path.is_file()
}

#[cfg(test)]
mod tests {
    use tempfile::TempDir;

    use super::{HealthStatus, run_health_checks};
    use crate::paths::ResolvedPaths;

    #[test]
    fn health_report_contains_platform_check() {
        let temp = TempDir::new().expect("tempdir");
        let report = run_health_checks(&ResolvedPaths::for_test_home(temp.path()));
        assert!(report.checks.iter().any(|check| check.name == "platform"));
        assert!(matches!(
            report.status,
            HealthStatus::Ready | HealthStatus::Degraded | HealthStatus::Blocked
        ));
    }
}
