use std::process::Command;
use std::time::Instant;

use serde::{Deserialize, Serialize};

use crate::error::{AshError, ErrorCode, Result};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct MaintenanceCommand {
    pub name: String,
    pub description: String,
    pub requires_sudo: bool,
    pub useful: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct MaintenanceCatalog {
    pub commands: Vec<MaintenanceCommand>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct MaintenanceRunRequest {
    pub name: String,
    pub dry_run: bool,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct MaintenanceCommandResult {
    pub name: String,
    pub dry_run: bool,
    pub success: bool,
    pub output: String,
    pub elapsed_ms: u64,
}

pub fn list_maintenance_commands() -> MaintenanceCatalog {
    MaintenanceCatalog {
        commands: vec![
            MaintenanceCommand {
                name: "dns.flush".to_string(),
                description: "Flush resolver caches and restart mDNSResponder.".to_string(),
                requires_sudo: true,
                useful: true,
            },
            MaintenanceCommand {
                name: "launchservices.rebuild".to_string(),
                description: "Rebuild the Launch Services registration database.".to_string(),
                requires_sudo: false,
                useful: true,
            },
            MaintenanceCommand {
                name: "spotlight.reindex".to_string(),
                description: "Reindex Spotlight from the filesystem root.".to_string(),
                requires_sudo: true,
                useful: true,
            },
            MaintenanceCommand {
                name: "font-cache.clear".to_string(),
                description: "Remove cached font databases.".to_string(),
                requires_sudo: true,
                useful: true,
            },
        ],
    }
}

pub fn run_maintenance_command(request: MaintenanceRunRequest) -> Result<MaintenanceCommandResult> {
    let start = Instant::now();
    let requires_sudo = list_maintenance_commands()
        .commands
        .into_iter()
        .find(|command| command.name == request.name)
        .map(|command| command.requires_sudo)
        .ok_or_else(|| {
            AshError::new(
                ErrorCode::Unsupported,
                format!("unknown maintenance command: {}", request.name),
                "run `ash maintenance list --json` to inspect supported maintenance commands",
            )
        })?;
    if request.dry_run {
        return Ok(MaintenanceCommandResult {
            elapsed_ms: start.elapsed().as_millis() as u64,
            dry_run: true,
            name: request.name,
            output: "dry-run: no command executed".to_string(),
            success: true,
        });
    }
    if requires_sudo && !effective_user_is_root()? {
        return Err(AshError::new(
            ErrorCode::PrerequisiteBlocked,
            format!(
                "maintenance command {} requires root privileges",
                request.name
            ),
            "re-run the command from a root shell or keep using --dry-run",
        ));
    }

    let output = match request.name.as_str() {
        "dns.flush" => {
            let first = Command::new("/usr/bin/dscacheutil")
                .arg("-flushcache")
                .output()
                .map_err(|error| {
                    AshError::new(
                        ErrorCode::MaintenanceFailed,
                        format!("failed to run dscacheutil: {error}"),
                        "ensure the command exists and retry from a normal macOS shell",
                    )
                })?;
            if !first.status.success() {
                return Err(AshError::new(
                    ErrorCode::MaintenanceFailed,
                    "dscacheutil flushcache failed",
                    String::from_utf8_lossy(&first.stderr).trim().to_string(),
                ));
            }
            Command::new("/usr/bin/killall")
                .args(["-HUP", "mDNSResponder"])
                .output()
                .map_err(|error| {
                    AshError::new(
                        ErrorCode::MaintenanceFailed,
                        format!("failed to restart mDNSResponder: {error}"),
                        "retry from a root shell",
                    )
                })?
        }
        "launchservices.rebuild" => Command::new(
            "/System/Library/Frameworks/CoreServices.framework/Frameworks/LaunchServices.framework/Support/lsregister",
        )
        .args(["-kill", "-r", "-domain", "local", "-domain", "user"])
        .output()
        .map_err(|error| {
            AshError::new(
                ErrorCode::MaintenanceFailed,
                format!("failed to rebuild Launch Services: {error}"),
                "verify the lsregister tool exists on this macOS version",
            )
        })?,
        "spotlight.reindex" => Command::new("/usr/bin/mdutil")
            .args(["-E", "/"])
            .output()
            .map_err(|error| {
                AshError::new(
                    ErrorCode::MaintenanceFailed,
                    format!("failed to start Spotlight reindex: {error}"),
                    "retry from a root shell",
                )
            })?,
        "font-cache.clear" => Command::new("/usr/bin/atsutil")
            .args(["databases", "-remove"])
            .output()
            .map_err(|error| {
                AshError::new(
                    ErrorCode::MaintenanceFailed,
                    format!("failed to clear font cache: {error}"),
                    "retry from a root shell",
                )
            })?,
        _ => unreachable!("validated against registry above"),
    };

    if !output.status.success() {
        return Err(AshError::new(
            ErrorCode::MaintenanceFailed,
            format!("maintenance command {} failed", request.name),
            String::from_utf8_lossy(&output.stderr).trim().to_string(),
        ));
    }

    Ok(MaintenanceCommandResult {
        elapsed_ms: start.elapsed().as_millis() as u64,
        dry_run: false,
        name: request.name,
        output: String::from_utf8_lossy(&output.stdout).trim().to_string(),
        success: true,
    })
}

fn effective_user_is_root() -> Result<bool> {
    let output = Command::new("/usr/bin/id")
        .arg("-u")
        .output()
        .map_err(|error| {
            AshError::new(
                ErrorCode::PrerequisiteBlocked,
                format!("failed to determine effective user id: {error}"),
                "ensure `/usr/bin/id` is available and retry",
            )
        })?;
    if !output.status.success() {
        return Err(AshError::new(
            ErrorCode::PrerequisiteBlocked,
            "failed to determine effective user id",
            String::from_utf8_lossy(&output.stderr).trim().to_string(),
        ));
    }
    Ok(String::from_utf8_lossy(&output.stdout).trim() == "0")
}

#[cfg(test)]
mod tests {
    use super::{MaintenanceRunRequest, list_maintenance_commands, run_maintenance_command};
    use crate::ErrorCode;

    #[test]
    fn list_contains_expected_commands() {
        let catalog = list_maintenance_commands();
        assert!(
            catalog
                .commands
                .iter()
                .any(|command| command.name == "dns.flush")
        );
    }

    #[test]
    fn dry_run_succeeds_without_execution() {
        let result = run_maintenance_command(MaintenanceRunRequest {
            name: "dns.flush".to_string(),
            dry_run: true,
        })
        .expect("dry-run maintenance");
        assert!(result.success);
        assert!(result.dry_run);
    }

    #[test]
    fn sudo_commands_are_blocked_when_not_root() {
        let is_root = super::effective_user_is_root().expect("effective uid");
        if is_root {
            return;
        }
        let error = run_maintenance_command(MaintenanceRunRequest {
            name: "dns.flush".to_string(),
            dry_run: false,
        })
        .expect_err("dns.flush should be blocked when not root");
        assert_eq!(error.code, ErrorCode::PrerequisiteBlocked);
    }
}
