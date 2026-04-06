use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq, PartialOrd, Ord)]
#[serde(rename_all = "camelCase")]
pub enum RiskLevel {
    Safe,
    Review,
    Dangerous,
}

impl RiskLevel {
    pub fn allows(self, candidate: RiskLevel) -> bool {
        self >= candidate
    }

    pub fn as_str(self) -> &'static str {
        match self {
            Self::Safe => "safe",
            Self::Review => "review",
            Self::Dangerous => "dangerous",
        }
    }
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub enum CandidateClass {
    TempPath,
    UserCache,
    AppCacheLeftover,
    UserLog,
    AppLogLeftover,
    BrowserDiskCache,
    HomebrewCache,
    XcodeDerivedData,
    XcodeDeviceSupport,
    XcodeArchives,
    SimulatorDeviceSet,
    ApplicationSupport,
    PreferencePlist,
    SandboxContainer,
    GroupContainer,
    SavedApplicationState,
    WebKitData,
    HttpStorage,
    CookieStore,
    LaunchAgent,
    SystemLog,
}

impl CandidateClass {
    pub fn category(self) -> &'static str {
        match self {
            Self::TempPath => "temp",
            Self::UserCache
            | Self::AppCacheLeftover
            | Self::BrowserDiskCache
            | Self::HomebrewCache => "caches",
            Self::UserLog | Self::AppLogLeftover | Self::SystemLog => "logs",
            Self::XcodeDerivedData
            | Self::XcodeDeviceSupport
            | Self::XcodeArchives
            | Self::SimulatorDeviceSet => "xcode",
            Self::ApplicationSupport
            | Self::PreferencePlist
            | Self::SandboxContainer
            | Self::GroupContainer
            | Self::SavedApplicationState
            | Self::WebKitData
            | Self::HttpStorage
            | Self::CookieStore
            | Self::LaunchAgent => "apps",
        }
    }

    pub fn default_risk(self) -> RiskLevel {
        match self {
            Self::TempPath
            | Self::UserCache
            | Self::AppCacheLeftover
            | Self::HomebrewCache
            | Self::XcodeDerivedData => RiskLevel::Safe,
            Self::UserLog
            | Self::AppLogLeftover
            | Self::BrowserDiskCache
            | Self::XcodeDeviceSupport
            | Self::XcodeArchives => RiskLevel::Review,
            Self::SimulatorDeviceSet
            | Self::ApplicationSupport
            | Self::PreferencePlist
            | Self::SandboxContainer
            | Self::GroupContainer
            | Self::SavedApplicationState
            | Self::WebKitData
            | Self::HttpStorage
            | Self::CookieStore
            | Self::LaunchAgent
            | Self::SystemLog => RiskLevel::Dangerous,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct Evidence {
    pub kind: String,
    pub detail: String,
}

impl Evidence {
    pub fn new(kind: impl Into<String>, detail: impl Into<String>) -> Self {
        Self {
            kind: kind.into(),
            detail: detail.into(),
        }
    }
}

pub const NEVER_DELETE_PREFIXES: &[&str] = &[
    ".Trash",
    ".git",
    ".ssh",
    ".gnupg",
    "Library/Keychains",
    "Library/Application Support/AddressBook",
    "Library/Application Support/MobileSync/Backup",
];

pub const NEVER_DELETE_ABSOLUTE_PREFIXES: &[&str] = &[
    "/Applications",
    "/System",
    "/usr",
    "/bin",
    "/sbin",
    "/private/var/db",
    "/private/var/vm",
    "/Library/Keychains",
    "/Network",
    "/cores",
];

pub fn is_protected_absolute_path(path: &str, home: &str) -> bool {
    let normalized = path.trim_end_matches('/');
    if NEVER_DELETE_ABSOLUTE_PREFIXES
        .iter()
        .any(|prefix| normalized == *prefix || normalized.starts_with(&format!("{prefix}/")))
    {
        return true;
    }

    NEVER_DELETE_PREFIXES.iter().any(|prefix| {
        let expanded = format!("{home}/{prefix}");
        normalized == expanded || normalized.starts_with(&format!("{expanded}/"))
    })
}

pub fn is_safe_tool_cache_name(name: &str) -> bool {
    matches!(
        name.to_ascii_lowercase().as_str(),
        "argmax-sdk-swift"
            | "bun"
            | "cargo"
            | "claude-cli-nodejs"
            | "esbuild"
            | "go-build"
            | "goimports"
            | "golangci-lint"
            | "ms-playwright"
            | "node-gyp"
            | "npm"
            | "org.swift.swiftpm"
            | "pip"
            | "pnpm"
            | "rust-analyzer"
            | "rustup"
            | "ruff"
            | "svelte-check-rs"
            | "swiftpm"
            | "turbo"
            | "typescript"
            | "uv"
            | "vite"
            | "yarn"
            | "zig"
    )
}

#[cfg(test)]
mod tests {
    use super::{CandidateClass, RiskLevel, is_protected_absolute_path, is_safe_tool_cache_name};

    #[test]
    fn risk_order_allows_expected_levels() {
        assert!(RiskLevel::Review.allows(RiskLevel::Safe));
        assert!(RiskLevel::Dangerous.allows(RiskLevel::Review));
        assert!(!RiskLevel::Safe.allows(RiskLevel::Dangerous));
    }

    #[test]
    fn candidate_classes_have_expected_defaults() {
        assert_eq!(CandidateClass::TempPath.default_risk(), RiskLevel::Safe);
        assert_eq!(
            CandidateClass::BrowserDiskCache.default_risk(),
            RiskLevel::Review
        );
        assert_eq!(
            CandidateClass::ApplicationSupport.default_risk(),
            RiskLevel::Dangerous
        );
    }

    #[test]
    fn protected_path_detection_blocks_home_and_system_roots() {
        let home = "/Users/example";
        assert!(is_protected_absolute_path(
            "/Users/example/.ssh/id_ed25519",
            home
        ));
        assert!(is_protected_absolute_path(
            "/System/Library/CoreServices",
            home
        ));
        assert!(!is_protected_absolute_path(
            "/Users/example/Library/Caches/com.example.app",
            home
        ));
    }

    #[test]
    fn safe_tool_cache_allowlist_matches_expected_names() {
        assert!(is_safe_tool_cache_name("bun"));
        assert!(is_safe_tool_cache_name("org.swift.swiftpm"));
        assert!(!is_safe_tool_cache_name("Adobe"));
    }
}
