#![forbid(unsafe_code)]

use std::fs;
use std::path::Path;

use assert_cmd::Command;
use serde_json::Value;
use tempfile::TempDir;

fn parse_single_line_json(output: &[u8]) -> Value {
    let stdout = String::from_utf8(output.to_vec()).expect("stdout utf-8");
    let lines = stdout.lines().collect::<Vec<_>>();
    assert_eq!(lines.len(), 1, "expected exactly one JSON line on stdout");
    serde_json::from_str(lines[0]).expect("valid JSON envelope")
}

fn base_command(home: &Path) -> Command {
    let mut command = Command::cargo_bin("ash").expect("ash binary");
    command.env("HOME", home);
    command.env("ASH_HOME", home.join(".tools").join("ash"));
    command
}

#[test]
fn tools_command_emits_the_stable_json_envelope() {
    let temp = TempDir::new().expect("tempdir");
    let output = base_command(temp.path())
        .args(["tools", "--json"])
        .assert()
        .success()
        .get_output()
        .stdout
        .clone();
    let envelope = parse_single_line_json(&output);

    assert_eq!(envelope["ok"], true);
    assert!(envelope.get("data").is_some());
    assert!(envelope.get("error").is_some());
    assert!(envelope.get("meta").is_some());
    assert_eq!(envelope["meta"]["tool"], "tools.list");
    assert!(
        envelope["data"]["tools"]
            .as_array()
            .expect("tools array")
            .iter()
            .any(|tool| tool["name"] == "scan.run")
    );
}

#[test]
fn health_command_emits_the_stable_json_envelope() {
    let temp = TempDir::new().expect("tempdir");
    let output = base_command(temp.path())
        .args(["health", "--json"])
        .assert()
        .success()
        .get_output()
        .stdout
        .clone();
    let envelope = parse_single_line_json(&output);

    assert_eq!(envelope["ok"], true);
    assert!(envelope.get("data").is_some());
    assert!(envelope.get("error").is_some());
    assert!(envelope.get("meta").is_some());
    assert_eq!(envelope["meta"]["tool"], "health.check");
    assert!(envelope["data"]["status"].is_string());
}

#[test]
fn config_validate_command_emits_the_stable_json_envelope() {
    let temp = TempDir::new().expect("tempdir");
    let output = base_command(temp.path())
        .args(["config", "validate", "--json"])
        .assert()
        .success()
        .get_output()
        .stdout
        .clone();
    let envelope = parse_single_line_json(&output);

    assert_eq!(envelope["ok"], true);
    assert!(envelope.get("data").is_some());
    assert!(envelope.get("error").is_some());
    assert!(envelope.get("meta").is_some());
    assert_eq!(envelope["meta"]["tool"], "config.validate");
    assert!(envelope["data"]["valid"].is_boolean());
}

#[test]
fn scan_uses_the_config_default_profile_when_profile_is_omitted() {
    let temp = TempDir::new().expect("tempdir");
    let ash_home = temp.path().join(".tools").join("ash");
    fs::create_dir_all(&ash_home).expect("ash home");
    fs::write(ash_home.join("config.toml"), "default_profile = \"full\"").expect("config");

    let output = base_command(temp.path())
        .args(["scan", "--json"])
        .assert()
        .success()
        .get_output()
        .stdout
        .clone();
    let envelope = parse_single_line_json(&output);

    assert_eq!(envelope["ok"], true);
    assert_eq!(envelope["data"]["plan"]["profile"], "full");
}
