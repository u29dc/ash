#![forbid(unsafe_code)]

use std::fs;
use std::io::{self, Read};
use std::path::PathBuf;
use std::time::Instant;

use ash_sdk::{
    ApplyRequest, AshError, CleanRequest, CleanupPlan, ContractSpec, EnvelopeMeta, ErrorCode,
    ErrorEnvelope, ErrorPayload, GlobalFlag, MaintenanceRunRequest, MaxRisk, ScanOptions,
    ScanProfile, Scope, SuccessEnvelope, ToolMeta, apply_plan, contract_spec,
    list_maintenance_commands, load_config, parse_cleanup_plan_payload, resolve_paths, run_clean,
    run_health_checks, run_maintenance_command, scan, show_config, tool_registry, validate_config,
};
use clap::{Parser, Subcommand, ValueEnum};
use serde::Serialize;
use serde_json::json;

#[derive(Debug, Parser)]
#[command(name = "ash", version = ash_sdk::SDK_VERSION, disable_help_subcommand = true)]
struct Cli {
    #[arg(long, global = true, default_value_t = false)]
    json: bool,
    #[command(subcommand)]
    command: Command,
}

#[derive(Debug, Subcommand)]
enum Command {
    Tools {
        name: Option<String>,
    },
    Health,
    Config {
        #[command(subcommand)]
        command: ConfigCommand,
    },
    Scan {
        #[arg(long, value_enum)]
        profile: Option<ProfileArg>,
        #[arg(long = "scope", value_enum)]
        scopes: Vec<ScopeArg>,
        #[arg(long)]
        app: Option<String>,
        #[arg(long)]
        output: Option<PathBuf>,
    },
    Clean {
        #[arg(long, value_enum)]
        profile: Option<ProfileArg>,
        #[arg(long = "scope", value_enum)]
        scopes: Vec<ScopeArg>,
        #[arg(long)]
        app: Option<String>,
        #[arg(long, value_enum, default_value_t = MaxRiskArg::Safe)]
        max_risk: MaxRiskArg,
        #[arg(long, default_value_t = false)]
        dry_run: bool,
        #[arg(long)]
        save_plan: Option<PathBuf>,
        #[arg(long, default_value_t = false)]
        include_plan: bool,
    },
    Apply {
        #[arg(long)]
        plan: Option<PathBuf>,
        #[arg(long, value_enum, default_value_t = MaxRiskArg::Safe)]
        max_risk: MaxRiskArg,
        #[arg(long, default_value_t = false)]
        dry_run: bool,
    },
    Maintenance {
        #[command(subcommand)]
        command: MaintenanceCommandArg,
    },
}

#[derive(Debug, Subcommand)]
enum ConfigCommand {
    Show,
    Validate,
}

#[derive(Debug, Subcommand)]
enum MaintenanceCommandArg {
    List,
    Run {
        name: String,
        #[arg(long, default_value_t = false)]
        dry_run: bool,
    },
}

#[derive(Debug, Clone, Copy, ValueEnum)]
enum ProfileArg {
    Safe,
    Full,
}

impl From<ProfileArg> for ScanProfile {
    fn from(value: ProfileArg) -> Self {
        match value {
            ProfileArg::Safe => Self::Safe,
            ProfileArg::Full => Self::Full,
        }
    }
}

#[derive(Debug, Clone, Copy, ValueEnum)]
enum ScopeArg {
    Temp,
    Caches,
    Logs,
    Xcode,
    Homebrew,
    Browsers,
    Apps,
    All,
}

impl From<ScopeArg> for Scope {
    fn from(value: ScopeArg) -> Self {
        match value {
            ScopeArg::Temp => Self::Temp,
            ScopeArg::Caches => Self::Caches,
            ScopeArg::Logs => Self::Logs,
            ScopeArg::Xcode => Self::Xcode,
            ScopeArg::Homebrew => Self::Homebrew,
            ScopeArg::Browsers => Self::Browsers,
            ScopeArg::Apps => Self::Apps,
            ScopeArg::All => Self::All,
        }
    }
}

#[derive(Debug, Clone, Copy, ValueEnum)]
enum MaxRiskArg {
    Safe,
    Review,
    Dangerous,
}

impl From<MaxRiskArg> for MaxRisk {
    fn from(value: MaxRiskArg) -> Self {
        match value {
            MaxRiskArg::Safe => Self::Safe,
            MaxRiskArg::Review => Self::Review,
            MaxRiskArg::Dangerous => Self::Dangerous,
        }
    }
}

#[derive(Debug, Clone, Default)]
struct MetaExtras {
    count: Option<usize>,
    total: Option<usize>,
    has_more: Option<bool>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct ToolsListData {
    contract: ContractSpec,
    global_flags: Vec<GlobalFlag>,
    tools: Vec<ToolMeta>,
}

#[derive(Debug, Serialize)]
#[serde(rename_all = "camelCase")]
struct ToolDetailData {
    tool: ToolMeta,
}

#[derive(Debug)]
struct CleanCommandRequest {
    profile: Option<ScanProfile>,
    scopes: Vec<Scope>,
    app: Option<String>,
    max_risk: MaxRisk,
    dry_run: bool,
    save_plan: Option<PathBuf>,
    include_plan: bool,
}

fn main() -> std::process::ExitCode {
    let start = Instant::now();
    let cli = Cli::parse();
    let paths = match resolve_paths() {
        Ok(paths) => paths,
        Err(error) => return to_exit_code(emit_error("bootstrap", &error, start)),
    };

    let result = match cli.command {
        Command::Tools { name } => handle_tools(name, start),
        Command::Health => {
            let report = run_health_checks(&paths);
            emit_success("health.check", &report, start, MetaExtras::default(), 0)
        }
        Command::Config { command } => match command {
            ConfigCommand::Show => match show_config(&paths) {
                Ok(data) => emit_success("config.show", &data, start, MetaExtras::default(), 0),
                Err(error) => emit_error("config.show", &error, start),
            },
            ConfigCommand::Validate => {
                let result = validate_config(&paths);
                emit_success("config.validate", &result, start, MetaExtras::default(), 0)
            }
        },
        Command::Scan {
            profile,
            scopes,
            app,
            output,
        } => handle_scan(
            &paths,
            profile.map(ScanProfile::from),
            scopes.into_iter().map(Scope::from).collect(),
            app,
            output,
            start,
        ),
        Command::Clean {
            profile,
            scopes,
            app,
            max_risk,
            dry_run,
            save_plan,
            include_plan,
        } => handle_clean(
            &paths,
            CleanCommandRequest {
                profile: profile.map(ScanProfile::from),
                scopes: scopes.into_iter().map(Scope::from).collect(),
                app,
                max_risk: max_risk.into(),
                dry_run,
                save_plan,
                include_plan,
            },
            start,
        ),
        Command::Apply {
            plan,
            max_risk,
            dry_run,
        } => handle_apply(&paths, plan, max_risk.into(), dry_run, start),
        Command::Maintenance { command } => match command {
            MaintenanceCommandArg::List => {
                let catalog = list_maintenance_commands();
                emit_success(
                    "maintenance.list",
                    &catalog,
                    start,
                    MetaExtras {
                        count: Some(catalog.commands.len()),
                        total: Some(catalog.commands.len()),
                        has_more: Some(false),
                    },
                    0,
                )
            }
            MaintenanceCommandArg::Run { name, dry_run } => {
                match run_maintenance_command(MaintenanceRunRequest { name, dry_run }) {
                    Ok(result) => {
                        emit_success("maintenance.run", &result, start, MetaExtras::default(), 0)
                    }
                    Err(error) => emit_error("maintenance.run", &error, start),
                }
            }
        },
    };

    to_exit_code(result)
}

fn handle_tools(name: Option<String>, start: Instant) -> i32 {
    let tools = tool_registry();
    if let Some(name) = name {
        match tools.into_iter().find(|tool| tool.name == name) {
            Some(tool) => emit_success(
                "tools.describe",
                &ToolDetailData { tool },
                start,
                MetaExtras::default(),
                0,
            ),
            None => emit_error(
                "tools.describe",
                &AshError::new(
                    ErrorCode::Unsupported,
                    format!("unknown tool metadata entry: {name}"),
                    "run `ash tools --json` to inspect the supported tool catalog",
                ),
                start,
            ),
        }
    } else {
        emit_success(
            "tools.list",
            &ToolsListData {
                contract: contract_spec(),
                global_flags: ash_sdk::global_flags(),
                tools: tools.clone(),
            },
            start,
            MetaExtras {
                count: Some(tools.len()),
                total: Some(tools.len()),
                has_more: Some(false),
            },
            0,
        )
    }
}

fn handle_scan(
    paths: &ash_sdk::ResolvedPaths,
    profile: Option<ScanProfile>,
    scopes: Vec<Scope>,
    app: Option<String>,
    output: Option<PathBuf>,
    start: Instant,
) -> i32 {
    let profile = match profile {
        Some(profile) => profile,
        None => match load_config(paths) {
            Ok(config) => config.default_profile,
            Err(error) => return emit_error("scan.run", &error, start),
        },
    };
    match scan(
        paths,
        ScanOptions {
            app_target: app,
            profile,
            scopes,
        },
    ) {
        Ok(plan) => {
            if let Some(output) = output {
                let summary = plan.summary.clone();
                let written_plan_path = match write_plan_file(&plan, &output) {
                    Ok(path) => path,
                    Err(error) => return emit_error("scan.run", &error, start),
                };
                emit_success(
                    "scan.run",
                    &json!({
                        "plan": plan,
                        "summary": summary,
                        "writtenPlanPath": written_plan_path,
                    }),
                    start,
                    MetaExtras::default(),
                    0,
                )
            } else {
                emit_success(
                    "scan.run",
                    &json!({
                        "plan": plan,
                        "summary": plan.summary,
                        "writtenPlanPath": serde_json::Value::Null,
                    }),
                    start,
                    MetaExtras::default(),
                    0,
                )
            }
        }
        Err(error) => emit_error("scan.run", &error, start),
    }
}

fn handle_clean(
    paths: &ash_sdk::ResolvedPaths,
    request: CleanCommandRequest,
    start: Instant,
) -> i32 {
    let profile = match request.profile {
        Some(profile) => profile,
        None => match load_config(paths) {
            Ok(config) => config.default_profile,
            Err(error) => return emit_error("clean.run", &error, start),
        },
    };

    match run_clean(
        paths,
        CleanRequest {
            scan: ScanOptions {
                app_target: request.app,
                profile,
                scopes: request.scopes,
            },
            max_risk: request.max_risk,
            dry_run: request.dry_run,
        },
    ) {
        Ok(run) => {
            let saved_plan_path = match request.save_plan {
                Some(path) => match write_plan_file(&run.plan, &path) {
                    Ok(path) => Some(path),
                    Err(error) => return emit_error("clean.run", &error, start),
                },
                None => None,
            };
            emit_success(
                "clean.run",
                &clean_response_data(
                    run.plan,
                    run.execution,
                    saved_plan_path,
                    request.include_plan,
                ),
                start,
                MetaExtras::default(),
                0,
            )
        }
        Err(error) => emit_error("clean.run", &error, start),
    }
}

fn clean_response_data(
    plan: CleanupPlan,
    execution: ash_sdk::ExecutionReport,
    saved_plan_path: Option<String>,
    include_plan: bool,
) -> serde_json::Value {
    let full_plan = include_plan.then(|| serde_json::to_value(&plan).expect("plan json"));
    json!({
        "plan": {
            "planHash": plan.plan_hash,
            "profile": plan.profile,
            "scopes": plan.scopes,
            "targetApp": plan.target_app,
            "summary": plan.summary,
        },
        "execution": execution,
        "savedPlanPath": saved_plan_path,
        "fullPlan": full_plan,
    })
}

fn write_plan_file(plan: &CleanupPlan, output: &PathBuf) -> ash_sdk::Result<String> {
    if let Some(parent) = output.parent() {
        fs::create_dir_all(parent).map_err(|error| {
            AshError::runtime(format!(
                "failed to create plan output directory {}: {error}",
                parent.display()
            ))
        })?;
    }
    let serialized = serde_json::to_vec_pretty(plan)
        .map_err(|error| AshError::runtime(format!("failed to serialize plan JSON: {error}")))?;
    fs::write(output, serialized).map_err(|error| {
        AshError::runtime(format!(
            "failed to write plan file {}: {error}",
            output.display()
        ))
    })?;
    Ok(output.display().to_string())
}

fn handle_apply(
    paths: &ash_sdk::ResolvedPaths,
    plan_path: Option<PathBuf>,
    max_risk: MaxRisk,
    dry_run: bool,
    start: Instant,
) -> i32 {
    let plan = match read_plan(plan_path) {
        Ok(plan) => plan,
        Err(error) => return emit_error("apply.run", &error, start),
    };
    match apply_plan(
        paths,
        ApplyRequest {
            dry_run,
            max_risk,
            plan,
        },
    ) {
        Ok(report) => emit_success("apply.run", &report, start, MetaExtras::default(), 0),
        Err(error) => emit_error("apply.run", &error, start),
    }
}

fn read_plan(plan_path: Option<PathBuf>) -> ash_sdk::Result<CleanupPlan> {
    let payload = if let Some(path) = plan_path {
        fs::read_to_string(&path).map_err(|error| {
            AshError::new(
                ErrorCode::PlanInvalid,
                format!("failed to read plan file {}: {error}", path.display()),
                "pass a readable plan file or pipe the plan JSON via stdin",
            )
        })?
    } else {
        let mut buffer = String::new();
        io::stdin().read_to_string(&mut buffer).map_err(|error| {
            AshError::new(
                ErrorCode::PlanInvalid,
                format!("failed to read plan JSON from stdin: {error}"),
                "pass `--plan <file>` or pipe a cleanup plan to stdin",
            )
        })?;
        if buffer.trim().is_empty() {
            return Err(AshError::new(
                ErrorCode::PlanInvalid,
                "no plan JSON was provided",
                "pass `--plan <file>` or pipe a cleanup plan to stdin",
            ));
        }
        buffer
    };
    parse_cleanup_plan_payload(&payload)
}

fn emit_success<T: Serialize>(
    tool: &str,
    data: &T,
    start: Instant,
    extras: MetaExtras,
    exit_code: i32,
) -> i32 {
    let mut meta = EnvelopeMeta::new(tool, start.elapsed().as_millis() as u64);
    meta.count = extras.count;
    meta.total = extras.total;
    meta.has_more = extras.has_more;
    let envelope = SuccessEnvelope::new(data, meta);
    match serde_json::to_string(&envelope) {
        Ok(json) => {
            println!("{json}");
            exit_code
        }
        Err(error) => emit_error(
            tool,
            &AshError::runtime(format!("failed to serialize success envelope: {error}")),
            start,
        ),
    }
}

fn emit_error(tool: &str, error: &AshError, start: Instant) -> i32 {
    let envelope = ErrorEnvelope {
        ok: false,
        data: None,
        error: ErrorPayload::new(error.code.as_str(), error.to_string(), error.hint.clone()),
        meta: EnvelopeMeta::new(tool, start.elapsed().as_millis() as u64),
    };
    match serde_json::to_string(&envelope) {
        Ok(json) => println!("{json}"),
        Err(serialize_error) => println!(
            "{{\"ok\":false,\"data\":null,\"error\":{{\"code\":\"RUNTIME_ERROR\",\"message\":\"failed to serialize error envelope: {serialize_error}\",\"hint\":\"check stderr for details\"}},\"meta\":{{\"tool\":\"{tool}\",\"elapsed\":{}}}}}",
            start.elapsed().as_millis()
        ),
    }
    error.code.exit_code()
}

fn to_exit_code(code: i32) -> std::process::ExitCode {
    std::process::ExitCode::from(match code {
        0 => 0,
        2 => 2,
        _ => 1,
    })
}
