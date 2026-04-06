use std::env;
use std::path::{Path, PathBuf};
use std::process::Command;

use serde::{Deserialize, Serialize};

use crate::error::{AshError, ErrorCode, Result};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ResolvedPaths {
    pub ash_home: PathBuf,
    pub config_path: PathBuf,
    pub cache_dir: PathBuf,
    pub user_home: PathBuf,
    pub user_temp_dir: PathBuf,
    pub user_cache_dir: PathBuf,
    pub app_roots: Vec<PathBuf>,
}

impl ResolvedPaths {
    pub fn for_test_home(home: impl Into<PathBuf>) -> Self {
        let user_home = home.into();
        let ash_home = user_home.join(".tools").join("ash");
        Self {
            config_path: ash_home.join("config.toml"),
            cache_dir: ash_home.join("cache"),
            user_temp_dir: user_home.join("tmp"),
            user_cache_dir: user_home.join("Library").join("Caches"),
            app_roots: standard_app_roots(&user_home),
            ash_home,
            user_home,
        }
    }
}

pub fn resolve_paths() -> Result<ResolvedPaths> {
    let user_home = env::var_os("HOME").map(PathBuf::from).ok_or_else(|| {
        AshError::new(
            ErrorCode::PlatformBlocked,
            "HOME is not set",
            "run ash from a normal user shell on macOS",
        )
    })?;
    let ash_home = resolve_ash_home(&user_home);
    let user_temp_dir = darwin_dir("DARWIN_USER_TEMP_DIR")
        .or_else(|| env::var_os("TMPDIR").map(PathBuf::from))
        .unwrap_or_else(|| PathBuf::from("/tmp"));
    let user_cache_dir = user_home.join("Library").join("Caches");

    Ok(ResolvedPaths {
        config_path: ash_home.join("config.toml"),
        cache_dir: ash_home.join("cache"),
        app_roots: standard_app_roots(&user_home),
        ash_home,
        user_home,
        user_temp_dir,
        user_cache_dir,
    })
}

fn resolve_ash_home(user_home: &Path) -> PathBuf {
    if let Some(path) = env::var_os("ASH_HOME").filter(|value| !value.is_empty()) {
        return PathBuf::from(path);
    }
    if let Some(path) = env::var_os("TOOLS_HOME").filter(|value| !value.is_empty()) {
        return PathBuf::from(path).join("ash");
    }
    user_home.join(".tools").join("ash")
}

fn darwin_dir(name: &str) -> Option<PathBuf> {
    let output = Command::new("/usr/bin/getconf").arg(name).output().ok()?;
    if !output.status.success() {
        return None;
    }
    let value = String::from_utf8_lossy(&output.stdout).trim().to_string();
    if value.is_empty() {
        return None;
    }
    Some(PathBuf::from(value))
}

fn standard_app_roots(user_home: &Path) -> Vec<PathBuf> {
    vec![
        PathBuf::from("/Applications"),
        PathBuf::from("/System/Applications"),
        user_home.join("Applications"),
        PathBuf::from("/Applications/Setapp"),
    ]
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    use super::ResolvedPaths;

    #[test]
    fn test_paths_follow_expected_layout() {
        let paths = ResolvedPaths::for_test_home(PathBuf::from("/tmp/example-home"));
        assert_eq!(
            paths.ash_home,
            PathBuf::from("/tmp/example-home/.tools/ash")
        );
        assert_eq!(
            paths.user_cache_dir,
            PathBuf::from("/tmp/example-home/Library/Caches")
        );
        assert!(paths.app_roots.contains(&PathBuf::from("/Applications")));
    }
}
