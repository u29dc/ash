use std::fs;
use std::io::Cursor;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::{Duration, UNIX_EPOCH};

use chrono::{DateTime, Utc};
use plist::Value;
use serde::{Deserialize, Serialize};
use walkdir::WalkDir;

use crate::config::AppConfig;
use crate::error::Result;
use crate::paths::ResolvedPaths;

const INVENTORY_CACHE_FILE: &str = "app-inventory.json";

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct AppRecord {
    pub bundle_id: String,
    pub name: String,
    pub display_name: String,
    pub app_path: String,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
struct InventoryCache {
    pub generated_at: DateTime<Utc>,
    pub root_mtimes: Vec<RootMtime>,
    pub apps: Vec<AppRecord>,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
struct RootMtime {
    pub path: String,
    pub modified_unix_seconds: Option<u64>,
}

pub fn load_inventory(paths: &ResolvedPaths, config: &AppConfig) -> Result<Vec<AppRecord>> {
    if let Some(cached) = read_cache(paths, config)? {
        return Ok(cached.apps);
    }

    let apps = build_inventory(paths, config)?;
    write_cache(paths, config, &apps)?;
    Ok(apps)
}

fn read_cache(paths: &ResolvedPaths, config: &AppConfig) -> Result<Option<InventoryCache>> {
    if config.inventory_cache_ttl_seconds == 0 {
        return Ok(None);
    }

    let cache_path = paths.cache_dir.join(INVENTORY_CACHE_FILE);
    let contents = match fs::read_to_string(&cache_path) {
        Ok(contents) => contents,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => return Ok(None),
        Err(error) => return Err(error.into()),
    };
    let cache: InventoryCache = serde_json::from_str(&contents).map_err(|error| {
        crate::error::AshError::runtime(format!("invalid app inventory cache: {error}"))
    })?;
    let max_age = Duration::from_secs(config.inventory_cache_ttl_seconds);
    let age = Utc::now()
        .signed_duration_since(cache.generated_at)
        .to_std()
        .unwrap_or_default();
    if age > max_age {
        return Ok(None);
    }

    let current = collect_root_mtimes(paths, config);
    if cache.root_mtimes != current {
        return Ok(None);
    }
    Ok(Some(cache))
}

fn write_cache(paths: &ResolvedPaths, config: &AppConfig, apps: &[AppRecord]) -> Result<()> {
    if config.inventory_cache_ttl_seconds == 0 {
        return Ok(());
    }
    fs::create_dir_all(&paths.cache_dir)?;
    let cache = InventoryCache {
        apps: apps.to_vec(),
        generated_at: Utc::now(),
        root_mtimes: collect_root_mtimes(paths, config),
    };
    fs::write(
        paths.cache_dir.join(INVENTORY_CACHE_FILE),
        serde_json::to_vec_pretty(&cache).map_err(|error| {
            crate::error::AshError::runtime(format!("failed to serialize inventory cache: {error}"))
        })?,
    )?;
    Ok(())
}

fn build_inventory(paths: &ResolvedPaths, config: &AppConfig) -> Result<Vec<AppRecord>> {
    let mut apps = Vec::new();
    let mut seen_paths = std::collections::BTreeSet::new();
    for root in effective_app_roots(paths, config) {
        if !root.exists() {
            continue;
        }
        for entry in WalkDir::new(root)
            .follow_links(false)
            .into_iter()
            .filter_entry(|entry| !is_hidden(entry.path()))
            .filter_map(std::result::Result::ok)
        {
            if entry.file_type().is_dir()
                && entry
                    .path()
                    .extension()
                    .is_some_and(|extension| extension.eq_ignore_ascii_case("app"))
            {
                let path = entry.path().to_path_buf();
                if seen_paths.insert(path.clone())
                    && let Some(record) = read_app_record(&path)
                {
                    apps.push(record);
                }
            }
        }
    }
    apps.sort_by(|left, right| {
        left.bundle_id
            .cmp(&right.bundle_id)
            .then(left.app_path.cmp(&right.app_path))
    });
    Ok(apps)
}

fn read_app_record(path: &Path) -> Option<AppRecord> {
    let plist_path = path.join("Contents").join("Info.plist");
    let value = Value::from_file(&plist_path).ok()?;
    let dict = value.as_dictionary()?;
    let bundle_id = dict.get("CFBundleIdentifier")?.as_string()?.to_string();
    let derived_name = derive_display_name(&bundle_id);
    let name = dict
        .get("CFBundleName")
        .and_then(Value::as_string)
        .unwrap_or(&derived_name)
        .to_string();
    let display_name = dict
        .get("CFBundleDisplayName")
        .and_then(Value::as_string)
        .unwrap_or(&name)
        .to_string();

    Some(AppRecord {
        app_path: path.display().to_string(),
        bundle_id,
        display_name,
        name,
    })
}

fn derive_display_name(bundle_id: &str) -> String {
    bundle_id
        .split('.')
        .next_back()
        .unwrap_or(bundle_id)
        .replace(['-', '_'], " ")
}

fn effective_app_roots(paths: &ResolvedPaths, config: &AppConfig) -> Vec<PathBuf> {
    if config.app_roots.is_empty() {
        return paths.app_roots.clone();
    }
    config.app_roots.iter().map(PathBuf::from).collect()
}

fn collect_root_mtimes(paths: &ResolvedPaths, config: &AppConfig) -> Vec<RootMtime> {
    let mut mtimes = effective_app_roots(paths, config)
        .into_iter()
        .map(|path| RootMtime {
            modified_unix_seconds: root_mtime(&path),
            path: path.display().to_string(),
        })
        .collect::<Vec<_>>();
    mtimes.sort_by(|left, right| left.path.cmp(&right.path));
    mtimes
}

fn root_mtime(path: &Path) -> Option<u64> {
    let modified = fs::metadata(path).ok()?.modified().ok()?;
    modified
        .duration_since(UNIX_EPOCH)
        .ok()
        .map(|duration| duration.as_secs())
}

fn is_hidden(path: &Path) -> bool {
    path.file_name()
        .and_then(|name| name.to_str())
        .is_some_and(|name| name.starts_with('.'))
}

pub fn find_app_by_bundle_id(inventory: &[AppRecord], bundle_id: &str) -> Option<AppRecord> {
    inventory
        .iter()
        .find(|app| app.bundle_id == bundle_id)
        .cloned()
}

pub fn read_app_groups(app_path: &Path) -> Vec<String> {
    let output = match Command::new("/usr/bin/codesign")
        .args(["-d", "--entitlements", ":-"])
        .arg(app_path)
        .output()
    {
        Ok(output)
            if output.status.success()
                || !output.stdout.is_empty()
                || !output.stderr.is_empty() =>
        {
            output
        }
        _ => return Vec::new(),
    };

    let bytes = if !output.stdout.is_empty() {
        output.stdout
    } else {
        output.stderr
    };
    let start = bytes.iter().position(|byte| *byte == b'<').unwrap_or(0);
    let slice = &bytes[start..];
    let value = match Value::from_reader_xml(Cursor::new(slice)) {
        Ok(value) => value,
        Err(_) => return Vec::new(),
    };
    value
        .as_dictionary()
        .and_then(|dict| dict.get("com.apple.security.application-groups"))
        .and_then(Value::as_array)
        .map(|items| {
            items
                .iter()
                .filter_map(Value::as_string)
                .map(ToString::to_string)
                .collect::<Vec<_>>()
        })
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use std::fs;

    use tempfile::TempDir;

    use super::{build_inventory, find_app_by_bundle_id, read_app_groups};
    use crate::config::AppConfig;
    use crate::paths::ResolvedPaths;

    #[test]
    fn inventory_builds_from_app_bundles() {
        let temp = TempDir::new().expect("tempdir");
        let home = temp.path();
        let app_dir = home.join("Applications/Test.app/Contents");
        fs::create_dir_all(&app_dir).expect("app bundle");
        fs::write(
            app_dir.join("Info.plist"),
            r#"<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
<key>CFBundleIdentifier</key><string>com.example.test</string>
<key>CFBundleName</key><string>Test</string>
<key>CFBundleDisplayName</key><string>Test App</string>
</dict></plist>"#,
        )
        .expect("plist");

        let paths = ResolvedPaths::for_test_home(home);
        let apps = build_inventory(&paths, &AppConfig::default()).expect("inventory");
        let app = find_app_by_bundle_id(&apps, "com.example.test").expect("record");
        assert_eq!(app.display_name, "Test App");
    }

    #[test]
    fn app_groups_fallback_is_empty_for_missing_bundle() {
        let temp = TempDir::new().expect("tempdir");
        assert!(read_app_groups(temp.path()).is_empty());
    }
}
