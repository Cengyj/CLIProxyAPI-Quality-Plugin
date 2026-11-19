---
name: "Model Quality"
description: "UI 11.18 visual baseline restored by UI 12.1; see the linked revision for contrast and small-screen changes."
appliesTo: "11.18.0"
supersededBy: "docs/UI_REWORK_12_1_1_CN.md"
colors:
  page: "#faf9f5"
  card: "#f0eee8"
  input: "#faf9f5"
  floating: "#fffdf9"
  hover: "#e9e6df"
  ink: "#2d2a26"
  muted: "#6d6760"
  border: "#e3e1db"
  control-border: "#d5d2cb"
  accent: "#8b8680"
  accent-hover: "#7f7a74"
  on-accent: "#ffffff"
  focus: "#8b8680"
  focus-ring: "#8b86802e"
  good: "#065f46"
  good-bg: "#d1fae5"
  warn: "#92400e"
  warn-bg: "#d977061a"
  bad: "#8a3a30"
  bad-bg: "#c6574624"
  danger: "#c65746"
  on-danger: "#ffffff"
  white-page: "#ffffff"
  white-card: "#ffffff"
  white-input: "#ffffff"
  white-floating: "#ffffff"
  white-hover: "#f6f6f6"
  white-border: "#e5e5e5"
  dark-page: "#151412"
  dark-card: "#1d1b18"
  dark-input: "#151412"
  dark-floating: "#2a2723"
  dark-hover: "#2e2a26"
  dark-ink: "#f6f4f1"
  dark-muted: "#c9c3bb"
  dark-border: "#3a3530"
  dark-control-border: "#4a453f"
  dark-accent: "#8b8680"
  dark-accent-hover: "#9a948e"
  dark-on-accent: "#ffffff"
  dark-focus: "#a6a099"
  dark-focus-ring: "#8b86802e"
  dark-good: "#6ee7b7"
  dark-good-bg: "#064e3b4d"
  dark-warn: "#fcd34d"
  dark-warn-bg: "#f59e0b24"
  dark-bad: "#f1b0a6"
  dark-bad-bg: "#c657463d"
  dark-danger: "#c65746"
  dark-on-danger: "#ffffff"
typography:
  brand:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, \"Helvetica Neue\", Arial, \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "17px"
    fontWeight: 700
    lineHeight: 1.5
  headline:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, \"Helvetica Neue\", Arial, \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "26px"
    fontWeight: 700
    lineHeight: 1.3
  headline-mobile:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, \"Helvetica Neue\", Arial, \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "24px"
    fontWeight: 700
    lineHeight: 1.3
  title:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, \"Helvetica Neue\", Arial, \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "18px"
    fontWeight: 700
    lineHeight: 1.5
  section:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, \"Helvetica Neue\", Arial, \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "16px"
    fontWeight: 600
    lineHeight: 1.5
  body:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, \"Helvetica Neue\", Arial, \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.5
  label:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, \"Helvetica Neue\", Arial, \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "14px"
    fontWeight: 550
    lineHeight: 1.5
  supporting:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, \"Helvetica Neue\", Arial, \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.7
  metadata:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, \"Helvetica Neue\", Arial, \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "12px"
    fontWeight: 400
    lineHeight: 1.65
  status:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, \"Helvetica Neue\", Arial, \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "13px"
    fontWeight: 600
    lineHeight: 1.5
  answer:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, \"Helvetica Neue\", Arial, \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "16px"
    fontWeight: 400
    lineHeight: 1.8
  answer-code:
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace"
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.7
  code:
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace"
    fontSize: "12px"
    fontWeight: 400
    lineHeight: 1.5
  metric:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, \"Helvetica Neue\", Arial, \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "24px"
    fontWeight: 650
    lineHeight: 1.3
rounded:
  tag: "6px"
  control: "8px"
  card: "12px"
  switch: "24px"
spacing:
  tight: "4px"
  compact: "8px"
  panel-gap: "12px"
  field: "16px"
  section: "24px"
  wide: "32px"
  workspace: "48px"
components:
  button-primary:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.on-accent}"
    rounded: "{rounded.control}"
    padding: "10px 14px"
  button-primary-hover:
    backgroundColor: "{colors.accent-hover}"
  button-secondary:
    backgroundColor: "{colors.input}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "10px 14px"
  button-ghost:
    textColor: "{colors.muted}"
    rounded: "{rounded.control}"
    padding: "10px 14px"
  button-danger:
    backgroundColor: "{colors.danger}"
    textColor: "{colors.on-danger}"
    rounded: "{rounded.control}"
    padding: "10px 14px"
  input:
    backgroundColor: "{colors.input}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "10px 12px"
  navigation:
    textColor: "{colors.muted}"
    padding: "8px 12px 11px"
  switch:
    backgroundColor: "{colors.input}"
    rounded: "{rounded.switch}"
    width: "44px"
    height: "24px"
  status-tag:
    backgroundColor: "{colors.hover}"
    textColor: "{colors.muted}"
    rounded: "{rounded.tag}"
    padding: "2px 8px"
    typography: "{typography.status}"
  card:
    backgroundColor: "{colors.card}"
    textColor: "{colors.ink}"
    rounded: "{rounded.card}"
    padding: "16px"
  dialog:
    backgroundColor: "{colors.floating}"
    textColor: "{colors.ink}"
    rounded: "{rounded.card}"
    padding: "24px"
---

# Design System: Model Quality

> **Current design:** [UI 12.8.0 reset confirmation](docs/UI_REWORK_12_8_0_CN.md) builds on [12.1.1's workspaces](docs/UI_REWORK_12_1_1_CN.md). The reset dialog separates account identity, authenticator impact, optional actions and the exact pending plan. The rest of the interface keeps its three themes, horizontal navigation, compact account rows, responsive answer comparisons and shared metrics band.

> UI 11.18.0 / plugin 0.17.3 / Go+Sobek 2.1.3 puts Account management first and redesigns Login tasks around compact results and visible diagnostic logs. Offline checks, native packaging and 8317 deployment/restart verification passed with protected state preserved. The unchanged CPA foundation and toSub2 plan colors remain. [UI 11.18.0](docs/UI_REWORK_11_18_0_CN.md) tracks evidence; prior reports remain historical.

## Overview

**Creative North Star: "The CPA Quality Workspace"**

Model Quality follows the CPA plugin-management page and its shared UI components. Warm light, white and warm dark themes retain the host's surface hierarchy, system typography and restrained controls. Consistency makes the administrator's current task, evidence and next action easy to find.

The plugin supplies task navigation inside the existing CPA shell and keeps direct access complete. Buttons, fields, cards, primary and local navigation, switches, tables and focused dialogs share host contracts; adjustments serve readable contrast, mobile input and the plugin's precise state semantics. The interface uses native HTML, CSS and ES modules with Go embed. No raster artwork, remote fonts or runtime component library ships.

**Key Characteristics:**

- Separate page, card, input, floating and hover roles in all three themes.
- Host system type, eight-pixel controls and twelve-pixel working surfaces; workspace headings identify the task and purpose.
- From 960px content width, a 320px settings column sits beside answers; narrower layouts stack. Local answer/history switching and expanded reading preserve the mounted draft.
- Neutral grouped primary navigation, underlined local tabs, consistent table density and native control behavior.
- Independent labels for execution, quality, rule state and credential restrictions.

The current workspaces are 账号管理, 智商测试 and 质量观察, in that order. Account management opens by default for an empty or invalid hash; explicit manual-testing and observation links continue to work. Account filters, search, inspect-all and import share a single desktop toolbar; the redundant proxy shortcut is removed. The account Actions cell shows login/reset-2FA/delete above account profile/logs/more, following the specified reference. More contains task, inspection and logout-all. Reset and delete remain explicit actions with confirmation. The observation table presents credential, availability and restrictions. Main testing forms omit reference-version labels; unchanged built-in questions have no caption and edited prompts show “已修改”. Frozen conditions, raw evidence and identity/version metadata remain available in details and exports. Product semantics remain in `PRODUCT.md`. Current source evidence belongs in `artifacts/ui-rework-11.18.0/`; older versioned records below are historical.

## Colors

The frontmatter foundation color roles match UI 11.16.0; plan-specific colors are described below. Unprefixed tokens are light; `white-` and `dark-` override the matching role. UI 11.0.0 extracted the actual running CPA stylesheet into `artifacts/ui-rework-11.0.0/host-styles.css` and mapped its page, card, input, hover, border, text and action roles directly. This supersedes the custom blue palette. The earlier upstream component reference and license inventory remain in `plugin/NOTICE.md`.

### Primary

- **Warm Stone Action** (`accent`, `accent-hover`, `on-accent`) distinguishes the explicit submit. All three themes use the host action gray `#8b8680` with white text; dark hover is `#9a948e`.
- **Neutral Focus** (`focus`, `focus-ring`) marks the keyboard and text-editing location. Field focus uses a border plus a soft ring; other keyboard targets keep an offset outline.
- **Consequential Action** (`danger`, `on-danger`) fills only the final consequential confirmation. Other consequential actions use outlined error text.

### Semantic states

Deep green, amber and clay pairs identify named positive, attention and error states. Complete execution and comparability remain neutral. Missing usage stays unknown and recorded zero remains zero. Manual release feedback does not imply full credential availability.

**The Separate State Rule.** Execution, answer quality, rule enablement and credential restrictions are different facts. Pair color with precise text and never derive one state from another.

### Neutral

Page, card, input, floating and hover are separate semantic roles. Light uses page/input `#faf9f5`, card `#f0eee8`, and floating `#fffdf9`; white uses white surfaces; dark uses page/input `#151412`, card `#1d1b18`, and floating `#2a2723`. Borders and text also come from the running host palette. Cards have a fine border and a subtle light-theme shadow.

Earlier graphite-action substitutions and contrast reports describe historical releases. UI 11.16.0 retains the actual CPA foundation colors and does not claim a new contrast audit or accessibility certification.

When embedded in a recognizable same-origin CPA page, the plugin follows only the parent's public applied root theme: missing attribute means light, `white` means white and `dark` means dark. A MutationObserver tracks changes. The host's auto mode applies white in system light, so white must not collapse into light. The local selector is hidden and disabled while following. Unrecognized parent, cross-origin access, unsupported theme or observer failure falls back to a local in-memory selector; no parent key, store or browser storage is read.

## Typography

The UI uses the host's system-font approach with explicit Chinese fallback. Titles are strong, body copy is compact and ordinary model prose is larger for reading. Code, HTML, SVG and raw diagnostics use the local monospace stack. No remote font loads.

The frontmatter records typography roles; current component sizes are authoritative in `plugin/ui/style.css`. Visible workspace headings identify each task. Explanatory paragraphs stay concise. Long identifiers and original source wrap; timing, usage, dates and comparable values use tabular numerals.

Supporting metadata can be smaller than the body, but restriction, action and verdict meaning must remain in readable named text. Native input and select behavior remains intact; component sizes are defined in the current stylesheet.

## Layout

The host owns the shell, sidebar and floating tools. The plugin header starts with the active page title and description, with refresh/connection tools alongside. Primary navigation sits below the title inside the shared workspace. Section headings remain available to assistive technology without duplicating the visible title. Embedded mode reserves the top-right area for host tools. The shared workspace has a 1600px maximum width and uses continuous page scrolling. At 768px and below, the embedded header reflows below the host tools; local navigation and actions remain reachable.

Manual testing splits into a 320px settings column and a flexible answer/history surface at a content width of 960px. The submit button stays in the settings header and remains associated with `run-form`. Credential, model, reasoning, answer count and question controls share a compact vertical form. Narrower viewports stack settings above answers. “展开阅读” hides the mounted settings temporarily without clearing the draft. Reference-version strings no longer appear in the question caption; editing a built-in prompt still shows “已修改”. Frozen task conditions and raw evidence remain available on demand. UI 11.13 removes the entire footer summary after the question field, including its reserved spacing, unused updatePlan logic and selectors. The form ends with the question content and applicable field errors. The header submit button retains eligibility checks, disabled state and its explanatory title; sample count and execution behavior are unchanged.

The account surface includes a tablist for Accounts, Login tasks and Login settings (登录设置), followed by the selected panel. Tabs use tab/tabpanel associations, aria-selected, roving tabindex and Left/Right/Home/End keyboard selection. The list toolbar places segmented status filters with count badges at the left and search, inspect-all and import at the right. Search and action controls are 36px high. At 900px and below, filters occupy the first row and search/actions the second; at 600px and below, compact action labels preserve full accessible names. The redundant proxy shortcut is absent. The four-column table and pagination follow inside the same surface. Columns are Account, CPA state, Recent operation and Actions. At 800px and below, rows become labeled account groups. Each Actions cell uses two rows with 1.12:1.12:0.8 tracks: login/reset-2FA/delete, then account profile/logs/more. All six controls are 29px high on desktop and phones. Icons, thin borders and stable tracks follow the specified reference while colors follow CPA. Logs continue inline; more contains task, inspection and logout-all. Reset and delete open existing guarded dialogs. Tasks and settings keep their separate local views. UI 11.14 removes the “服务状态” disclosure from Login settings and the unused login-engine rendering/styles; task IDs and machine diagnostic codes remain in “排障信息”. Actionable failure explanations and security consequences remain visible.

Quality observation uses concise metrics and a three-column credential table: credential, availability and restriction details. Rule configuration, audit records and retained pause ownership remain available. Official-host limitations stay explicit; UI layout must never enable an unsupported pause or release. Editor sections remain grouped and preserve existing form IDs, validation and save behavior.

| Content viewport | Adaptation |
| --- | --- |
| At least 960px wide | Manual settings and answers sit side by side. |
| Below 960px wide | Manual settings precede answers in document flow. |
| At or below 900px wide | Account toolbar uses two rows: status filters, then search and actions. |
| At or below 800px wide | Account and observation tables become labeled groups. |
| At or below 768px wide | Embedded header reflows below the host tools. |
| At or below 600px wide | Account task columns stack; row action buttons retain 29px height; toolbar actions stay 36px with compact labels, while dialog actions retain 44px targets; forms and answer controls wrap. |

The actual iframe width determines these changes. Native dialogs have bounded height and internal scrolling; long values wrap. Polling preserves stable drafts, focus, disclosure state and reading positions. Current verification scope is recorded in the UI 11.18.0 report, rather than inferred from these layout rules.

## Elevation & Depth

CPA-style cards combine a fine structural edge and a small soft shadow. Floating dialogs and the mobile action menu use the deeper shadow. Ordinary groups inside cards use spacing or separators rather than extra elevated containers. Surface tones remain deliberate across all three themes. Plan badges use toSub2 tier colors: Pro has a silver gradient, Pro Lite and Business Premium have gold gradients, and ordinary tiers use a restrained tint. Other surfaces remain flat. No raster texture or entrance animation ships.

The sidecar records card/dialog shadows and motion. Shared control feedback uses 150ms; switch movement and progress use 200ms. Polling never replays entrance motion. Reduced motion disables transitions and animations. These are deliberate adaptations of the host's shared feedback language, without its larger page entrance effects.

## Shapes

Control corners are eight pixels; cards and native dialogs are twelve. Account plan and state badges use full pill corners; other small state tags retain their local shape. Switch tracks are 44×24 with an 18px thumb; the native checkbox remains the focusable control and falls back to native appearance in forced colors. Underlined tabs retain rounded top corners and a two-pixel inset selection line. Inline icons use one restrained line language; existing independently authored SVG paths remain.

## Components

**The Host Component Rule.** Use the fixed CPA plugin-management page and shared primitives as the visual authority. Adapt contrast and responsive dimensions explicitly; preserve the host role and interaction contract.

### Buttons

Primary, secondary, ghost and consequential variants share dimensions, weight, spacing and radius. The filled primary action identifies the explicit submit; secondary controls have a readable boundary; ghost actions defer to nearby content. Final deletion/release confirmation receives the filled danger pair while earlier stop/release affordances stay outlined. Desktop control type is generally 12–13px; primary state and task text remain explicitly named. Busy actions preserve their measured width, disable duplicate submission and announce current state. Disabled and active states are distinct; keyboard focus is visible.

### Account actions — retained from UI 11.5.0

The action layout follows the read-only reference `/Users/ceng/Downloads/toSub2-go-port-2026.11.17/web/src/components/accounts.jsx` (around line 470), `account-actions.css` and `docs/pure-go-runtime/ui/row-actions-polished.jpg`. Two rows share three tracks in a 1.12:1.12:0.8 ratio: login/reset-2FA/delete above profile/logs/more. Profile replaces the reference upload slot because this plugin already uploads and enables CPA credentials after eligible successful login, subject to configured plan exclusions. The column is 242px with a 218px action group; 29px buttons use 10.5px type, 14px icons, 6px gaps, 8px corners, thin borders and CPA background/hover colors. Explicit appearance:none prevents native WebKit button bezels. Delete uses the readable CPA danger text role; more has text plus a down arrow. Desktop 1064px and phone 390px measurements both give approximately 75.89/75.89/54.22px tracks, with no clipping or horizontal overflow. Dynamic state and capability checks disable unsafe reset/delete entries. Existing confirmation dialogs, pending-state protections and row-specific focus restoration remain in effect; no security operation was executed during validation.

### Account toolbar — retained from UI 11.6.0

The toolbar has three desktop tracks: a segmented status filter group, a flexible search track capped at 260px, and inspect/import actions. Controls align at 36px; filter buttons are 28px inside their 36px container. The toolbar measures 68px high at 1064px viewport width. Below 900px, the status group spans the first row. At 600px and below, four filters share equal tracks, inspect becomes an icon button and import displays “导入”; full aria-labels and titles remain. At 390px, the toolbar measures 110px high. A 320px check confirms the toolbar itself fits; it does not certify the whole page against horizontal overflow. The UI 11.6 toolbar change preserved existing IDs and JavaScript; UI 11.7 adds the plan presentation and setting described below.

### Account identity, recent operation and plans — UI 11.16.0

Identity uses two visible lines: a full-value email rendered on one ellipsized line, then a small authorization-plan badge and an optional single-line real label or enabled-automation note; UI 11.17 omits the default manual-login note. The 34px avatar aligns with that block. Recent operation also uses two lines: a named result and compact timestamp, followed by an operation/diagnostic summary that opens inline logs. Pending candidate status opens the existing task review. Full email remains available in the title and profile; the log action's accessible name and tooltip retain its full operation summary. CPA status uses a compact state tag and one secondary line; its tooltip preserves full binding, reason, previous inspection and next inspection values. Long values do not increase each row's information stack. Underlying account values and logs are not truncated.

UI 11.15 follows the actual CPA AuthFileQuota plan label/value structure and SiwcStatusBadge proportions, recorded in [CPA badge research](artifacts/ui-rework-11.15.0/CPA_BADGE_REFERENCE_CN.md). The specified Go host source downloads a separate management asset; the verified source is the static frontend served by the local 8317 host. Each plan pill is 22px high, with 8px horizontal padding, a quiet 10px “套餐” label and 11px monospace value. UI 11.16 supersedes those CPA tier colors with the requested toSub2 PlanBadge palette: Pro silver (`#f9fbfd`, `#e3eaf1`, `#abb8c8`, `#eef2f6`); Pro Lite and Business Premium gold (`#fff9e3`, `#f5df9b`, `#dfba58`), with explicit dark gold. Plus uses blue `#618cc4`, Go green `#56877a`, Team purple `#8d79b3`, Enterprise blue-grey `#67768b` and Edu rose `#b08398`; ordinary tints blend with the current theme. The geometry and state styles remain from UI 11.15. Unknown reads “套餐 未知” with a dashed border. The tooltip identifies known plans as authorization snapshots rather than live subscriptions. Canonical `plan_type` values and upload-exclusion meanings are unchanged.

CPA health and recent-operation badges share 22px height, 11px text, a 5px dot and full pill corners. Green, red, amber and neutral tones retain explicit status text. Identity, CPA state and operation occupy aligned 48px two-line blocks, with a 4px gap between 22px tracks. Only an `awaiting_inspection` reason paired with `unknown` or `not_checked` displays “待巡检”; other unknown states remain uncertain. Operation time sits beside its badge. At 600px and below, its date is hidden; at 360px and below, inline time is hidden. The complete timestamp remains in the title and details, and critical state labels retain priority.

The Login settings (登录设置) view includes “CPA 上传排除套餐” as a grouped native checkbox fieldset. UI 11.16 retains each option as a 30px-high rounded pill with the same toSub2 plan palette. Checked options reinforce the border while preserving the tier background; native checkbox, focus and disabled states remain. `cpa_upload_excluded_plans` defaults to empty. The help text covers manual login, automatic reauthorization and authorization after security actions. Known local, candidate and currently bound plans participate in the write guard; `unknown` applies only without known evidence. Selection remains a draft until Save. Saving never replays an excluded candidate. A blocked candidate receives “套餐已排除” and retains explicit review within the current session. This broader upload policy follows the user's request; the reference only applied its exclusion in automatic repair.

[UI 11.16.0](docs/UI_REWORK_11_16_0_CN.md) records passing light/dark palette checks: displayed plan pills remain22px with no label overflow, checked exclusion options preserve their tier backgrounds, and no console errors were observed. Native build and deployment/restart passed with protected state preserved. The retained layout was verified in [UI 11.15.0](docs/UI_REWORK_11_15_0_CN.md): six desktop rows have22px badges and aligned48px identity/state/operation blocks;390px state headings and plan names fit, and a320px check has no document overflow or critical text clipping. Three themes and native build/deployment/restart passed. The 20px badges and 45px identity measurements in [UI 11.14.0](docs/UI_REWORK_11_14_0_CN.md) describe the previous design. Earlier [UI 11.7.0](docs/UI_REWORK_11_7_0_CN.md) contains synthetic upload-policy interactions and responsive action measurements. Historical checks do not certify upstream authentication or real credential writes.

### Login task workspace — UI 11.18.0

The desktop task surface uses a 288px history sidebar and a flexible detail area. History has a compact count, search field and all/active/attention filters; each card separates operation/time, identity and result. Explicit maximum-content rows prevent a long history from compressing cards into overlap. At 900px and below the history stacks above the detail with a bounded list viewport; at 600px it uses a single card column. Selected cards, controls and semantic result colors use the existing CPA tokens.

The result header pairs a small semantic icon with an operation label and 20px outcome heading. Identity, method, start time and recorded span sit below it. Confirmed reset/logout/reauthorization facts appear as horizontal numbered or completed steps with subtle arrows; pending or uncertain steps never claim completion. Duplicate success banners are removed. Necessary unknown-write guidance remains above the logs.

Execution logs are always visible in a framed area with compact tools and a 330px maximum scrolling viewport. They reuse the account-log event grid and safe rendering: time/interval, severity, readable phase/result, optional request method/HTTP status/duration, then expandable labeled fields and escaped JSON. Search and abnormal filtering affect display only. Polling keeps the mounted log panel, search input, expanded events and reading position; following the latest event is explicit. Task-level diagnostic facts remain in a small disclosure below the log list.

Reports use the selected task and all loaded retained events. They exclude full account identity, secrets, URLs, proxy configuration and response bodies. Schema 2 includes job/log revisions, mismatch and incomplete-read flags, request-detail limits and retained-range metadata. HTTP status or transport failures describe individual requests; confirmed side effects are recorded only when known. The backend caps request details at 128 with an explicit marker, keeps the latest 200 events per task and retains terminal jobs for 30 days. Historical records are not backfilled. [UI 11.18.0](docs/UI_REWORK_11_18_0_CN.md) separates current checks from earlier account-log evidence.

### Account troubleshooting logs — UI 11.17.0

The default “手动登录” text is omitted from account identity metadata. Explicit labels, automation states and unreadable-material warnings remain meaningful optional content beside the existing plan pill. The account's policy data is not changed.

The inline panel starts with compact range and export tools, then a selected-task overview. History uses a native selector with date, operation, result and short ID, loading events only for that task. The full ID, task start, last recorded stage, recorded span and current confirmed security facts remain available without expanding raw JSON. The last stage means the last recorded event, not a guessed failure cause. The event grid has time/interval, severity, and stage/result columns, with a 360px maximum local viewport. A toolbar combines severity filtering and text search across Chinese summaries and raw fixed identifiers. Expanded event details pair labeled diagnostic values with escaped JSON. Existing focus, disclosure, scroll and single-flight polling behavior is preserved.

The recorded span uses the first and last loaded timestamps; adjacent-event intervals are computed against the unfiltered event sequence. Neither is advertised as phase duration or complete elapsed execution. Legacy and truncated records have separate notices. A failed task's diagnosis separates code, stage, reason and advice. Uncertain credential writes and security mutations always retain their non-replay guidance before network advice. Event titles never spread the current Job's final security flags into prior records.

Copy and JSON export use a field/type allowlist and all loaded events of the selected task, irrespective of display filters. The report includes versions, generation time/time zone, task identity, current facts, recorded span and incomplete/legacy/truncated metadata. It excludes full account identity, secrets, proxy configuration, URLs and raw runtime responses. Backend `/logs` adds `first_id`, `last_id`, `retained_count` and `truncated` for the persisted window, and `has_more`/`next_cursor` for the requested page; empty pages retain the cursor. Each job remains bounded at 200 events, and the account job-list cap is separately indicated. No historical record is rewritten. The new `cpa_plan_excluded` event is warning-level, and the UI also recognizes older events with an error code. See [UI 11.17.0](docs/UI_REWORK_11_17_0_CN.md) for that release’s implementation and validation status.

### Login workspace selection — plugin 0.17.1

This runtime correction retained UI 11.7.0 when released. New login tasks use Go+Sobek 2.1.2 to continue workspace selection in both web and OAuth phases: an explicit pin wins only on an exact ID match; otherwise the first valid organization, then first valid workspace, is selected. Empty/missing pinned targets still show the existing named interaction-required state. Bound credential identity and a security operation’s verified workspace cannot be silently replaced by the default. Existing task rows and logs preserve their historical result; the operator starts a new login to use the corrected flow. See [workspace-selection behavior and evidence](docs/WORKSPACE_SELECTION_CN.md).

### Inputs and selections

Native input, textarea and select preserve familiar editing and keyboard behavior. The manual model field is a real select populated from the selected credential's registered catalog; users do not need to type an ID. Answer count is a select with exactly one to eight choices. Visible labels precede controls, compact normal hints may share their row, and expanded errors remain adjacent and associated. Validation marks `aria-invalid` and preserves input on failure. Text fields use the input role, control border and host-style focus ring. Native selects retain their browser keyboard behavior; the plugin does not claim a custom searchable combobox.

Reasoning always offers the omitted/default setting, plus only the selected model's declared supported levels, including `max` when advertised. Host flags may add off or automatic choices. Unknown capability and capabilities with no usable named levels leave the default selected and explain the limit. Active credential/model changes reset incompatible choices; background refresh preserves a removed model or reasoning level as a disabled stale option and blocks submission until the administrator explicitly reselects. Empty catalogs disable execution and point back to CPA configuration and refresh; they never become a free-text fallback.

### Cards, tables and navigation

A card uses the host title/actions/body vocabulary, compact 16px padding and restrained border/shadow. The selected local view is the working surface; a redundant card title is omitted when the active navigation already names it. Tables share header fill, aligned padding, top-aligned cells, row separators and hover. Numeric data uses tabular figures; actions wrap inside their allocated column. Primary navigation uses a neutral grouped surface and filled active state. Local tabs use an underline, selected weight, hover and focus. Narrow navigation scrolls locally without introducing a second sidebar.

### Switches and states

The official rule editor exposes one switch for scheduled observation. Off is manual-only for new rules. Turning it on authorizes only the edited rule at its next due time; changing the draft never runs a model. The list shows the effective state, not another enable control. Historical holds and paused rules remain stopped on unrelated edits.

### Answer reading

The current manual task presents one model and up to eight independent answers, two columns when space permits and one on phones. Its model identity appears once in the run summary; answer cards emphasize sample number, execution state, original answer, timing/usage and nearby actions. Single-model manual results hide redundant model headings and comparison/filter controls. Legacy multi-model histories and rule evidence keep their applicable comparison and filtering controls. Prose/code presentation changes typography only: original model content remains escaped and copy/export preserves original text. Selection, expanded answers, review drafts and internal scroll positions survive unchanged polling results. On phones, a local sticky sample heading preserves sample identity while an expanded answer is read; the global header stays in normal flow.

### Current account action controls (UI 12.7.0)

The user-supplied toSub2 account actions are the visual reference. Keep the two aligned rows with 1.12:1.12:0.8 tracks and 6px gaps. All six buttons share a visible thin border, floating surface, 8px corners, 10.5px text and 14px icons; remove the earlier borderless secondary row. Hover and expanded logs/more use the theme hover surface. Delete retains danger text, disabled controls retain their guards, and keyboard focus remains visible. Desktop controls are 29px high; narrow layouts retain 44px touch targets. Existing login/reset/delete and profile/logs/more actions remain: the user authorized visual alignment, not new upload actions or split authorization behavior.

### Reset confirmation (UI 12.8.0)

Use one compact column for the title, a short unboxed consequence note and two optional switches. The header has a shield mark and circular close control. Per the user’s explicit request, the reset dialog omits the email row; standalone logout still displays its account identity. Do not restore the inner key-comparison card. Each optional operation follows the supplied toSub2 reference: a separate bordered row, a small icon surface, concise outcome text and a native checkbox exposed as a switch. The row is 54px high including borders with 8px vertical padding, 12px regular-weight titles, 10.5px descriptions, 28px icon surfaces and 32×18px switches; the full label is clickable. Logout defaults off and CPA authorization defaults on. Retain CPA's warm light, white and dark color roles, with stronger ink contrast for the primary action and selected switches.

The footer contains a bordered, rounded execution card followed by a full-width divider before the buttons. Its horizontal flow uses sequential numbered circles, directional connecting lines, operation names and a brief outcome per step. Render only selected operations in execution order; both options off shows one reset-only node. Update the selected count and a concise accessible status description together. Keep the flow and footer height stable across all four combinations. Expandable operation notes explain confirmed-seed preservation, upload exclusions and uncertain results. The native disclosure resets to closed on reopening. Cancel-first focus, Space toggling, Escape, focus return and submission guards remain intact. Short screens scroll the body while the flow and buttons remain visible. Standalone logout retains checkbox semantics and its separate flow. Unselected hover, selected and selected hover surfaces use 7%, 14% and 20% theme-accent mixes over the dialog surface, respectively. Disabled choices do not gain hover feedback. See [UI 12.8.0](docs/UI_REWORK_12_8_0_CN.md) for screenshots and verification.

### Historical security confirmation (UI 11.12)

Use one short main-action sentence followed by a single bordered group of equal-height optional rows. The complete row labels a native checkbox; concise titles and descriptions remain on one line. Remove the repeated required-action card, step numbering and duplicate selected/unselected text. Selection remains visible through checkbox state and CPA background feedback. Reset has two independent options; standalone logout has one.

Keep the requested horizontal flow below the options as one neutral band rather than separately boxed nodes. Only step names and arrows remain visible in the flow; required/execute/skip text is removed. Optional nodes retain selected/unselected aria-labels and visual state, with unselected nodes occupying their position and dashed adjacent connectors. Put the operation count beside the flow heading. Preserve fixed header/footer, viewport-bounded sizing, internal scrolling, error-body scrolling, keyboard behavior, focus return and existing guards. Defaults remain logout=false and reauthorize=true; backend semantics do not change. Historical measurements and deployment belong in [UI 11.11.0](docs/UI_REWORK_11_11_0_CN.md), with12 layout cases confirming single-line complete text, equal66px rows and aligned checkboxes. Desktop reset measures480×500px; phones retain500px height, short windows480×432px and logout480×434px. Keyboard, focus, defaults and stale-state guards passed; native and static checks passed. Deployment and restart passed with unchanged account/job/settings snapshots, configuration and host image. UI 11.12 reduces reset/logout heights by14px to486/420px; new evidence belongs in [UI 11.12.0](docs/UI_REWORK_11_12_0_CN.md), with four reset combinations, narrow-screen overflow, two-node logout, static/native checks and deployment/restart verified. Earlier numbered-card and boxed-flow designs remain historical in their versioned reports.

### Editors and protected dialogs

The quality rule editor (UI 12.18) uses the same single column at every width. The rule name is its editable title, with credential scope immediately below. A full-width graphite strip separates selected models from ID entry and shortcuts; its lower edge holds the shared reasoning level and sample count. Grading and scheduling are two unboxed settings rows. Reference answers expand below the grading row in place. Keep the five-field server-time Cron label intact, with one input and one scheduling switch. Estimate calls only when confirming a batch execution. The sticky footer contains cancel, save and a status only when the draft has changed. Focus validation errors in context and preserve conditional and historical fields, keyboard navigation and draft-discard protection.

Long rules and global budget settings stay inline. They use normal page flow with a sticky save footer where applicable. Closing a dirty draft requires an explicit discard decision; stale async responses cannot overwrite a newer editor. Connection and short confirmations use native modal dialogs with labeled headings, separated actions, bounded scrolling, Escape and focus restoration. Evidence and preview use wider dialogs when reading requires it. The native dialog supplies background non-interactivity; no framework modal implementation is added.

Historical release presentation keeps exact ownership, identity, version, scope and remaining restrictions together; official-host release remains unavailable. Conflict and uncertain outcomes expose refresh/reconciliation and disable unsupported repeat confirmation. Release leaves dependent rules stopped and never promises general availability. The HTML/SVG preview retains empty sandbox, inert state, strict CSP and blocked scripts/network/forms/navigation.

**The Ownership Rule.** Preserve the exact owned pause, identity, action version, consequences and remaining restrictions. Keep conflict and uncertain outcomes visible; never simulate official-host release with another action.

Copy feedback stays on its action and is also announced. Save feedback identifies its object. Connection loss, stale data and unresolved release stay visible where their recovery belongs. Selection, caret, scrollbars and focus inherit theme roles. The skip link, native controls and status live regions remain; the evidence report defines tested scope, not a blanket accessibility claim.

## Do's and Don'ts

### Do:

- Do use the role tokens and all three theme mappings for every shared component.
- Do begin with the fixed CPA plugin-management and shared-component references when extending the interface.
- Do preserve visible labels, keyboard focus, native dialogs, draft state and reduced-motion behavior.
- Do keep ordinary text readable, phone input at least 16px and main touch targets at least 44px.
- Do keep unknown values explicit and preserve original answers for copy and export.
- Do check embedded and standalone layouts with actual iframe widths and complete data states.
- Do keep configuration and primary actions in the desktop first viewport; let long content scroll locally and short or narrow windows use document flow.

### Don't:

- Don't duplicate the host sidebar, global tools or authentication state.
- Don't use the floating-surface color as an ordinary card default.
- Don't turn successful completion or manual release into an unearned quality or availability claim.
- Don't turn a long rule editor into a confirmation modal.
- Don't repaint the whole reading surface on polling or move the reader's focus.
- Don't hide overflow or shrink text to claim that unlimited content fits without scrolling.
- Don't execute model-generated content in the parent page or relax the preview sandbox.


## Historical account design records

The following versioned records explain earlier design decisions and evidence. UI 11.18.0 above and the current source supersede conflicting layout, palette, endpoint and authorization statements.

### Managed OpenAI accounts

The independent login workspace defaults to the full-width account directory. Four local views separate accounts, temporary login, login jobs and inspection settings. Global inspection, global automatic reauthorization and per-account consent remain distinct. Full account identity, CPA binding, observed status and recent activity lead each row. Unknown remote health stays unknown. First authorization and manual login retain candidate review. Secret fields clear after successful import, closing an editor, view changes and disconnect. The existing CPA theme roles, native controls, responsive row layout and bounded history rules apply.

### OpenAI workspace composition — UI 8.0.0

A concise page heading sits above four flat, underlined local tabs. The default directory uses the full workspace width with no permanent utility rail. A compact toolbar groups inspection and import actions; search and status filtering precede a four-column table. Full email and CPA identifiers wrap instead of truncating. Search matches email, remark and CPA identifier; attention/linked filters and twenty-row pagination bound longer directories. Empty lists have no inactive pager.

Import, account details and login-target selection replace the directory with a focused wide form and an explicit return. Returning preserves search/filter state and restores focus to the initiating control. The account editor groups remarks and automatic-login consent at left, complete login materials at right; existing secrets are read-only until explicitly enabled for replacement. Temporary login pairs authorization options at left with credential entry at right inside one form. Inspection settings occupy their own local view; unsaved settings survive local navigation without causing a write.

At widths above 1050px and heights of at least 600px, directory and active form bodies scroll within the workspace while action footers remain visible. Login jobs use an approximately 300px master list beside a broad detail pane, with candidate confirmation at the footer. Explicit task selection reveals its row; a persistent outline distinguishes selection from hover. Polling preserves manual scrolling. Narrow or short windows use document flow; phones stack related fields in reading order and use full-width primary actions and 44px controls.

Authenticated details show complete email, password and TOTP secret as readable text; candidate review shows full identity identifiers. Closing or changing views clears loaded secrets, and stale requests cannot restore them. Plaintext is not stored in the browser. Local plaintext storage requires no encryption initialization; temporary login still saves credentials only when explicitly selected. Management authentication and explicit candidate confirmation remain product boundaries.

All three CPA theme roles, system typography and shared controls remain authoritative. Local navigation uses a simple active underline without a segmented shell or selection shadow. No new raster assets, remote fonts or decorative motion ship. Evidence belongs in `artifacts/login-content-8.0.0/` and `docs/OPENAI_UI_CONTENT_CN.md`.


### Account directory and import dialog — UI 8.2.0

The account directory uses a single bordered surface with a visible title, summary and right-aligned actions. Counted status filters precede search; the four-column table and pagination share the container. Email leads each row, followed by optional remark and full CPA identifier. Table rows use compact spacing, neutral hover and stable action positions. Narrow screens reflow into labeled account groups without horizontal document scrolling.

Batch import opens a native 720px dialog over the directory. Header and footer remain visible while long input/help content scrolls within the body. Format guidance precedes the textarea; live line count and local errors remain near the input. Closing clears the form and restores its opener. Saving disables dismissal and duplicate submission; navigation or disconnect still closes and clears it. The directory query, filter and scroll remain intact. Existing account detail and authorization destination editors still use their dedicated content view. Three CPA themes and full readable values remain authoritative. Reference: toSub2-go-port account-list hierarchy and batch modal, adapted to the CPA iframe and phone widths. Evidence: artifacts/account-layout-8.2.0/.


### Sparse account lists — UI 8.2.1

The account directory fits the actual rows rather than filling the desktop viewport. At its maximum available height, only the table body scrolls, retaining toolbar/header/pagination. A single unbound account must leave no large empty area inside the bordered panel. Status and authorization mode share a wrapping row; the reason occupies a secondary line. Activity is condensed only when every task/check/next-check field is absent. Cells align vertically, and single-page unfiltered lists omit repeated pagination. This supersedes the8.2.0 height behavior; import modal, full values and all other workspaces remain as before.


### Account actions in place — UI 8.3.0

Account login starts from the row and leaves the directory visible. Its button shows submission/active/waiting-confirmation state and its activity link opens task review only when requested. Account detail opens a native760px dialog with full readable values, a credentials group, settings group, optional existing-target disclosure for first authorization and a fixed action footer. The directory remains mounted and retains search/filter/page/scroll. Modal errors and retry are local; closing clears details and returns focus to the matching row. Submitted save/delete/start blocks dismissal and duplicates; auth loss and navigation still clear. This replaces the old editor/login pages, retaining the compact sparse-list height and import dialog.

### Management connection persistence — UI 8.3.1

A successful connection saves the plugin's own management key as plaintext in localStorage under `model-quality:management-key:/v8/management/plugins/model-quality`. Reload, reopen and restored browser history reconnect using that value without opening a dialog or submitting model or login work. Explicit disconnect and HTTP 401 remove it; transient network failures retain it. If browser storage is unavailable, keep the active connection in page memory and explain that refresh will require the key again. Keep the existing connection controls and layout. Do not read the parent CPA authentication store or persist account passwords, TOTP secrets or OAuth callbacks in the browser.

### Account diagnosis and login proxies — UI 8.4.0

The directory retains its content-sized bordered surface and now uses four aligned columns: account/CPA binding, authorization and inspection, login network, and actions. Email leads with the optional remark, full CPA identifier and automation consent below it. The status column prioritizes the latest login result, links explicitly to its task and shows the recorded stop step/reason when available. CPA health remains separately named when a task failure and existing credential state coexist. An old interaction-required record without diagnostic evidence says its stop step was not recorded; it must not assert that a human challenge actually occurred. Table actions retain direct in-place login and the account-detail modal.

Login network shows the complete effective proxy plus its source: account override, login default, CPA credential, CPA global or direct. Unsupported inherited proxies retain their original full address, display the fixed validation error and disable login until the configuration is corrected; they never look like a direct connection. No proxy credentials are masked. The toolbar exposes Proxy Settings, and the fourth local tab is now Proxy and Inspection. Its form places network defaults beside existing inspection controls on desktop and stacks them on phones. The content scrolls within the bounded desktop details body while its save footer remains outside that scroll region.

The default proxy, per-account override, temporary-login override and optional batch-import proxy are explicit text fields. Empty fields inherit; `direct` selects direct connection. The UI documents priority and supported HTTP/HTTPS and unauthenticated SOCKS5/SOCKS5H formats. Saving a proxy does not start login or alter the CPA business proxy. The temporary browser-authorization view distinguishes the user's browser network from the host's token-exchange proxy. Account and import proxy fields clear with their existing modal/secret lifecycle; the settings draft follows its existing explicit-save and conflict behavior. Fields and full URLs wrap without document overflow, the directory alone scrolls when dense, and phones preserve 44px primary targets.

Task details render only named diagnostic steps/reasons and fixed recovery guidance, with the stable error code available for support. Proxy failures point to proxy settings; explicit human/email/phone/workspace challenges point to browser authorization; unknown old records remain uncertain. This adds no automatic retry or candidate commit. Existing CPA themes, system typography, plaintext persistence and management-key recovery remain unchanged. Reference: toSub2-go-port account identity/status/action hierarchy and proxy settings, adapted to this plugin's available fields. Current evidence belongs under `artifacts/account-proxy-8.4.0/`.

### Account directory columns — UI 8.6.1

The user's explicit correction requires the five-column account table: Account, CPA state, Recent authorization, Login network and Actions. Keep account automation, full credential ID, observed state/reason, inspection times, authorization diagnostics/date and proxy source directly visible. Do not merge columns or hide these fields in a disclosure as a density optimization. The8.6.0 semantic list/disclosure experiment was rejected and is superseded.

Compactness is achieved through a single36px-control toolbar,10px cell padding, consistent top alignment and a170px action column. The table uses the available workspace width; long values wrap without masking or ellipses. Sparse data remains content-sized, dense tables scroll with pagination visible, and phones preserve all five content groups with44px controls. Existing CPA themes, system typography and native import/profile dialogs remain authoritative. Evidence: artifacts/account-columns-8.6.1/.


### Reference account table and inline logs — UI 8.8.0

The user's explicit authority is the current toSub2-go-port account table and inline log panel. Preserve its non-usage/probe content columns and retain the plugin's distinct CPA state and login-network columns. The plugin uses seven columns after removing the unsupported expiry field. Never reduce columns to make the table seem simpler. Use 42px headers, 8px vertical cell padding, middle alignment, compact typography, and a two-row three-column action grid with 29px buttons and 6px gaps. Full values wrap. Seven columns may scroll within the table on narrow desktops; phones keep all information and 44px controls. No placeholder controls for unsupported reset-2FA, usage, probes or uploads.

Open one log row directly below the account, spanning all seven columns. Match the reference's time/category/content grid, compact filters, refresh/copy tools, 270px local viewport and follow-latest footer. Current/history and all/abnormal use actual persisted events, with honest legacy and bounded-history notices. Polling preserves scroll, raw disclosure and keyboard focus; raw detail or manual scroll pauses follow. Read errors retain previously loaded events. Copy exports the same visible safe records. Normal account metadata stays in the directory, not inside logs.

The latest refinement explicitly removes the expiry column. A warning result above the event grid identifies the stopped stage, readable cause and next action; actual elapsed time is shown only from a complete retained start event. Event rows separate the human status from its explanation; raw codes and full job identifiers remain in expandable JSON. An authorization-phase runtime compatibility stop is not an account-health failure.


### Account wrapping refinement — UI 8.8.1

Status and the recorded diagnostic stage are separate unbroken phrases on the current-operation heading; its full reason sits below with pretty wrapping. Operation type has two left-aligned lines: intent and manual/automatic execution. Do not duplicate the operation outcome in this narrow column. Allocate more width to current operation while retaining seven columns and full long values; never use masking, ellipsis or clipping to hide ordinary metadata.


### Account-only login workspace — UI 8.9.0

The latest user explicitly removes the Login network column and Temporary login view. The account table now has six columns: Account, CPA state, Current operation, Operation time, Operation type and Actions. Redistribute the removed column width to the remaining data; use a1080px minimum width. Keep three local views: Accounts, Login tasks, Proxy and inspection. Remove the temporary form and its unused controls instead of hiding them. Preserve proxy configuration in account details and global settings; surface invalid proxy errors in current operation. Existing task review, candidate confirmation and historical browser callbacks remain supported. No recommendations may send users to the removed view.

### Read recovery and account actions — UI 9.3.0

This additive record documents plugin 0.16.0 within the established CPA themes, six-column account directory and three local views. It does not revise earlier layout decisions or global tokens. The reference is the read-only toSub2-go-port-2026.11.17 account, polling, history and security workflows; the implementation keeps the user's current account login and full-value display decisions.

Retain the last validated account/settings/job content when reads fail and name the affected data beside a retry action. Stale state must not authorize account writes. Only reads recover automatically; a submitted login, save or security action is never automatically replayed. Account state polling and account log loading each use a single in-flight cycle, with logs independent from the main refresh. Hidden/offline pages pause scheduled reads. A management 401 disconnects and clears account secrets; stale reads cannot repopulate a closed or replaced editor.

The active filter selects actual running account operations, including security and saving. A newly requested account-row 2FA login retains its approved one-action behavior: success saves and enables the CPA credential, keeps the directory visible and reports completion. Historical candidate review remains explicit. Account-scoped logs can show retained history outside the global latest-200 window, with honest per-account and per-task limits. Failed reads preserve existing events.

Account details show the effective saved proxy and its source, with copy feedback for complete login material. Security dialogs preview the selected execution order and task detail distinguishes completed steps from later failure or uncertainty. Import uses the original input line number, disables empty/over-100 submissions, preserves password bytes and validates supported TOTP material. Closing clears transient secrets. After any failed or uncertain account save, require a fresh detail read before another edit; cleared proxy/credential fields must not be resubmitted.

Evidence is documented in docs/LOGIN_EXPERIENCE_0_16_0_CN.md and artifacts/login-experience-0.16.0/. Offline Node regressions cover 16 cases; the Go build log records module tests and the Linux ARM64 package. CUA used six synthetic accounts to check recovery, filters, import, source/copy feedback, security steps, account history, simulated completion and 401 cleanup. Captures cover desktop/security plus 390px dark and 360px white with no horizontal document overflow. These checks do not certify real authentication or security mutations. Deployment completion is recorded separately in the release report.

The detector returned 47 existing findings: seven container-padding warnings and 40 historical token advisories. Review considered rendered child insets and captures; no new finding was attributed to the changed JS/CSS. The token configuration was not modified. No new artwork, fonts or raster product assets ship; screenshots are verification evidence.
