# Product

> The [CPA 8.0.76 panel on port 8317](http://127.0.0.1:8317/management.html#/plugin-pages/model-quality/0) runs UI 12.8.0, plugin 0.18.11 and Go+Sobek 2.1.3 as of 2026-11-18. This release removes the reset email row, distinguishes option hover and selected surfaces, and separates the bordered execution flow from the buttons. The connection dialog shows the loaded UI version. See the [dialog and UI checks](docs/UI_REWORK_12_8_0_CN.md) and [deployment record](artifacts/ui-rework-12.8.0/deployment.json). Port 18417 is a synthetic preview.

## Platform

web

## Users

The primary user is a single self-hosting administrator who already knows CLIProxyAPI (CPA), evaluates model output, and manages the quality of their own credentials. The user confirmed this audience on 2026-11-14. Team administration and onboarding for a broad public audience are not confirmed primary use cases.

## Product Purpose

Model Quality is a standalone CPA native plugin with an embedded web quality console. It helps the administrator collect comparable model-output evidence, distinguish incorrect answers from execution problems, and manage account authorization through explicit actions. The official host supports quality observation; independent quality pause, release and automatic pause are unavailable.

Success means the administrator can understand what was tested, whether an answer is correct, which credential is affected, and which restrictions remain in CPA. Manual evaluation itself never pauses or releases credentials automatically.

## Positioning

Model-level results provide evidence. Historical quality-pause ownership remains durable and separate from operator disable, native cooldown, and restrictions held by other owners; the official host cannot create or release it through this plugin. The plugin uses the host's normal model execution pipeline with a fixed credential instead of creating a separate provider client.

A paused credential cannot be tested again, including as a control or judge. Historical ownership must not be silently removed or emulated with operator disable. Dependent rules stay stopped until an explicitly supported lifecycle allows otherwise.

## Operating Context

The administrator runs a standalone official CPA host with its native plugin interface and opens the plugin's browser interface. The existing implementation uses plain HTML, CSS, and JavaScript ES modules embedded in a separate Go module and native C ABI library. It has no frontend framework, external font service, runtime CDN, or analytics dependency.

The three independent workspaces appear in this order. Account management is the default for an empty or invalid hash; explicit `#manual` and `#operations` links remain valid.

- **Account management** (账号管理): import locally persisted email/password/TOTP accounts, use a four-column account panel, explicitly authorize and bind CPA credentials, and inspect locally recorded CPA authentication state. A newly requested account-row 2FA login authorizes successful upload and enablement in one action, subject to the configured plan exclusions and existing identity/policy guards. Historical candidate review stays explicit and is never automatically replayed. Global inspection, global automatic reauthorization and per-account consent enable bounded repair only for verified bindings with explicit authentication failures; disabled or paused credentials are not reactivated. Inspection defaults to 15 minutes (5–1440 configurable), with automation off. The local views are accounts, login tasks, and login settings (登录设置); the latter contains proxy, inspection and upload-exclusion controls. Reset-2FA and logout-all remain explicit manual security actions with visible consequences.
- **IQ question testing** (current UI label: 智商测试): select a credential, choose the versioned candy logic or pelican HTML question (or a custom question), edit the prompt if desired, select one model and reasoning effort, request 1–8 independent answers, and read the responses alongside or below the same form. Manual runs do not score automatically or invoke a control/judge credential. Editing built-in text submits a custom question without inheriting its answer. Copy, isolated HTML preview, cancellation, eligible rerun, redacted download and history remain available. Legacy questions, presets and multi-model histories remain readable through their existing APIs; the current manual surface does not expose a general evaluation-configuration workflow.
- **Quality observation** (质量观察): configure observation rules, explicitly run or schedule detection, inspect credential restrictions and evidence, pause or resume detection rules, and review audit history and budgets. Credential state uses three columns: credential, availability, and restriction details. Independent quality pause/release controls remain unavailable.

Plugin 0.17.1 / Go+Sobek 2.1.2 aligns workspace selection with the specified reference in both web login after TOTP and OAuth. An explicit workspace ID must match exactly. Without a pin, choose the first valid organization workspace, then the first valid workspace if there is no organization. Missing pinned identities and lists without a valid ID stop safely. Bound reauthorization validates the current credential before passing its workspace; security reauthorization retains the verified security workspace and checks the resulting candidate. Historical tasks stay unchanged and require an explicit new login to use the update. See [workspace selection](docs/WORKSPACE_SELECTION_CN.md) for verified deployment and offline coverage.

Tasks run in the background independently of the browser. Closing the page does not cancel a task. An explicit cancellation uses the host operation lifecycle. Interrupted tasks are retained after restart without automatically replaying potentially billable model requests.

The administrator connects using the existing management key. After successful authentication, the plugin stores that key as plaintext in its own endpoint-scoped localStorage entry and restores the connection on reload, reopen, or browser back navigation. Explicit disconnect and HTTP 401 clear the saved key; transient connection failures retain it. Storage restrictions fall back to page memory with a visible explanation. The plugin never reads the parent CPA authentication store, and account passwords, TOTP secrets and OAuth callbacks remain outside browser storage. The public `/v0/resource/plugins/model-quality/index.html` entry serves static content; data and actions use authenticated same-origin `/v0/management/plugins/model-quality/` APIs. Opening the page, connecting, or saving configuration does not call a model.

## Capabilities and Constraints

### Credential and rule semantics

- Default to observation. Scheduling requires explicit global and rule configuration. Independent quality pause, release and automatic pause are rejected on the official host; operator disable cannot emulate them.
- Quality-paused credentials cannot be tested, including as a control or judge. Historical ownership and dependent stopped rules remain intact; no bypass, automatic recovery or retest switch exists.
- Rule state, operator disable, quality pause, native cooldown and effective availability remain separate facts. Pausing a detection rule does not release any credential restriction.
- The release has no groups or per-model isolation. Model-level evidence does not grant credential-wide action authority.

### Authorization plans and CPA upload exclusions

Account, Job and Candidate expose canonical `plan_type`: `free`, `go`, `plus`, `pro`, `prolite`, `business_premium`, `team`, `enterprise`, `edu` or `unknown`. This is metadata from an authorization snapshot or a verified local CPA binding, never a live subscription check. Missing or unrecognized metadata stays `unknown`; it must not be inferred as Free.

`cpa_upload_excluded_plans` belongs to account settings and defaults to an empty list, so upgrading does not opt any account into exclusion. Unlike the reference project's automatic-repair-only exclusion, the user's policy applies to every plugin CPA credential upload or update: account-row login, explicit candidate confirmation, scheduled reauthorization and authorization after manual security actions. Known plans in the local account snapshot, new candidate and verified current binding each veto a write when excluded. The `unknown` exclusion applies only if no source supplies a known plan. The latest setting is checked before the write; the official callback still offers no cross-process atomic compare-and-swap guarantee.

An excluded candidate remains available for explicit review within its current session and expiry; changing settings alone does not upload it. The user may explicitly confirm while it remains valid, or discard it. Restart and expired sessions do not replay uploads. This policy does not undo a security action: an already confirmed new 2FA seed remains saved, and uncertain security mutations remain quarantined. Existing credentials and their enabled state are not rewritten merely because the exclusions changed.

### Evidence and interpretation

- Keep execution status, quality verdict, and action status separate. A successful HTTP response does not establish a correct answer.
- Only complete, comparable, explicitly incorrect evidence advances a quality-failure observation. A model round requires all planned samples to be complete, comparable, and decidable; at least one incorrect sample then constitutes failure. Observation does not authorize a credential pause on the official host.
- Missing samples, unknown outcomes, mixed model outcomes, or changed conditions break consecutive failure evidence. Network errors, authentication failures, rate limits, server errors, refusal, empty or truncated output, cancellation, and judge failures do not count as incorrect answers.
- “All models failed” means all selected detection models, not every model the credential can serve. Random-question automatic decisions additionally require a valid passing control under the same comparable conditions.
- Show quality pass rate alongside coverage. Unknown timing, reasoning capability, or usage remains unknown; missing usage is not zero cost. `response_model` is an upstream label, not proof of model identity. There is no calibrated general IQ score.

### Deployment, safety, and scope

- The current plugin installs into the unchanged official CPA host. The current 8.0.76 package is verified on Linux arm64; other platforms need native build and verification. Patched-host and historical macOS packages are not this release.
- The official public SDK does not expose the independent quality-pause lifecycle or trusted final evidence required for automatic actions. This release remains observation-only. Home and remote credential storage are outside the verified local standalone save path.
- Preserve fixed credential selection, host model mapping, thinking configuration, and the final payload rules. Conditions that cannot be proven comparable cannot support automatic pause.
- Keep calls, samples, output, concurrency, queues, storage, and known usage bounded. Development and demonstrations use offline fake upstreams and isolated fake credentials, never real model accounts or copied production credentials.
- Model-generated HTML/SVG is untrusted. Show escaped source and an isolated preview without scripts, external network access, forms, navigation, or access to the parent page.
- Never expose management keys or raw credentials through URLs or logs. The administrator explicitly requested persistence of the plugin management key in its scoped browser storage entry; account passwords, TOTP secrets and OAuth callbacks remain outside browser storage. At the administrator’s explicit request, OpenAI management views show full emails and identity IDs; opening account details displays saved passwords/TOTP in plaintext controls. Saved login materials and login proxy settings persist as plaintext in the host’s local database; no encryption initialization or storage key is required. Management authentication, restricted file permissions and transient secret handling remain intact. UI 11.18 replaces the legacy login-task list export with the same selected-task diagnostic report used for troubleshooting. Both task and account reports use an explicit field/type allowlist that excludes email, credential identity, passwords, TOTP seeds, tokens, proxy addresses, URLs and raw response bodies. Production prompts and answers are not stored by default.
- Account groups, automatic credential deletion, external notifications, BPS, Jev, ModelTrace, and Codex ticket probes are outside the current core scope.

## Brand Commitments

The existing product name is **Model Quality**, plugin ID `model-quality`, with the Chinese descriptor 模型质量控制台. The interface currently uses Simplified Chinese and the three workspace labels recorded above. Preserve the precise distinctions between 暂停检测规则, 质量暂停, 人工禁用, 有效可用状态, and 人工解除.

The interface follows the CPA Management Center plugin-management page and shared component language. The 2026-11-14 host-alignment instruction supersedes the previous color-only inheritance: warm light, white and warm dark themes, buttons, fields, cards, task navigation, switches, tables and dialogs share clear host references. UI 11.18.0 retains the actual local CPA foundation palette captured during UI 11.0.0 in `artifacts/ui-rework-11.0.0/host-styles.css`: warm light, white and warm dark, with `#8b8680` actions. The page heading precedes neutral primary navigation. Account tabs live inside the account surface with tab/tabpanel semantics and Left/Right/Home/End keyboard selection. Tests split settings and answers at a 960px content width; the account toolbar combines segmented status filters, search, inspect-all and import in one desktop row. At 900px and below, filters occupy the first row and search/actions the second; at 600px and below, inspect uses its icon and import uses a short visible label while both retain their full accessible names. The four-column table follows directly. Account identity uses two visible lines: email, then a compact plan badge with an optional real label or enabled-automation note; the default manual-login note is omitted. Recent operation uses status/time followed by a single-line operation summary; long text truncates visually while full values remain in accessible names, tooltips and the existing profile/log/task views. CPA status also uses two lines; its tooltip preserves full binding and inspection times. The redundant proxy shortcut is removed; proxy settings remain in Login settings. The Actions cell follows the specified reference with two rows of three tracks in a 1.12:1.12:0.8 ratio. Login, reset-2FA and delete appear above account profile, logs and more. The plugin saves and enables CPA credentials after eligible successful login, so profile occupies the reference upload position without adding a separate upload step. Buttons are 29px high at every breakpoint, with 10.5px type, 14px icons, 6px gaps and 8px corners. Delete uses the readable CPA danger color; more includes text and a down arrow. The menu contains task, inspection and logout-all, without duplicate reset or delete entries. Dynamic guards preserve pending-state, capability, credential and proxy restrictions; reset and delete open existing confirmation dialogs and restore focus to their row entry. UI 11.14 removes the service-status disclosure from Login settings; UI 11.18 displays task logs directly, with optional expanded diagnostic fields. No accessibility certification is implied. The CPA iframe is the primary setting and relies on the host shell; direct access retains a compact identity and complete tools. Same-origin theme following reads only the publicly applied parent theme attribute and never implies plugin authentication. The fixed reference and license are recorded in plugin/NOTICE.md; DESIGN.md documents the implemented system.

## Account plans and status — UI 11.16

The account list retains CPA's compact AuthFileQuota structure and SiwcStatusBadge proportions. UI 11.16 follows the user's latest preference for toSub2's PlanBadge colors: Pro silver; Pro Lite and Business Premium gold; Plus blue, Go green, Team purple, Enterprise blue-grey and Edu rose. Gold has explicit dark-theme colors. Plan pills stay 22px high, with a quiet “套餐” label and an 11px monospace value; unknown remains explicit with a dashed outline. Upload-exclusion options retain 30px native-checkbox pills and matching colors; a checked border preserves the tier background. Canonical plan values, authorization-snapshot meaning and upload policy do not change.

CPA health and latest-operation states use 22px-high dot badges. Observed healthy states and successful operations use green, failures red, pending actions amber and unknown states neutral. Identity, CPA status and latest operation each occupy a 48px two-line block. Only `awaiting_inspection` with `unknown` or `not_checked` is presented as “待巡检”; this never asserts health. The timestamp sits beside its operation status; at widths up to 600px the date is hidden, and up to 360px the inline time is hidden, with complete date/time retained in the title and task details. The removed service-status disclosure and manual-test footer stay absent.

## Login tasks and detailed diagnostics — UI 11.18

Login tasks use a 288px desktop history sidebar with account/task/code search and all, active and attention filters. A selected task presents its actual result, identity, recorded span and compact horizontal security steps. Logs remain visible below the result rather than hidden behind a generic disclosure. Narrow layouts stack the history above the detail; existing CPA themes and toSub2 plan colors remain.

Task and account logs share readable event rows, millisecond times, adjacent-event intervals, abnormal filtering, Chinese/code/request search, expanded fixed fields and escaped JSON. New request events record a fixed endpoint label, HTTP method/status, request duration and transport failure. HTTP errors describe that request, not an inferred final task result. Timeouts retain the last known stage; the mapped runtime source code, OAuth response/transport diagnosis and confirmed security facts survive into the task record. Unknown mutations and writes remain quarantined and never become a retry instruction.

Copy and JSON export include all loaded events for the selected task regardless of display filters. Reports state the job and log revision; a mismatch or incomplete read is marked rather than presented as a synchronized complete record. Logs retain at most 200 events per job; request detail stops after 128 requests with an explicit marker while stage and terminal events continue. Terminal-job retention remains 30 days. Existing history is not rewritten, and older tasks cannot gain request details that were never recorded. Request diagnostics cover the protocol-facing login/security calls, not raw Sobek challenge internals. See [UI 11.18.0](docs/UI_REWORK_11_18_0_CN.md) for validation and deployment status.

## Account troubleshooting logs — UI 11.17

UI 11.17 removes the account row's default “手动登录” note. A real account label, enabled automation policy or unreadable-material warning can still accompany the plan badge. This is presentation only and does not change account login permissions.

Inline logs focus on one selected task. “最近一次” selects the account's latest operation; “历史任务” lists the account's recent retained jobs with an explicit 200-task limit when applicable. The overview identifies the operation, current result, method, trigger, full task ID, start time, last recorded stage and recorded span. Current security facts remain separate from event-time facts. Millisecond timestamps and adjacent-event intervals help locate delays without claiming complete task or phase durations. Search matches readable summaries and fixed diagnostic codes, and combines with the abnormal filter. Expanding or manually reading events pauses follow mode; failed reads retain loaded content.

Copy summary and JSON export include all loaded events for the selected task independently of display filters. Reports carry UI/runtime versions, time zone, task ID, fixed diagnostic fields and explicit loaded/legacy/truncated/incomplete state. A field/type allowlist excludes email, passwords, TOTP seeds, proxy addresses, authorization URLs, callbacks and tokens. The backend retains at most 200 events per job and exposes retained-window and page metadata without rewriting old records. New plan-exclusion events are warnings, while old error-code events remain findable in the UI. Unknown writes and security outcomes take precedence over network-retry advice. Account execution and runtime behavior do not change.

## Manual-test settings — UI 11.13

The test form ends after the question content and applicable field feedback. The footer showing the expected answer count and the automatic-scoring/pause disclaimer is removed, together with its spacing and unused update logic/styles. The answer-count input still sets the requested sample count. The header submit button retains disabled-state and eligibility checks, explanatory title and field errors; removing the summary does not change model execution or quality behavior.

## Evidence on Hand

- [UI 11.18.0](docs/UI_REWORK_11_18_0_CN.md) records account-first navigation, the task redesign and request diagnostics. Seventeen JavaScript tests, backend tests/vet, offline browser checks, native packaging and deployment/restart passed. Protected snapshots and existing log events remain unchanged.

- [UI 11.17.0](docs/UI_REWORK_11_17_0_CN.md) records the account-note removal and troubleshooting-log redesign. Backend and UI tests, themes/narrow layouts, native build and deployment/restart passed. Protected snapshots and existing log events are preserved.

- [UI 11.16.0](docs/UI_REWORK_11_16_0_CN.md) restores the requested toSub2 plan palette while retaining UI 11.15 layout and state semantics. Light/dark palette measurements, native build and deployment/restart passed with protected state preserved.

- [UI 11.15.0](docs/UI_REWORK_11_15_0_CN.md) aligns plan tier colors and compact state badges with the actual CPA frontend. Desktop badges measure22px with48px aligned blocks;390/320px checks preserve key state labels and plan names. Three themes, native build and deployment/restart passed with protected state preserved.

- [UI 11.14.0](docs/UI_REWORK_11_14_0_CN.md) removes the settings service-status disclosure and restyles plan badges/options using the reference PlanBadge and CPA AuthFileQuota. Desktop and narrow layouts retain 20px badges, 45px identity blocks and ten unclipped 30px options. Light/white/dark and keyboard Space checks passed, with no settings save or real account/model operation. Native tests/vet/build and deployment/restart preserve exact protected data and settings.

- [UI 11.13.0](docs/UI_REWORK_11_13_0_CN.md) removes the entire manual-test footer summary and its spacing. Offline checks show no footer, a valid three-answer configuration enabled, and an operator-disabled credential blocked with both button title and field reason retained. Native tests/vet/build and deployment/restart passed, preserving the existing one account, two historical jobs, one CPA credential and exact settings snapshots. No real model or account operation was performed.

- [UI 11.12.0](docs/UI_REWORK_11_12_0_CN.md) removes visible required/execute/skip labels from the horizontal flow, preserving step names, arrows, visual state and accessible selected/unselected labels. Reset remains480×486px across four choices, mobile281×486px and logout480×420px; visible status labels are absent. Static/native tests/vet/build and deployment/restart passed with protected state unchanged.

- [UI 11.11.0](docs/UI_REWORK_11_11_0_CN.md) removes repeated action cards, numbering and duplicate selection labels. Equal-height checkbox rows share one group, concise descriptions stay on one line, and the horizontal preview becomes one neutral band. Twelve layout cases confirm complete single-line descriptions, equal 66px rows, aligned checkboxes and a horizontal flow. Desktop reset measures480×500px, mobile reset500px high, short-window reset480×432px and standalone logout480×434px. Keyboard/whole-row interaction, defaults, focus and stale-state guards passed; Node16/static/native tests/vet/build passed. Deployment and restart preserve exact account, job and setting snapshots, configuration and host image.

- [UI 11.10.0](docs/UI_REWORK_11_10_0_CN.md) restores a horizontal security flow beneath the option cards. Stable nodes and arrows distinguish required, selected and skipped steps before submission. Twelve layout cases passed across desktop, phones and short windows; reset has three equal-width nodes and standalone logout two. Three themes, selection/skip connector states and Escape focus return were checked. Sixteen Node tests and native tests/vet/build passed, as did deployment/restart with exact account/job/settings preservation. No real or synthetic security action was submitted in this presentation-only verification.

- [UI 11.9.0](docs/UI_REWORK_11_9_0_CN.md) combines required and optional security steps into one ordered flow with whole-row labels, native checkboxes, explicit selected/unselected states and a footer operation count. Desktop/phone/short-window measurements, both action types, whole-row clicks, Space, focus return, defaults and three themes passed synthetic checks. Native tests/vet/build and 16 Node regressions passed. Four reset and two logout combinations produced exactly six correct synthetic requests. Stale-account feedback preserves page position and permits cancel while disabling choices/submit. Deployment/restart preserves exact account, job and setting snapshots.

- [UI 11.8.0](docs/UI_REWORK_11_8_0_CN.md) defines stable security-confirmation dialog sizing, fixed step rows and execution/skip states. Synthetic checks cover four reset-option combinations, both logout-option states, desktop/phone/short-window sizing, all three themes, cancel/Escape focus return and default selections on reopen. The stale-account error scrolls only the modal body and blocks submission while leaving cancel available. Two synthetic submissions preserve exact option payloads; 16 Node regressions and native build/tests/vet passed. Deployment and restart preserve the full account/job/settings snapshot, and the actual CPA dialog repeats the stable-size result without a real security submission.

- [Workspace selection 0.17.1](docs/WORKSPACE_SELECTION_CN.md) describes reference-aligned organization preference, exact bound identity, security reauthorization and historical-task behavior. Fourteen Web/OAuth scenarios, runtime race/vet, plugin login race, version capability compatibility and native build checks passed. Deployment preserves the exact account/job/settings snapshot; no real login was attempted.

- [UI 11.7.0](docs/UI_REWORK_11_7_0_CN.md) records the new plan fields, broad upload-exclusion scope and compact row design. Browser measurements cover 1064/840/390/320px widths and light/white/dark themes, with fixed two-line summaries and 29px row actions. Synthetic settings/candidate interactions and 16 Node regressions passed, as did backend plan/race checks, native module tests/vet and Linux arm64 packaging. Deployment and restart preserve the existing account/job and protected files; only explicit unknown-plan and empty-exclusion defaults were added to the API snapshot.

- [UI 11.6.0](docs/UI_REWORK_11_6_0_CN.md) records the HTML/CSS toolbar update, unchanged JavaScript and element IDs, syntax checks, native tests/vet, measured desktop/mobile layout, filter/search/import interactions, host baselines and deployment/restart evidence. The Node client/import suite was not repeated for this presentation-only change; its 16-test result remains in UI 11.5.

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

## Security confirmation dialogs

Reset always changes the authenticator for the account selected from its row. The new seed must be confirmed and saved locally before optional logout or CPA reauthorization. Logout defaults off; reauthorization defaults on. The reset confirmation presents a plain consequence note and two compact, individually bordered optional switches in one column. The email row is omitted at the user’s request; the selected account ID and revision still govern submission. The user explicitly chose to keep the current two functions; authorization and CPA upload remain one action. There is no inner key-comparison card. It inherits the plugin palette and shows the selected count. Its visible footer flow uses numbered, connected nodes and names only actions in the selected plan, in execution order; if both options are off it explicitly says reset only. Standalone logout never changes 2FA and offers reauthorization only. Opening either dialog, changing the options, pressing Escape or cancelling submits no task. Expandable notes explain local seed preservation, plan exclusions and uncertain results. Native checkboxes, cancel-first focus, inline errors and the original submission guards remain in place. See [UI 12.8.0](docs/UI_REWORK_12_8_0_CN.md) and the [earlier confirmation design](docs/UI_REWORK_11_12_0_CN.md).

## Accessibility & Inclusion

The current web interface declares `zh-CN`, supports narrow screens and reduced-motion preferences, and uses native form controls with accessible labels and state announcements. Preserve those existing capabilities. No formal accessibility conformance target or additional user-specific accommodation was established during init; do not claim certification.

## Planned Work and Open Decisions

OpenAI account management is implemented as an independent workspace. [docs/OPENAI_ACCOUNTS_CN.md](docs/OPENAI_ACCOUNTS_CN.md) records current behavior; older planning documents are historical. Successful authorization does not imply quality approval, release a quality pause, resume a rule, clear cooldown, or trigger model execution.

Broader team roles, public-distribution onboarding requirements, and a formal accessibility standard remain unconfirmed. Do not introduce them as established product requirements.
