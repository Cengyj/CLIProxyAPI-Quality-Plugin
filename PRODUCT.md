# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

The primary user is a single self-hosting administrator who already knows CLIProxyAPI (CPA), evaluates model output, and manages the quality of their own credentials. The user confirmed this audience on 2026-11-14. Team administration and onboarding for a broad public audience are not confirmed primary use cases.

## Product Purpose

Model Quality is a standalone CPA native plugin with an embedded web quality console. It helps the administrator collect comparable model-output evidence, distinguish incorrect answers from execution problems, and apply a credential-wide quality pause through explicitly enabled quality rules.

Success means the administrator can understand what was tested, whether the evidence supports a decision, which credential is affected, and which restrictions remain after a manual release. Manual evaluation itself never pauses or releases credentials automatically.

## Positioning

Model-level results provide evidence; the host enforces the action against the entire credential. Quality pauses have durable ownership and remain separate from operator disable, native cooldown, and restrictions held by other owners. The plugin uses the host's normal model execution pipeline with a fixed credential instead of creating a separate provider client.

A paused credential cannot be tested again, including as a control or judge. Only an explicit authenticated administrator action can release the matching owned pause. Release leaves dependent rules stopped and does not promise that the credential is otherwise usable.

## Operating Context

The administrator runs a standalone CPA host with the required extensions and opens the plugin's browser interface. The existing implementation uses plain HTML, CSS, and JavaScript ES modules embedded in a separate Go module and native C ABI library. It has no frontend framework, external font service, runtime CDN, or analytics dependency.

The three independent workspaces are:

- **IQ question testing** (current UI label: 智商测试): select a credential, choose the versioned candy logic or pelican HTML question (or a custom question), edit the prompt if desired, select one model and reasoning effort, request 1–8 independent answers, and read the responses below the same form. Manual runs do not score automatically or invoke a control/judge credential. Editing built-in text submits a custom question without inheriting its answer. Copy, isolated HTML preview, cancellation, eligible rerun, redacted download and history remain available. Legacy questions, presets and multi-model histories remain readable through their existing APIs; the current manual surface does not expose a general evaluation-configuration workflow.
- **Quality operations** (质量运维): configure observation and pause rules, explicitly run or schedule detection, inspect credential restrictions and evidence, pause or resume detection rules, review audit history and budgets, and preview then manually release an owned credential pause.

- **OpenAI login** (OpenAI 登录): manage locally persisted OpenAI accounts imported as email/password/TOTP, explicitly authorize and bind CPA credentials, and inspect locally recorded CPA authentication state. Separate global inspection, global automatic reauthorization and per-account consent enable bounded automatic repair only for verified bindings with explicit credential-level auth failures. Inspection defaults to 15 minutes (5–1440 configurable), with automation off. First and manual authorization require candidate confirmation. Temporary login and job history remain separate subviews; authentication repair never releases quality pauses or resumes rules.

Tasks run in the background independently of the browser. Closing the page does not cancel a task. An explicit cancellation uses the host operation lifecycle. Interrupted tasks are retained after restart without automatically replaying potentially billable model requests.

The administrator connects using the existing management key. After successful authentication, the plugin stores that key as plaintext in its own endpoint-scoped localStorage entry and restores the connection on reload, reopen, or browser back navigation. Explicit disconnect and HTTP 401 clear the saved key; transient connection failures retain it. Storage restrictions fall back to page memory with a visible explanation. The plugin never reads the parent CPA authentication store, and account passwords, TOTP secrets and OAuth callbacks remain outside browser storage. The public `/v0/resource/plugins/model-quality/index.html` entry serves static content; data and actions use authenticated same-origin `/v8/management/plugins/model-quality/` APIs. Opening the page, connecting, or saving configuration does not call a model.

## Capabilities and Constraints

### Credential and rule semantics

- Default to observation. Scheduling and automatic pause require separate explicit configuration, including the applicable global and rule controls. There is no automatic recovery switch or retest interval.
- A quality pause blocks new business executions for every model on the credential. Business transmissions already admitted by the host may finish naturally.
- Pausing a credential stops all dependent detection rules, queued tasks, and ongoing evaluations, including dependencies that use it as a control or judge. There are no manual or automatic retests, bypass grants, or automatic recovery paths.
- Manual release removes only the pause with matching ownership, identity, and action version. Dependent rules remain stopped, old failure counters are cleared, and an explicit rule resume is required before new detection. Operator disable, native cooldown, and other owners' restrictions remain effective.
- Pausing a detection rule does not release a credential pause. Rule state, operator disable, quality pause, and effective availability must remain distinct in language and behavior.
- Pauses survive restart, plugin disable, and uninstall. The host retains an authenticated manual release path when the plugin is unavailable.
- The current release has no groups or per-model isolation. At most one rule with the `pause_credential` action may target a credential, even when its automatic pause switch is off; manual evaluations and observation rules can coexist.

### Evidence and interpretation

- Keep execution status, quality verdict, and action status separate. A successful HTTP response does not establish a correct answer.
- Only complete, comparable, explicitly incorrect evidence advances a quality-failure decision. By default, every selected detection model must fail in each of three consecutive valid rounds before a credential pause. A model round requires all planned samples to be complete, comparable, and decidable; at least one incorrect sample then constitutes failure.
- Missing samples, unknown outcomes, mixed model outcomes, or changed conditions break consecutive failure evidence. Network errors, authentication failures, rate limits, server errors, refusal, empty or truncated output, cancellation, and judge failures do not count as incorrect answers.
- “All models failed” means all selected detection models, not every model the credential can serve. Random-question automatic decisions additionally require a valid passing control under the same comparable conditions.
- Show quality pass rate alongside coverage. Unknown timing, reasoning capability, or usage remains unknown; missing usage is not zero cost. `response_model` is an upstream label, not proof of model identity. There is no calibrated general IQ score.

### Deployment, safety, and scope

- The current plugin requires a patched CPA host with quality capability v2; it is not plug-and-play with the original 8.0.67 source archive. Current platform verification covers macOS arm64; other platforms require their own build and validation.
- Automatic decision evidence currently covers the native OpenAI-compatible chat/completions execution paths. Unverified paths remain observation-only. Home quality actions are unsupported; the plugin must respect Home's control boundary.
- Preserve fixed credential selection, host model mapping, thinking configuration, and the final payload rules. Conditions that cannot be proven comparable cannot support automatic pause.
- Keep calls, samples, output, concurrency, queues, storage, and known usage bounded. Development and demonstrations use offline fake upstreams and isolated fake credentials, never real model accounts or copied production credentials.
- Model-generated HTML/SVG is untrusted. Show escaped source and an isolated preview without scripts, external network access, forms, navigation, or access to the parent page.
- Never expose management keys or raw credentials through URLs or logs. The administrator explicitly requested persistence of the plugin management key in its scoped browser storage entry; account passwords, TOTP secrets and OAuth callbacks remain outside browser storage. At the administrator’s explicit request, OpenAI management views show full emails and identity IDs; opening account details displays saved passwords/TOTP in plaintext controls. Saved login materials and login proxy settings persist as plaintext in the host’s local database; no encryption initialization or storage key is required. Management authentication, restricted file permissions and transient secret handling remain intact. Task reports include full identity metadata and fixed diagnostic codes but do not include passwords, TOTP seeds or tokens. Production prompts and answers are not stored by default.
- Account groups, automatic credential deletion, external notifications, BPS, Jev, ModelTrace, and Codex ticket probes are outside the current core scope.

## Brand Commitments

The existing product name is **Model Quality**, plugin ID `model-quality`, with the Chinese descriptor 模型质量控制台. The interface currently uses Simplified Chinese and the three workspace labels recorded above. Preserve the precise distinctions between 暂停检测规则, 质量暂停, 人工禁用, 有效可用状态, and 人工解除.

The interface follows the CPA Management Center plugin-management page and shared component language. The 2026-11-14 host-alignment instruction supersedes the previous color-only inheritance: warm light, white and warm dark themes, buttons, fields, cards, underlined navigation, switches, tables and dialogs share clear host references. Contrast and mobile target sizes may be adapted for readability. The CPA iframe is the primary setting and relies on the host shell; direct access retains a compact identity and complete tools. Same-origin theme following reads only the publicly applied parent theme attribute and never implies plugin authentication. The fixed reference and license are recorded in plugin/NOTICE.md; DESIGN.md documents the implemented system.

## Evidence on Hand

- [AGENTS.md](AGENTS.md), the override at the beginning of [PROMPT_CN.md](PROMPT_CN.md), and [docs/RESEARCH_CN.md](docs/RESEARCH_CN.md) establish current invariants and source boundaries. Historical retest/recovery requirements below those overrides are superseded.
- [README_CN.md](README_CN.md), [docs/DESIGN_CN.md](docs/DESIGN_CN.md), [docs/API_CN.md](docs/API_CN.md), and [docs/INSTALL_CN.md](docs/INSTALL_CN.md) describe current capabilities, semantics, deployment requirements, and limits.
- [plugin/ui/index.html](plugin/ui/index.html), its sibling JavaScript/CSS files, and [docs/UI_CN.md](docs/UI_CN.md) contain the existing three-workspace interface. Desktop light/dark and mobile captures are in `artifacts/verification/`; they use mock data and are not customer or production evidence.
- [docs/TEST_REPORT_CN.md](docs/TEST_REPORT_CN.md) records module, native lifecycle, fake-API UI, and native-host browser verification. These are existing reports, not tests rerun by init. [docs/DEMO_CN.md](docs/DEMO_CN.md) documents the offline demonstration; `python3 scripts/demo.py start` reports its local page URL, and `python3 scripts/demo.py stop` stops it.
- [plugin/NOTICE.md](plugin/NOTICE.md) and [docs/SOURCE_AUDIT_CN.md](docs/SOURCE_AUDIT_CN.md) record reference sources and licensing. This context does not establish customer testimonials, commercial pricing, public adoption, or generalized model benchmark claims.

## Product Principles

1. **Evidence before action.** Preserve comparability, provenance, coverage, and uncertainty; infrastructure failure is not answer-quality failure.
2. **Make the scope explicit.** Model evidence, credential-wide restrictions, and rule scheduling are different concepts with different consequences.
3. **Keep authority with the administrator.** Observation is the default, automation is opt-in, and release plus rule resumption are separate explicit actions.
4. **Preserve ownership across lifecycle events.** Restart, unload, identity changes, and stale tasks must not erase restrictions or reuse invalid evidence.
5. **Make execution intentional.** Viewing or saving does not spend model budget; retries, cancellation, unknown cost, and unsupported capabilities remain visible and bounded.

## Accessibility & Inclusion

The current web interface declares `zh-CN`, supports narrow screens and reduced-motion preferences, and uses native form controls with accessible labels and state announcements. Preserve those existing capabilities. No formal accessibility conformance target or additional user-specific accommodation was established during init; do not claim certification.

## Planned Work and Open Decisions

OpenAI account management is implemented as an independent workspace. [docs/OPENAI_ACCOUNTS_CN.md](docs/OPENAI_ACCOUNTS_CN.md) records current behavior; older planning documents are historical. Successful authorization does not imply quality approval, release a quality pause, resume a rule, clear cooldown, or trigger model execution.

Broader team roles, public-distribution onboarding requirements, and a formal accessibility standard remain unconfirmed. Do not introduce them as established product requirements.
