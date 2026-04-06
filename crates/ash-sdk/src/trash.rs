use std::fs;
use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

use serde::{Deserialize, Serialize};

use crate::error::{AshError, ErrorCode, Result};
use crate::paths::ResolvedPaths;

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub enum TrashBackend {
    Native,
    Fallback,
}

pub fn detect_trash_backend(paths: &ResolvedPaths) -> TrashBackend {
    let current_home = std::env::var_os("HOME").map(PathBuf::from);
    if current_home
        .as_ref()
        .is_some_and(|home| *home == paths.user_home)
        && Path::new("/usr/bin/trash").exists()
    {
        return TrashBackend::Native;
    }
    TrashBackend::Fallback
}

pub fn trash_backend_ready(paths: &ResolvedPaths) -> bool {
    match detect_trash_backend(paths) {
        TrashBackend::Native => true,
        TrashBackend::Fallback => fallback_trash_dir_ready(paths).is_ok(),
    }
}

pub fn move_to_trash(path: &Path, paths: &ResolvedPaths) -> Result<TrashBackend> {
    match detect_trash_backend(paths) {
        TrashBackend::Native => {
            let output = Command::new("/usr/bin/trash").arg(path).output().map_err(|error| {
                AshError::new(
                    ErrorCode::Runtime,
                    format!("failed to invoke /usr/bin/trash: {error}"),
                    "verify macOS 15+ trash support or fall back to a writable user trash directory",
                )
            })?;
            if !output.status.success() {
                return Err(AshError::new(
                    ErrorCode::Runtime,
                    format!("failed to move {} to trash", path.display()),
                    String::from_utf8_lossy(&output.stderr).trim().to_string(),
                ));
            }
            Ok(TrashBackend::Native)
        }
        TrashBackend::Fallback => {
            move_to_fallback_trash(path, paths)?;
            Ok(TrashBackend::Fallback)
        }
    }
}

fn move_to_fallback_trash(path: &Path, paths: &ResolvedPaths) -> Result<()> {
    let trash_dir = ensure_fallback_trash_dir(paths)?;
    let base_name = path.file_name().ok_or_else(|| {
        AshError::new(
            ErrorCode::Runtime,
            "path has no file name",
            "pass a concrete file or directory path",
        )
    })?;
    let mut destination = trash_dir.join(base_name);
    if destination.exists() {
        destination = unique_fallback_destination(&trash_dir, base_name);
    }
    fs::rename(path, destination)?;
    Ok(())
}

fn ensure_fallback_trash_dir(paths: &ResolvedPaths) -> Result<PathBuf> {
    let trash_dir = paths.user_home.join(".Trash");
    fs::create_dir_all(&trash_dir)?;
    let metadata = fs::symlink_metadata(&trash_dir)?;
    if metadata.file_type().is_symlink() {
        return Err(AshError::new(
            ErrorCode::SafetyBlocked,
            format!("trash path must not be a symlink: {}", trash_dir.display()),
            "replace the symlinked trash path with a real directory and retry",
        ));
    }
    fs::set_permissions(&trash_dir, fs::Permissions::from_mode(0o700))?;
    Ok(trash_dir)
}

fn fallback_trash_dir_ready(paths: &ResolvedPaths) -> Result<()> {
    let trash_dir = paths.user_home.join(".Trash");
    match fs::symlink_metadata(&trash_dir) {
        Ok(metadata) => {
            if metadata.file_type().is_symlink() {
                return Err(AshError::new(
                    ErrorCode::SafetyBlocked,
                    format!("trash path must not be a symlink: {}", trash_dir.display()),
                    "replace the symlinked trash path with a real directory and retry",
                ));
            }
            if !metadata.is_dir() {
                return Err(AshError::new(
                    ErrorCode::SafetyBlocked,
                    format!("trash path is not a directory: {}", trash_dir.display()),
                    "replace the trash path with a real directory and retry",
                ));
            }
            if !path_is_writable(&trash_dir) {
                return Err(AshError::new(
                    ErrorCode::SafetyBlocked,
                    format!("trash path is not writable: {}", trash_dir.display()),
                    "ensure the trash directory is writable and retry",
                ));
            }
            Ok(())
        }
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
            if path_is_writable(&paths.user_home) {
                Ok(())
            } else {
                Err(AshError::new(
                    ErrorCode::SafetyBlocked,
                    format!(
                        "trash parent is not writable: {}",
                        paths.user_home.display()
                    ),
                    "ensure the home directory is writable and retry",
                ))
            }
        }
        Err(error) => Err(error.into()),
    }
}

fn path_is_writable(path: &Path) -> bool {
    path.metadata()
        .map(|metadata| (metadata.permissions().mode() & 0o222) != 0)
        .unwrap_or(false)
}

fn unique_fallback_destination(trash_dir: &Path, base_name: &std::ffi::OsStr) -> PathBuf {
    let stamp = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_nanos();
    let mut file_name = base_name.to_os_string();
    file_name.push(format!(".{stamp}"));
    trash_dir.join(file_name)
}

#[cfg(test)]
mod tests {
    use std::fs;

    use tempfile::TempDir;

    use super::{TrashBackend, detect_trash_backend, move_to_trash};
    use crate::paths::ResolvedPaths;

    #[test]
    fn test_home_uses_fallback_backend() {
        let temp = TempDir::new().expect("tempdir");
        let paths = ResolvedPaths::for_test_home(temp.path());
        assert_eq!(detect_trash_backend(&paths), TrashBackend::Fallback);
    }

    #[test]
    fn fallback_move_creates_test_trash() {
        let temp = TempDir::new().expect("tempdir");
        let paths = ResolvedPaths::for_test_home(temp.path());
        let source = temp.path().join("cache.bin");
        fs::write(&source, "cache").expect("source");
        move_to_trash(&source, &paths).expect("move to trash");
        assert!(!source.exists());
        assert!(temp.path().join(".Trash").exists());
    }

    #[test]
    fn readiness_check_does_not_create_trash_directory() {
        let temp = TempDir::new().expect("tempdir");
        let paths = ResolvedPaths::for_test_home(temp.path());
        assert!(super::trash_backend_ready(&paths));
        assert!(!temp.path().join(".Trash").exists());
    }
}
