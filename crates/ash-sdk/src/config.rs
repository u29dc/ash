use std::fs;
use std::path::Path;

use serde::{Deserialize, Serialize};

use crate::error::{AshError, ErrorCode, Result};
use crate::paths::ResolvedPaths;
use crate::planner::ScanProfile;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct AppConfig {
    pub parallelism: usize,
    pub default_profile: ScanProfile,
    pub app_roots: Vec<String>,
    pub inventory_cache_ttl_seconds: u64,
}

impl Default for AppConfig {
    fn default() -> Self {
        Self {
            parallelism: 8,
            default_profile: ScanProfile::Safe,
            app_roots: Vec::new(),
            inventory_cache_ttl_seconds: 300,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ConfigShowData {
    pub config: AppConfig,
    pub config_path: String,
    pub ash_home: String,
    pub user_home: String,
    pub user_temp_dir: String,
    pub user_cache_dir: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ConfigValidationCheck {
    pub name: String,
    pub status: String,
    pub message: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ConfigValidationResult {
    pub valid: bool,
    pub checks: Vec<ConfigValidationCheck>,
}

pub fn load_config(paths: &ResolvedPaths) -> Result<AppConfig> {
    load_config_from_path(&paths.config_path)
}

fn load_config_from_path(path: &Path) -> Result<AppConfig> {
    match fs::read_to_string(path) {
        Ok(contents) => toml::from_str(&contents).map_err(|error| {
            AshError::new(
                ErrorCode::ConfigInvalid,
                format!("failed to parse config at {}: {error}", path.display()),
                "fix the TOML syntax or remove the file to fall back to defaults",
            )
        }),
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(AppConfig::default()),
        Err(error) => Err(AshError::new(
            ErrorCode::ConfigBlocked,
            format!("failed to read config at {}: {error}", path.display()),
            "ensure the config file is readable or remove it to use defaults",
        )),
    }
}

pub fn show_config(paths: &ResolvedPaths) -> Result<ConfigShowData> {
    let config = load_config(paths)?;
    Ok(ConfigShowData {
        ash_home: paths.ash_home.display().to_string(),
        config,
        config_path: paths.config_path.display().to_string(),
        user_cache_dir: paths.user_cache_dir.display().to_string(),
        user_home: paths.user_home.display().to_string(),
        user_temp_dir: paths.user_temp_dir.display().to_string(),
    })
}

pub fn validate_config(paths: &ResolvedPaths) -> ConfigValidationResult {
    let mut checks = Vec::new();
    let valid = match load_config_from_path(&paths.config_path) {
        Ok(config) => {
            if config.parallelism == 0 {
                checks.push(ConfigValidationCheck {
                    name: "parallelism".to_string(),
                    status: "error".to_string(),
                    message: "parallelism must be greater than zero".to_string(),
                });
            } else {
                checks.push(ConfigValidationCheck {
                    name: "parallelism".to_string(),
                    status: "ok".to_string(),
                    message: format!("parallelism is {}", config.parallelism),
                });
            }
            if config.inventory_cache_ttl_seconds == 0 {
                checks.push(ConfigValidationCheck {
                    name: "inventoryCacheTtlSeconds".to_string(),
                    status: "warn".to_string(),
                    message:
                        "inventory cache TTL is zero; every scan will rebuild the app inventory"
                            .to_string(),
                });
            } else {
                checks.push(ConfigValidationCheck {
                    name: "inventoryCacheTtlSeconds".to_string(),
                    status: "ok".to_string(),
                    message: format!(
                        "inventory cache TTL is {} seconds",
                        config.inventory_cache_ttl_seconds
                    ),
                });
            }
            if config.app_roots.is_empty() {
                checks.push(ConfigValidationCheck {
                    name: "appRoots".to_string(),
                    status: "ok".to_string(),
                    message: "using the built-in macOS app roots".to_string(),
                });
            }
            config.parallelism > 0
        }
        Err(error) => {
            checks.push(ConfigValidationCheck {
                name: "config".to_string(),
                status: "error".to_string(),
                message: error.to_string(),
            });
            false
        }
    };

    ConfigValidationResult { valid, checks }
}

#[cfg(test)]
mod tests {
    use std::fs;

    use tempfile::TempDir;

    use super::{AppConfig, load_config, show_config, validate_config};
    use crate::paths::ResolvedPaths;

    #[test]
    fn missing_config_uses_defaults() {
        let dir = TempDir::new().expect("tempdir");
        let paths = ResolvedPaths::for_test_home(dir.path());
        let config = load_config(&paths).expect("default config");
        assert_eq!(config, AppConfig::default());
    }

    #[test]
    fn validate_flags_parse_errors() {
        let dir = TempDir::new().expect("tempdir");
        let paths = ResolvedPaths::for_test_home(dir.path());
        fs::create_dir_all(paths.ash_home.clone()).expect("create ash home");
        fs::write(paths.config_path.clone(), "parallelism = 'bad'").expect("write config");
        let result = validate_config(&paths);
        assert!(!result.valid);
    }

    #[test]
    fn show_config_reports_paths() {
        let dir = TempDir::new().expect("tempdir");
        let paths = ResolvedPaths::for_test_home(dir.path());
        let data = show_config(&paths).expect("show config");
        assert!(data.config_path.ends_with("config.toml"));
    }
}
