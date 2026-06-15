# Withings CLI Spec

## Name + One-liner
- Name: `Withings CLI`
- One-liner: Interact with Withings Health Solutions data and OAuth tokens from the CLI.

## Usage
```
withings [global flags] <subcommand> [args]
```

## Subcommands
- `withings auth ...` manage OAuth tokens
- `withings measures ...` weight/BP/body metrics
- `withings activity ...` activity summaries
- `withings sleep ...` sleep summaries
- `withings heart ...` heart data
- `withings api ...` low-level action-based requests (escape hatch)

## Global flags
- `-h, --help` show help and exit
- `--version` print version to stdout
- `-v, --verbose` increase diagnostic verbosity (repeatable: `-v/-vv`)
- `-q, --quiet` suppress non-error output
- `--json` machine-readable JSON output
- `--plain` stable line-based output (no tables, no colors)
- `--no-color` disable ANSI color
- `--no-input` disable prompts; fail if required input is missing
- `--config <path>` override config file path
- `--cloud <eu|us>` select API cloud (default `eu`)
- `--base-url <url>` override API base URL (advanced)

## I/O contract
- stdout: primary results (human or `--json`/`--plain`)
- stderr: errors, warnings, progress, diagnostics
- prompts only when stdin is a TTY and `--no-input` is not set
- `--json` outputs an envelope: `{ "ok": true|false, "data": ..., "meta": ... }`

## Exit codes
- `0` success
- `1` generic failure
- `2` invalid usage/flags
- `3` auth required or refresh failed
- `4` network/connectivity error
- `5` API error (non-2xx or Withings error code)

## Config / env / precedence
- precedence: flags > project config > user config > system
- user config: `~/.config/withings-cli/config.toml`
- project config (optional): `./withings-cli.toml`
- env vars:
  - `WITHINGS_CLIENT_ID`
  - `WITHINGS_CLIENT_SECRET` (secret; prefer env or prompt)
- client credentials are read from env only; the CLI does not store them in config files

## Auth commands
- `withings auth login`
  - performs browser OAuth with local callback server by default
  - requires `WITHINGS_CLIENT_ID` and `WITHINGS_CLIENT_SECRET`
  - exchanges the authorization code and stores tokens automatically
  - flags: `--redirect-uri <uri>`, `--no-open`, `--listen <addr:port>`
  - default callback URL: <http://127.0.0.1:9876/callback>
  - create client credentials at <https://developer.withings.com/dashboard/>
- `withings auth status` show token age/scopes/expiry
- `withings auth logout` delete stored tokens (requires confirmation or `--force`)
- access tokens are refreshed automatically when expired (requires `WITHINGS_CLIENT_ID` and `WITHINGS_CLIENT_SECRET`)

## Data commands (common flags)
- common flags: `--start <rfc3339|YYYY-MM-DD|epoch>`, `--end <rfc3339|YYYY-MM-DD|epoch>`, `--last-update <epoch>`, `--limit <n>`, `--offset <n>`, `--user-id <id>`
- output: tables by default; `--json` returns raw API `body`

### measures
- `withings measures get`
  - flags: `--type <list>` (e.g., `weight,bp_sys,bp_dia,fat_mass`)
    - accepted aliases: `weight`, `bodyweight`, `bp_sys`, `bp_dia`,
      `fat_mass`, `fat_mass_weight`, `fat_ratio`, `fat_free_mass`,
      `heart_rate`, `temp`, `temperature`, `spo2`, `body_temp`, `skin_temp`,
      `muscle_mass`,
      `hydration`, `bone_mass`, `pulse_wave_velocity` (or numeric IDs)
  - `--category <real|goal|1|2>`
  - `--units <metric|imperial>` unit system for table output (default `metric`); does not affect `--json`
  - `--last-update` cannot be combined with `--start` or `--end`
  - behavior: idempotent, read-only
  - table output columns: `time`, `type`, `value`, `unit`, `category`
  - `--plain` outputs tab-separated lines with a header row

### activity
- `withings activity get`
  - flags: `--date <YYYY-MM-DD>`, `--start/--end` for range
  - `--end` defaults to the current datetime when omitted
  - behavior: idempotent, read-only
  - table output columns: `date`, `steps`, `distance`, `calories`, `total_calories`, `active`, `elevation`, `soft`, `moderate`, `intense`
  - `--plain` outputs tab-separated lines with a header row

### sleep
- `withings sleep get`
  - flags: `--date`, `--start/--end`, `--model <1|2>` (if supported)
  - `--end` defaults to the current datetime when omitted
  - behavior: idempotent, read-only
  - table output columns: `start`, `end`, `duration`, `score`, `wakeups`, `model`
  - `--plain` outputs tab-separated lines with a header row

### heart
- `withings heart get`
  - flags: `--start/--end`, `--signal` (include signal metadata if available)
  - behavior: idempotent, read-only
  - table output columns: `time`, `heart_rate`, `model`, `device`, `signal_id`, `ecg`, `afib`, `signal`
  - `--plain` outputs tab-separated lines with a header row

## API escape hatch
- `withings api call --service <service> --action <action> --params <json>`
  - `--params` accepts a JSON object; use `@file.json` or `-` for stdin
  - `--dry-run` prints request URL/body without executing
  - use `--json` for raw response passthrough

## Safety rules
- `auth logout` requires confirmation unless `--force`
- prompts only when TTY and `--no-input` is not set
- `api call` supports `--dry-run` and warns on likely non-idempotent actions

## Examples
```bash
withings auth login
withings auth status
withings measures get --type weight,bp_sys,bp_dia --start 2025-12-23 --end 2025-12-30
withings activity get --date 2025-12-29 --json
withings sleep get --start 2025-12-01 --end 2025-12-31 --plain
withings api call --service measure --action getmeas --params @params.json --json
```
