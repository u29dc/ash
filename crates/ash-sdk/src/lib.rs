#![forbid(unsafe_code)]

pub mod bench;
pub mod clean;
pub mod config;
pub mod contracts;
pub mod error;
pub mod executor;
pub mod health;
pub mod inventory;
pub mod maintenance;
pub mod paths;
pub mod planner;
pub mod policy;
pub mod trash;

pub use bench::{
    FixtureSeedStats, ScanBenchmarkResult, benchmark_fixture_scan, benchmark_seeded_fixture_scan,
    seed_benchmark_fixture,
};
pub use clean::{CleanRequest, CleanRun, run_clean};
pub use config::{
    AppConfig, ConfigShowData, ConfigValidationCheck, ConfigValidationResult, load_config,
    show_config, validate_config,
};
pub use contracts::{
    ContractSpec, EnvelopeMeta, ErrorEnvelope, ErrorPayload, GlobalFlag, HealthStatus,
    OutputFieldSchema, OutputSchema, ParameterMeta, SuccessEnvelope, ToolMeta, contract_spec,
    global_flags, tool_registry,
};
pub use error::{AshError, ErrorCode, Result};
pub use executor::{
    ApplyRequest, ApplyResultItem, ExecutionReport, MaxRisk, apply_plan, parse_cleanup_plan_payload,
};
pub use health::{HealthCheck, HealthReport, run_health_checks};
pub use maintenance::{
    MaintenanceCatalog, MaintenanceCommand, MaintenanceCommandResult, MaintenanceRunRequest,
    list_maintenance_commands, run_maintenance_command,
};
pub use paths::{ResolvedPaths, resolve_paths};
pub use planner::{
    CleanupCandidate, CleanupPlan, OwnerStatus, PlanSummary, RunningProcess, ScanOptions,
    ScanProfile, Scope, TargetApp, scan,
};
pub use policy::{CandidateClass, Evidence, RiskLevel};

pub const SDK_NAME: &str = "ash-sdk";
pub const SDK_VERSION: &str = env!("CARGO_PKG_VERSION");
