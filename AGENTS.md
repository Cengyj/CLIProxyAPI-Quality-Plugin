# Model Quality Plugin Workspace

This workspace is for a standalone CLIProxyAPI native plugin with manual model evaluation and credential quality operations.

## Task specification

Read `PROMPT_CN.md` for the complete implementation task and `docs/RESEARCH_CN.md` for verified source findings. The latest direct user instruction takes precedence over these files.

The product action is **credential-wide quality pause**, followed only by explicit manual release. Model-level results are evidence; the first release does not implement groups or per-model isolation.

## Paths and source boundaries

- Workspace: `/Users/ceng/Downloads/CLIProxyAPI-Quality-Plugin`
- Plugin module and embedded UI: `plugin/`
- Editable host source, prepared when implementation starts: `workspaces/CLIProxyAPI/`
- Reference checkout, prepared when needed: `workspaces/sub2api/`
- Host patches and base manifest: `host-patches/`
- Documentation: `docs/`
- Build artifacts: `artifacts/`
- Existing host source reference: `/Users/ceng/Downloads/CLIProxyAPI-8.0.67`

Treat the existing host source reference as read-only. Make required host changes in the workspace copy. Read its `AGENTS.md` before preparing or editing host code and preserve those requirements in the copy. The source reference may lack Git metadata; record its actual baseline rather than assuming a branch or commit. Copy only source and required build assets, preserving source fidelity and excluding credentials, environment secrets, live configuration, logs, databases, binaries, and plugin state. Do not replace the reference with a newer upstream checkout without documenting the difference.

The plugin is a separate Go module using the public `/v8` SDK. Do not import the host's `internal` packages from the plugin. Maintain host extensions and integration adapters inside the host copy.

## Product invariants

- Manual evaluation does not pause or resume credentials automatically.
- A quality pause blocks new business executions for the entire credential and has durable ownership.
- Operator disable, quality pause, native cooldown, and other owners remain separate conditions.
- Quality-paused credentials must never be retested, manually or automatically. No bypass grants or automatic recovery exist. Stop every dependent rule and task, including use as control or judge.
- Only complete, comparable, explicitly incorrect evidence can advance a quality-failure decision. Infrastructure errors and unknown results do not constitute quality failures.
- Manual release removes only the matching owned pause; it does not promise the credential is otherwise usable. All dependent rules stay stopped, stale counters reset, and explicit rule resume is required.
- Default to observation. Scheduling and automatic pause require separate configuration switches. Recovery switches and retest intervals are removed.

## Engineering and verification

Follow the source repository's Go, JSON/Base64, thinking-pipeline, final-payload-barrier, logging, and network-timeout rules. Keep new business APIs under authenticated v8 management paths; public resource GETs serve static content only. Do not add or alter deprecated v0 management endpoints.

Use offline fake upstreams for development. Never copy live credentials into this workspace or use real model accounts during routine tests.

Test the plugin module separately from the host module, build the native shared library, and verify loading and lifecycle behavior. Run the required host formatting, tests, and compile checks after Go changes. Keep source-only host patches and their baseline evidence reproducible.

Use English code comments. New Chinese documentation must use an explicit `_CN.md` suffix.
