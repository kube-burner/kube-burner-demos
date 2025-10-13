#!/usr/bin/env python3
"""
Sync load-related fields in tmp/demo-user-data.yaml from a .env file.

Only known environment variables are processed. Existing YAML comments and
unrelated keys remain untouched. Run this script each time you change .env to
propagate the values before invoking kube-burner.
"""

from __future__ import annotations

import argparse
import sys
from pathlib import Path


ENV_MAPPING = {
    "ENABLE_LOAD": {
        "yaml_key": "enableLoad",
        "formatter": "bool",
    },
    "BASELINE_PAUSE": {
        "yaml_key": "baselinePause",
        "formatter": "raw",
    },
    "LOAD_PAUSE": {
        "yaml_key": "loadPause",
        "formatter": "raw",
    },
    "LOAD_REPLICAS": {
        "yaml_key": "loadGeneratorReplicas",
        "formatter": "int",
    },
    "BACKEND_SCALE_REPLICAS": {
        "yaml_key": "backendScaleReplicas",
        "formatter": "int",
    },
    "LOAD_SCALE_DELAY": {
        "yaml_key": "loadScaleDelay",
        "formatter": "raw",
    },
    "LOAD_BASE_RPS": {
        "yaml_key": "loadGeneratorBaseRps",
        "formatter": "quoted",
    },
    "LOAD_RAMP_FACTOR": {
        "yaml_key": "loadGeneratorRampFactor",
        "formatter": "quoted",
    },
    "LOAD_RAMP_INTERVAL_SECONDS": {
        "yaml_key": "loadGeneratorRampIntervalSeconds",
        "formatter": "quoted",
    },
    "LOAD_RUN_DURATION_SECONDS": {
        "yaml_key": "loadGeneratorRunDurationSeconds",
        "formatter": "quoted",
    },
}


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Sync load configuration values from a .env file into demo-user-data.yaml"
    )
    parser.add_argument(
        "--env-file",
        default=".env",
        type=Path,
        help="Path to the .env file (default: %(default)s)",
    )
    parser.add_argument(
        "--metadata-file",
        default=Path("tmp/demo-user-data.yaml"),
        type=Path,
        help="Path to the kube-burner user metadata file (default: %(default)s)",
    )
    return parser.parse_args()


def load_env(path: Path) -> dict[str, str]:
    if not path.exists():
        return {}

    env: dict[str, str] = {}
    for raw_line in path.read_text().splitlines():
        line = raw_line.strip()
        if not line or line.startswith("#"):
            continue
        if "=" not in line:
            continue
        key, value = line.split("=", 1)
        env[key.strip()] = value.strip().strip('"').strip("'")
    return env


def format_value(kind: str, value: str) -> str:
    if kind == "quoted":
        return f'"{value}"'
    if kind == "raw":
        return value
    if kind == "int":
        try:
            return str(int(value))
        except ValueError as exc:
            raise ValueError(f"Expected integer for value '{value}'") from exc
    if kind == "bool":
        lowered = value.lower()
        if lowered in {"true", "1", "yes", "on"}:
            return "true"
        if lowered in {"false", "0", "no", "off"}:
            return "false"
        raise ValueError(
            f"Expected boolean-style value for '{value}' (true/false, yes/no, 1/0)"
        )
    raise ValueError(f"Unknown formatter kind '{kind}'")


def update_metadata(metadata_path: Path, env_values: dict[str, str]) -> list[str]:
    if not metadata_path.exists():
        raise FileNotFoundError(f"Metadata file not found: {metadata_path}")

    lines = metadata_path.read_text().splitlines()
    updates: dict[str, str] = {}
    for env_key, config in ENV_MAPPING.items():
        if env_key not in env_values:
            continue
        value = format_value(config["formatter"], env_values[env_key])
        updates[config["yaml_key"]] = value

    if not updates:
        return []

    updated_keys: list[str] = []
    for index, line in enumerate(lines):
        if ":" not in line:
            continue
        left, right = line.split(":", 1)
        stripped_key = left.strip()
        if stripped_key not in updates:
            continue
        indent_length = len(left) - len(left.lstrip())
        indent = left[:indent_length]
        new_value = updates[stripped_key]
        lines[index] = f"{indent}{stripped_key}: {new_value}"
        updated_keys.append(stripped_key)

    metadata_path.write_text("\n".join(lines) + "\n")
    return updated_keys


def main() -> int:
    args = parse_args()
    env_values = load_env(args.env_file)
    if not env_values:
        print(f"No .env values found at {args.env_file}, nothing to sync.")
        return 0

    try:
        updated_keys = update_metadata(args.metadata_file, env_values)
    except FileNotFoundError as exc:
        print(exc, file=sys.stderr)
        return 1
    except ValueError as exc:
        print(f"Error: {exc}", file=sys.stderr)
        return 1

    if not updated_keys:
        print("No matching environment variables to sync.")
    else:
        print(
            f"Updated {args.metadata_file} keys: {', '.join(sorted(updated_keys))}"
        )
    return 0


if __name__ == "__main__":
    sys.exit(main())
