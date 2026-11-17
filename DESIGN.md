---
name: "Model Quality"
description: "Compact CPA task workspaces for manual question testing and credential quality operations."
colors:
  page: "#f7f6f2"
  card: "#fffdf9"
  input: "#ffffff"
  floating: "#fffdf9"
  hover: "#f0ede6"
  ink: "#2d2a26"
  muted: "#68625b"
  border: "#dedbd4"
  control-border: "#8e877e"
  accent: "#59564f"
  accent-hover: "#45423c"
  on-accent: "#ffffff"
  focus: "#59564f"
  focus-ring: "rgb(89 86 79 / 18%)"
  good: "#065f46"
  good-bg: "#e2f3e9"
  warn: "#85400e"
  warn-bg: "#fcf0d8"
  bad: "#963f32"
  bad-bg: "#f9e8e3"
  danger: "#a33e32"
  on-danger: "#ffffff"
  white-page: "#ffffff"
  white-card: "#ffffff"
  white-input: "#fafaf9"
  white-floating: "#ffffff"
  white-hover: "#f3f3f1"
  white-border: "#e5e5e5"
  dark-page: "#151412"
  dark-card: "#201e1b"
  dark-input: "#191715"
  dark-floating: "#27241f"
  dark-hover: "#302c26"
  dark-ink: "#f6f4f1"
  dark-muted: "#c9c3bb"
  dark-border: "#3a3530"
  dark-control-border: "#827b72"
  dark-accent: "#d5cfc4"
  dark-accent-hover: "#e0d9d0"
  dark-on-accent: "#1d1b18"
  dark-focus: "#c9c3bb"
  dark-focus-ring: "rgb(201 195 187 / 24%)"
  dark-good: "#6ee7b7"
  dark-good-bg: "#143b2f"
  dark-warn: "#fcd34d"
  dark-warn-bg: "#3d301b"
  dark-bad: "#f1b0a6"
  dark-bad-bg: "#442820"
  dark-danger: "#efb0a5"
  dark-on-danger: "#442820"
typography:
  brand:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, \"Helvetica Neue\", Arial, \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "17px"
    fontWeight: 700
    lineHeight: 1.5
  headline:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", Roboto, \"Helvetica Neue\", Arial, \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "28px"
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
  card: "#fffdf9"
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

## Overview

**Creative North Star: "The CPA Quality Workspace"**

Model Quality follows the CPA plugin-management page and its shared UI components. Warm light, white and warm dark themes retain the host's surface hierarchy, system typography and restrained controls. Consistency makes the administrator's current task, evidence and next action easy to find.

The plugin supplies task navigation inside the existing CPA shell and keeps direct access complete. Buttons, fields, cards, underlined navigation, switches, tables and focused dialogs share host contracts; adjustments serve readable contrast, mobile input and the plugin's precise state semantics. The interface uses native HTML, CSS and ES modules with Go embed. No raster artwork, remote fonts or runtime component library ships.

**Key Characteristics:**

- Separate page, card, input, floating and hover roles in all three themes.
- Host system type, eight-pixel controls and twelve-pixel working surfaces; primary navigation names the task without a repeated visible page title.
- Compact configuration stays above current answers; only answers and history switch locally, and explicit expanded reading preserves the mounted draft.
- Underlined task navigation, consistent table density and native accessible control behavior.
- Independent labels for execution, quality, rule state and credential restrictions.

The shared manual-testing composition records UI 4.9.0; the independent OpenAI workspace is redesigned in UI 8.0.0 in `plugin/ui/`. The manual form places credential selection immediately left of question selection above the wide left prompt editor. The right column groups native model selection, capability-based reasoning, a fixed one-to-eight answer selector and the start action. The desktop target keeps configuration, the primary action and the start of answers together in a 1280×720 window. Product semantics remain in `PRODUCT.md`; this route's Operate strategy lives in `.impeccable/surfaces/plugin-ui-index-html.md`. The pinned CPA component direction remains authoritative. Current verification artifacts belong in `artifacts/ui-polish-4.9.0/`; only completed reports establish passing checks. Captures from `artifacts/samepage-4.6.0/` and earlier remain historical.

## Colors

Frontmatter is the exact color source for this document. Unprefixed tokens are light; `white-` and `dark-` tokens override the matching role. The fixed upstream reference is `router-for-me/Cli-Proxy-API-Management-Center` at `d0126752201a35ff999bd2a1d4741215542258a8`, especially `themes.scss`, `variables.scss` and `components.scss`. The component and license inventory is in `plugin/NOTICE.md`.

### Primary

- **Warm Stone Action** (`accent`, `accent-hover`, `on-accent`) distinguishes the explicit submit. In dark mode it uses a light neutral fill with dark text.
- **Neutral Focus** (`focus`, `focus-ring`) marks the keyboard and text-editing location. Field focus uses a border plus a soft ring; other keyboard targets keep an offset outline.
- **Consequential Action** (`danger`, `on-danger`) fills only the final consequential confirmation. Other consequential actions use outlined error text.

### Semantic states

Deep green, amber and clay pairs identify named positive, attention and error states. Complete execution and comparability remain neutral. Missing usage stays unknown and recorded zero remains zero. Manual release feedback does not imply full credential availability.

**The Separate State Rule.** Execution, answer quality, rule enablement and credential restrictions are different facts. Pair color with precise text and never derive one state from another.

### Neutral

Page, card, input, floating and hover are distinct roles. Warm light uses an off-white working surface over a quiet warm page, with white form fields. White keeps white page/card/floating surfaces and subtly shaded inputs. Dark separates the raised working surface from darker fields. Cards use a single fine border without a shadow; floating dialogs use shadow without a second border. Primary and secondary ink carry reading; fine borders divide content and strengthened control borders identify fields and secondary actions.

The host's normal action gray with white small text is approximately 3.61:1, below the ordinary-text target. This implementation uses a stronger graphite action shade and strengthened secondary text and control boundaries; current ratios are recorded in the completed contrast report. These purposeful deviations keep the host family. Targets are 4.5:1 for ordinary text and 3:1 for applicable large text, controls and states; they do not assert accessibility certification. Historical UI 4.6.0 measurements are in `artifacts/samepage-4.6.0/verification/fixtures/contrast-results.json`; current verification belongs under `artifacts/ui-polish-4.9.0/` and must be read separately.

When embedded in a recognizable same-origin CPA page, the plugin follows only the parent's public applied root theme: missing attribute means light, `white` means white and `dark` means dark. A MutationObserver tracks changes. The host's auto mode applies white in system light, so white must not collapse into light. The local selector is hidden and disabled while following. Unrecognized parent, cross-origin access, unsupported theme or observer failure falls back to a local in-memory selector; no parent key, store or browser storage is read.

## Typography

The UI uses the host's system-font approach with explicit Chinese fallback. Titles are strong, body copy is compact and ordinary model prose is larger for reading. Code, HTML, SVG and raw diagnostics use the local monospace stack. No remote font loads.

The frontmatter records exact roles: page headline, card title, section, body, label, supporting copy, metadata, state, prose answer, code answer, diagnostic source and compact 24px metric. Page headline tokens remain available, but all three task H1 elements are screen-reader-only because the visible primary navigation already names the active task. Explanatory paragraphs have a readable maximum measure (75ch). Long identifiers and original source wrap; timing, usage, dates and comparable values use tabular numerals.

Supporting metadata can be smaller than the body, but restriction, action and verdict meaning must remain in readable named text. Phone inputs and selects use at least 16px to preserve editing usability.

## Layout

The host owns the global shell, sidebar and floating tools. The plugin header carries only task navigation, connection/freshness tools and a compact identity in direct mode. Task H1 elements remain available to assistive technology while the duplicate visible title/description rows are removed. The disconnected management connection is a compact button at the right of the primary tabs. Its explanatory copy lives in the existing connection dialog; no separate banner occupies a workspace row. The connected state replaces the button with connection status and freshness tools.

The workspace is bounded to 1440px with host-related responsive gutters. Embedded mode hides the wordmark and clears the pinned host toolbar vertically: the plugin header begins after an 80px desktop or 68px narrow-screen top inset. This adapts the fixed reference tools at y=24–76 desktop / y=12–60 mobile; it is not a claimed host geometry API. Sidebar width remains entirely outside the iframe.

At a CSS viewport width of at least 801px and height of at least 600px, the body and current workspace use the available dynamic viewport height. Compact configuration remains visible above the answer surface by default. Only current answers and history switch locally inside that surface. Starting a task shows its answers without hiding the form. An explicit “展开阅读” control temporarily hides configuration; “返回配置” restores the same mounted form and draft. The normal 1280×720 desktop target includes the configuration, primary submit and the start of the answers together. Unbounded prompts, outputs and records still need local scrolling.

The configuration card is a two-column workspace separated by whitespace, without a vertical divider. The wide left column places the credential selector immediately left of the question selector, above the editable prompt. Version and edited-state feedback sit below the editor. The reset action is removed. The right column is 260–320px wide and groups model selection, reasoning and answer count, then a run summary and full-width start action directly below. On wide desktop the prompt starts at 128px and grows with the settings column, keeping the disconnected options-to-action gap near 12–16px while retaining bottom alignment. Long prompts scroll inside the editor. Tablet and short-window prompts retain a 160px base; phones retain 180px. Normal credential status and model count sit beside their labels; blocked or unavailable states receive a full-width explanation below the control. Unknown reasoning retains its default-setting note. Before model selection the helper is hidden. The start action aligns with the bottom of the prompt editor; the caption occupies a separate grid row, so version feedback does not push the button below the editor. Option groups keep a 12px minimum gap without a horizontal action separator. Compact header controls stay beside the tabs when space allows and wrap without squeezing labels on narrow screens. At 800px and below, the question workspace comes first, then model settings and start. At 600px and below, credential and question selectors also stack. Native controls preserve readable type and 44px phone touch targets; DOM focus order follows the visual task order.

The answer/history surface sizes to short content by default, with its height bounded by the remaining desktop space. Explicit expanded reading fills that space. Its underlined navigation and reading control sit outside the scrolling results/history region, so switching views cannot obscure the heading. Empty history has no forced full-height body and no pagination; pagination appears only when needed and follows the records. Prose answers are capped at 80ch while code and side-by-side results retain their available width. A compact run summary carries the model, execution status, progress and question alongside the frozen-condition disclosure. Single-model manual results omit repeated model group headings, sample model labels and inapplicable comparison/filter controls. Task actions remain available. Short answers retain natural height, while long answers support explicit expanded reading. Editable configuration and frozen execution conditions remain distinct.

OpenAI login is a third primary tab with its own disconnected state, form and task list. Leaving the tab clears secret inputs without cancelling host tasks; returning reloads task state. On phones, all three tabs occupy a full navigation row and connection tools wrap below. The login form and task details use normal document flow on narrow or short screens, with a bounded body on desktop.

Quality operations use the same component foundation. Attention, rule lists and audit size to their content while remaining bounded on desktop; short lists no longer stretch into large empty panels. Rule table edges are flat, with quieter secondary actions and consistent status tags. Each rule keeps execution/pause on the first action row and edit/delete together on the second. Edit sections use a 20px rhythm, and the manual release consequence uses the warning surface to highlight that dependent rules remain stopped. On desktop, attention and credential restrictions share two columns, with secondary statistics spanning the row below. Each content-heavy panel can scroll locally. Rules retain a unified table and audit retains a list. Long rule and global-budget editors remain inline; their body scrolls while the title/close control and save footer stay outside the scrolling body. Rule fields retain the five groups for target/question, judgment, scheduling, credential action and call budget/retention.

| Viewport | Adaptation |
| --- | --- |
| At least 801px wide and 600px high | Compact configuration above answers: credential/question selectors and prompt on the left; model, reasoning, answer count and start on the right; primary actions outside local scroll regions. |
| At or below 1100px wide | Rule title columns narrow; wide rule tables scroll inside their wrapper. |
| At or below 800px wide | Normal document flow; credential/question selection and prompt precede model settings and start; rule groups stack in task order. |
| Below 600px high | Normal document flow even on a wide viewport, allowing zoom and short windows to expose every control. |
| At or below 600px wide | Inputs remain at least 16px and primary touch targets at least 44px; tables become labeled rows and answers use one column. |

The actual iframe CSS viewport determines these changes. Native dialogs retain dynamic maximum height and safe-area padding. Narrow and short layouts deliberately allow document scrolling so content remains reachable under mobile keyboards and browser zoom. Long identifiers wrap; local scrolling must not produce document-wide horizontal overflow.

**The First Viewport Rule.** Keep normal desktop configuration, primary actions and the start of the answers visible together. Use local scrolling for unbounded content, and restore document flow on narrow or short viewports instead of clipping controls or reducing readable type.

**The Reading Continuity Rule.** Polling updates changed evidence without replacing stable drafts, focused controls, selections, disclosures or reading positions.

## Elevation & Depth

CPA-style cards combine a fine structural edge and a small soft shadow. Floating dialogs and the mobile action menu use the deeper shadow. Ordinary groups inside cards use spacing or separators rather than extra elevated containers. Surface tones remain deliberate across all three themes. No decorative gradient, raster texture or entrance animation ships.

The sidecar records card/dialog shadows and motion. Shared control feedback uses 150ms; switch movement and progress use 200ms. Polling never replays entrance motion. Reduced motion disables transitions and animations. These are deliberate adaptations of the host's shared feedback language, without its larger page entrance effects.

## Shapes

Control corners are eight pixels; cards and native dialogs are twelve. Small state tags use four pixels. Switch tracks are 44×24 with an 18px thumb; the native checkbox remains the focusable control and falls back to native appearance in forced colors. Underlined tabs retain rounded top corners and a two-pixel inset selection line. Inline icons use one restrained line language; existing independently authored SVG paths remain.

## Components

**The Host Component Rule.** Use the fixed CPA plugin-management page and shared primitives as the visual authority. Adapt contrast and responsive dimensions explicitly; preserve the host role and interaction contract.

### Buttons

Primary, secondary, ghost and consequential variants share dimensions, weight, spacing and radius. The filled primary action identifies the explicit submit; secondary controls have a readable boundary; ghost actions defer to nearby content. Final deletion/release confirmation receives the filled danger pair while earlier stop/release affordances stay outlined. Small desktop buttons retain readable 14px type. Busy actions preserve their measured width, disable duplicate submission and announce current state. Disabled and active states are distinct; keyboard focus is visible.

### Inputs and selections

Native input, textarea and select preserve familiar editing and keyboard behavior. The manual model field is a real select populated from the selected credential's registered catalog; users do not need to type an ID. Answer count is a select with exactly one to eight choices. Visible labels precede controls, compact normal hints may share their row, and expanded errors remain adjacent and associated. Validation marks `aria-invalid` and preserves input on failure. Text fields use the input role, control border and host-style focus ring. Native selects retain their browser keyboard behavior; the plugin does not claim a custom searchable combobox.

Reasoning always offers the omitted/default setting, plus only the selected model's declared supported levels, including `max` when advertised. Host flags may add off or automatic choices. Unknown capability and capabilities with no usable named levels leave the default selected and explain the limit. Active credential/model changes reset incompatible choices; background refresh preserves a removed model or reasoning level as a disabled stale option and blocks submission until the administrator explicitly reselects. Empty catalogs disable execution and point back to CPA configuration and refresh; they never become a free-text fallback.

### Cards, tables and navigation

A card uses the host title/actions/body vocabulary, compact 16px padding and restrained border/shadow. The selected local view is the working surface; a redundant card title is omitted when the active navigation already names it. Tables share header fill, aligned padding, top-aligned cells, row separators and hover. Numeric data uses tabular figures; actions wrap inside their allocated column. Underlined ProviderTabs inform both task and local navigation, including selected weight, inset underline, hover and focus. Narrow navigation scrolls locally without introducing a second sidebar.

### Switches and states

Rule enablement, scheduling and automatic pause are three independent native checkboxes with switch styling. They change a draft until saved, never immediately execute a rule. State tags and notices include explicit meaning in text. Empty, not connected, loading, no matches and error states retain their own explanations rather than treating every absence as zero data.

### Answer reading

The current manual task presents one model and up to eight independent answers, two columns when space permits and one on phones. Its model identity appears once in the run summary; answer cards emphasize sample number, execution state, original answer, timing/usage and nearby actions. Single-model manual results hide redundant model headings and comparison/filter controls. Legacy multi-model histories and rule evidence keep their applicable comparison and filtering controls. Prose/code presentation changes typography only: original model content remains escaped and copy/export preserves original text. Selection, expanded answers, review drafts and internal scroll positions survive unchanged polling results. On phones, a local sticky sample heading preserves sample identity while an expanded answer is read; the global header stays in normal flow.

### Editors and protected dialogs

Long rules and global budget settings stay inline. In the bounded desktop workspace their body scrolls locally and the save footer remains visible. Closing a dirty draft requires an explicit discard decision; stale async responses cannot overwrite a newer editor. Connection and short confirmations use native modal dialogs with labeled headings, separated actions, bounded scrolling, Escape and focus restoration. Evidence and preview use wider dialogs when reading requires it. The native dialog supplies background non-interactivity; no framework modal implementation is added.

The release dialog keeps exact ownership, identity, version, scope and remaining restrictions together. Conflict and uncertain outcomes expose refresh/reconciliation and disable unsupported repeat confirmation. Release leaves dependent rules stopped and never promises general availability. The HTML/SVG preview retains empty sandbox, inert state, strict CSP and blocked scripts/network/forms/navigation.

**The Bounded Release Rule.** Show the exact owned pause, identity, action version, consequences and remaining restrictions together. Keep conflict and uncertain outcomes visible until reconciled.

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
