use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct EnvelopeMeta {
    pub tool: String,
    pub elapsed: u64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub count: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub total: Option<usize>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub has_more: Option<bool>,
}

impl EnvelopeMeta {
    pub fn new(tool: impl Into<String>, elapsed: u64) -> Self {
        Self {
            tool: tool.into(),
            elapsed,
            count: None,
            total: None,
            has_more: None,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct SuccessEnvelope<T> {
    pub ok: bool,
    pub data: T,
    pub error: Option<ErrorPayload>,
    pub meta: EnvelopeMeta,
}

impl<T> SuccessEnvelope<T> {
    pub fn new(data: T, meta: EnvelopeMeta) -> Self {
        Self {
            ok: true,
            data,
            error: None,
            meta,
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ErrorPayload {
    pub code: String,
    pub message: String,
    pub hint: String,
}

impl ErrorPayload {
    pub fn new(
        code: impl Into<String>,
        message: impl Into<String>,
        hint: impl Into<String>,
    ) -> Self {
        Self {
            code: code.into(),
            message: message.into(),
            hint: hint.into(),
        }
    }
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ErrorEnvelope {
    pub ok: bool,
    pub data: Option<serde_json::Value>,
    pub error: ErrorPayload,
    pub meta: EnvelopeMeta,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct ParameterMeta {
    pub name: String,
    #[serde(rename = "type")]
    pub param_type: String,
    pub required: bool,
    pub description: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct OutputFieldSchema {
    #[serde(rename = "type")]
    pub field_type: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
}

pub type OutputSchema = BTreeMap<String, OutputFieldSchema>;
pub type InputSchema = BTreeMap<String, OutputFieldSchema>;

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
pub struct GlobalFlag {
    pub name: String,
    pub description: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ToolMeta {
    pub name: String,
    pub command: String,
    pub category: String,
    pub description: String,
    pub parameters: Vec<ParameterMeta>,
    pub output_fields: Vec<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub output_schema: Option<OutputSchema>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub input_schema: Option<InputSchema>,
    pub idempotent: bool,
    pub read_only: bool,
    pub supports_json: bool,
    pub interactive_only: bool,
    pub rate_limit: Option<String>,
    pub example: String,
}

#[derive(Debug, Clone, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "camelCase")]
pub struct ContractSpec {
    pub schema_version: String,
    pub tool_catalog_version: String,
    pub envelope_keys: Vec<String>,
    pub exit_codes: BTreeMap<String, i32>,
    pub global_flags: Vec<GlobalFlag>,
    pub tool_metadata_fields: Vec<String>,
}

#[derive(Debug, Clone, Copy, Serialize, Deserialize, PartialEq, Eq)]
#[serde(rename_all = "lowercase")]
pub enum HealthStatus {
    Ready,
    Degraded,
    Blocked,
}

fn schema_field(field_type: &str, description: &str) -> OutputFieldSchema {
    OutputFieldSchema {
        field_type: field_type.to_string(),
        description: Some(description.to_string()),
    }
}

fn string_param(name: &str, description: &str, required: bool) -> ParameterMeta {
    ParameterMeta {
        name: name.to_string(),
        param_type: "string".to_string(),
        required,
        description: description.to_string(),
    }
}

fn bool_param(name: &str, description: &str, required: bool) -> ParameterMeta {
    ParameterMeta {
        name: name.to_string(),
        param_type: "boolean".to_string(),
        required,
        description: description.to_string(),
    }
}

fn enum_param(name: &str, description: &str, values: &[&str], required: bool) -> ParameterMeta {
    ParameterMeta {
        name: name.to_string(),
        param_type: format!("enum<{}>", values.join("|")),
        required,
        description: description.to_string(),
    }
}

pub fn global_flags() -> Vec<GlobalFlag> {
    vec![GlobalFlag {
        name: "--json".to_string(),
        description: "Accepted for explicit machine-mode calls; JSON is the default and only public output mode in v1."
            .to_string(),
    }]
}

pub fn contract_spec() -> ContractSpec {
    ContractSpec {
        schema_version: "1".to_string(),
        tool_catalog_version: "1".to_string(),
        envelope_keys: vec![
            "ok".to_string(),
            "data".to_string(),
            "error".to_string(),
            "meta".to_string(),
        ],
        exit_codes: BTreeMap::from([
            ("success".to_string(), 0),
            ("failure".to_string(), 1),
            ("blocked".to_string(), 2),
        ]),
        global_flags: global_flags(),
        tool_metadata_fields: vec![
            "name".to_string(),
            "command".to_string(),
            "category".to_string(),
            "description".to_string(),
            "parameters".to_string(),
            "outputFields".to_string(),
            "outputSchema".to_string(),
            "inputSchema".to_string(),
            "idempotent".to_string(),
            "readOnly".to_string(),
            "supportsJson".to_string(),
            "interactiveOnly".to_string(),
            "rateLimit".to_string(),
            "example".to_string(),
        ],
    }
}

pub fn tool_registry() -> Vec<ToolMeta> {
    let mut tools = vec![
        ToolMeta {
            name: "tools.list".to_string(),
            command: "ash tools --json".to_string(),
            category: "meta".to_string(),
            description: "List the full ash command catalog and global flags.".to_string(),
            parameters: vec![],
            output_fields: vec!["tools".to_string(), "globalFlags".to_string()],
            output_schema: Some(BTreeMap::from([
                ("tools".to_string(), schema_field("array<object>", "Tool metadata entries")),
                (
                    "globalFlags".to_string(),
                    schema_field("array<object>", "Global flag metadata"),
                ),
            ])),
            input_schema: None,
            idempotent: true,
            read_only: true,
            supports_json: true,
            interactive_only: false,
            rate_limit: None,
            example: "ash tools --json".to_string(),
        },
        ToolMeta {
            name: "tools.describe".to_string(),
            command: "ash tools <name> --json".to_string(),
            category: "meta".to_string(),
            description: "Show one tool's metadata entry from the registry.".to_string(),
            parameters: vec![string_param("name", "Dotted tool name.", true)],
            output_fields: vec!["tool".to_string()],
            output_schema: Some(BTreeMap::from([(
                "tool".to_string(),
                schema_field("object", "One registry entry"),
            )])),
            input_schema: None,
            idempotent: true,
            read_only: true,
            supports_json: true,
            interactive_only: false,
            rate_limit: None,
            example: "ash tools scan.run --json".to_string(),
        },
        ToolMeta {
            name: "health.check".to_string(),
            command: "ash health --json".to_string(),
            category: "meta".to_string(),
            description: "Return readiness, degraded states, and actionable remediation.".to_string(),
            parameters: vec![],
            output_fields: vec![
                "status".to_string(),
                "checks".to_string(),
                "paths".to_string(),
            ],
            output_schema: Some(BTreeMap::from([
                ("status".to_string(), schema_field("string", "Overall readiness status")),
                (
                    "checks".to_string(),
                    schema_field("array<object>", "Health check results"),
                ),
                (
                    "paths".to_string(),
                    schema_field("object", "Resolved runtime paths"),
                ),
            ])),
            input_schema: None,
            idempotent: true,
            read_only: true,
            supports_json: true,
            interactive_only: false,
            rate_limit: None,
            example: "ash health --json".to_string(),
        },
        ToolMeta {
            name: "config.show".to_string(),
            command: "ash config show --json".to_string(),
            category: "meta".to_string(),
            description: "Return the effective config, source path, and resolved runtime paths.".to_string(),
            parameters: vec![],
            output_fields: vec![
                "config".to_string(),
                "configPath".to_string(),
                "ashHome".to_string(),
                "userHome".to_string(),
            ],
            output_schema: None,
            input_schema: None,
            idempotent: true,
            read_only: true,
            supports_json: true,
            interactive_only: false,
            rate_limit: None,
            example: "ash config show --json".to_string(),
        },
        ToolMeta {
            name: "config.validate".to_string(),
            command: "ash config validate --json".to_string(),
            category: "meta".to_string(),
            description: "Validate the runtime config file and return structured warnings or errors.".to_string(),
            parameters: vec![],
            output_fields: vec!["valid".to_string(), "checks".to_string()],
            output_schema: None,
            input_schema: None,
            idempotent: true,
            read_only: true,
            supports_json: true,
            interactive_only: false,
            rate_limit: None,
            example: "ash config validate --json".to_string(),
        },
        ToolMeta {
            name: "scan.run".to_string(),
            command: "ash scan [--profile <profile>] --json".to_string(),
            category: "scan".to_string(),
            description: "Generate a cleanup plan without mutating state.".to_string(),
            parameters: vec![
                enum_param(
                    "profile",
                    "Optional scan profile. Defaults to the config default profile.",
                    &["safe", "full"],
                    false,
                ),
                string_param(
                    "--scope",
                    "Optional repeated scope filter: temp|caches|logs|xcode|homebrew|browsers|apps|all.",
                    false,
                ),
                string_param(
                    "--app",
                    "Optional bundle id or .app path for targeted app planning.",
                    false,
                ),
                string_param("--output", "Optional path to write the generated plan JSON.", false),
            ],
            output_fields: vec![
                "plan".to_string(),
                "summary".to_string(),
                "writtenPlanPath".to_string(),
            ],
            output_schema: None,
            input_schema: None,
            idempotent: true,
            read_only: true,
            supports_json: true,
            interactive_only: false,
            rate_limit: None,
            example: "ash scan --profile safe --json".to_string(),
        },
        ToolMeta {
            name: "apply.run".to_string(),
            command: "ash apply --plan <file> --json".to_string(),
            category: "apply".to_string(),
            description: "Execute a previously generated cleanup plan with risk gating and plan verification.".to_string(),
            parameters: vec![
                string_param("--plan", "Path to a cleanup plan JSON file. If omitted, stdin is read.", false),
                enum_param(
                    "--max-risk",
                    "Highest risk tier allowed during execution.",
                    &["safe", "review", "dangerous"],
                    false,
                ),
                bool_param("--dry-run", "Validate and report execution without moving files.", false),
            ],
            output_fields: vec![
                "planHash".to_string(),
                "dryRun".to_string(),
                "movedCount".to_string(),
                "blockedCount".to_string(),
                "failedCount".to_string(),
                "items".to_string(),
            ],
            output_schema: None,
            input_schema: Some(BTreeMap::from([(
                "plan".to_string(),
                schema_field("object", "Cleanup plan JSON"),
            )])),
            idempotent: false,
            read_only: false,
            supports_json: true,
            interactive_only: false,
            rate_limit: None,
            example: "ash apply --plan ./plan.json --dry-run --json".to_string(),
        },
        ToolMeta {
            name: "maintenance.list".to_string(),
            command: "ash maintenance list --json".to_string(),
            category: "maintenance".to_string(),
            description: "List the supported maintenance commands and their metadata.".to_string(),
            parameters: vec![],
            output_fields: vec!["commands".to_string()],
            output_schema: None,
            input_schema: None,
            idempotent: true,
            read_only: true,
            supports_json: true,
            interactive_only: false,
            rate_limit: None,
            example: "ash maintenance list --json".to_string(),
        },
        ToolMeta {
            name: "maintenance.run".to_string(),
            command: "ash maintenance run <name> --json".to_string(),
            category: "maintenance".to_string(),
            description: "Run one maintenance command in a structured, non-interactive path.".to_string(),
            parameters: vec![
                string_param("name", "Maintenance command name.", true),
                bool_param("--dry-run", "Report the command without executing it.", false),
            ],
            output_fields: vec![
                "name".to_string(),
                "dryRun".to_string(),
                "success".to_string(),
                "output".to_string(),
            ],
            output_schema: None,
            input_schema: None,
            idempotent: false,
            read_only: false,
            supports_json: true,
            interactive_only: false,
            rate_limit: None,
            example: "ash maintenance run dns.flush --json".to_string(),
        },
    ];

    tools.sort_by(|left, right| match left.category.cmp(&right.category) {
        std::cmp::Ordering::Equal => left.name.cmp(&right.name),
        ordering => ordering,
    });
    tools
}

#[cfg(test)]
mod tests {
    use super::{contract_spec, tool_registry};

    #[test]
    fn tool_registry_is_sorted() {
        let registry = tool_registry();
        let mut sorted = registry.clone();
        sorted.sort_by(|left, right| match left.category.cmp(&right.category) {
            std::cmp::Ordering::Equal => left.name.cmp(&right.name),
            ordering => ordering,
        });
        assert_eq!(registry, sorted);
    }

    #[test]
    fn contract_spec_contains_expected_envelope_keys() {
        let spec = contract_spec();
        assert_eq!(spec.envelope_keys, vec!["ok", "data", "error", "meta"]);
    }
}
