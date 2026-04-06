use std::io;

use thiserror::Error;

pub type Result<T> = std::result::Result<T, AshError>;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ErrorCode {
    Runtime,
    ConfigInvalid,
    ConfigBlocked,
    PlatformBlocked,
    HealthBlocked,
    PlanInvalid,
    PlanDrift,
    SafetyBlocked,
    Unsupported,
    MaintenanceFailed,
}

impl ErrorCode {
    pub fn as_str(self) -> &'static str {
        match self {
            Self::Runtime => "RUNTIME_ERROR",
            Self::ConfigInvalid => "CONFIG_INVALID",
            Self::ConfigBlocked => "CONFIG_BLOCKED",
            Self::PlatformBlocked => "PLATFORM_BLOCKED",
            Self::HealthBlocked => "HEALTH_BLOCKED",
            Self::PlanInvalid => "PLAN_INVALID",
            Self::PlanDrift => "PLAN_DRIFT",
            Self::SafetyBlocked => "SAFETY_BLOCKED",
            Self::Unsupported => "UNSUPPORTED",
            Self::MaintenanceFailed => "MAINTENANCE_FAILED",
        }
    }

    pub fn exit_code(self) -> i32 {
        match self {
            Self::PlatformBlocked
            | Self::ConfigBlocked
            | Self::HealthBlocked
            | Self::PlanDrift
            | Self::SafetyBlocked
            | Self::Unsupported => 2,
            Self::Runtime | Self::ConfigInvalid | Self::PlanInvalid | Self::MaintenanceFailed => 1,
        }
    }
}

#[derive(Debug, Error)]
#[error("{message}")]
pub struct AshError {
    pub code: ErrorCode,
    pub message: String,
    pub hint: String,
}

impl AshError {
    pub fn new(code: ErrorCode, message: impl Into<String>, hint: impl Into<String>) -> Self {
        Self {
            code,
            message: message.into(),
            hint: hint.into(),
        }
    }

    pub fn runtime(message: impl Into<String>) -> Self {
        Self::new(
            ErrorCode::Runtime,
            message,
            "inspect stderr details and retry after correcting the failing path or command",
        )
    }
}

impl From<io::Error> for AshError {
    fn from(value: io::Error) -> Self {
        Self::runtime(value.to_string())
    }
}
