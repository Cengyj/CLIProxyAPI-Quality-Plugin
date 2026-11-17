---
version: 1
slug: "plugin-ui-index-html"
primary_target: "plugin/ui/index.html"
related_targets: ["plugin/ui/app.js","plugin/ui/style.css","plugin/ui/view.js","plugin/ui/login.js"]
---

# Model Quality in CPA

Mode: Operate. Audience: a self-hosted CPA administrator. The primary setting is the CPA management iframe; direct access remains complete. The fixed CPA plugin-management page and shared UI components remain the visual authority. The 2026-11-15 instruction prioritizes configuration and primary actions in one desktop viewport, especially 1280×720, while long answers, history and rule bodies use local scrolling.

## Direction contract

THESIS: Manual evaluation, quality operations and OpenAI account management belong to the CPA management product, with one host shell and a consistent component vocabulary. Account management remains its own primary tab, independent of model evaluation.

OWN-WORLD: CPA warm light, white and dark surfaces; system typography; understated bordered cards; eight-pixel controls; underlined ProviderTabs; named state colors. Preserve readable contrast where the reference is too faint. The administrator works on desktop or phone in daylight and evening light.

STORY: Choose the credential and question together, edit the prompt on the left, then select the registered model and request one to eight answers using the right-side start action. Read results below. A model is always selected from the actual credential directory. Unknown reasoning keeps the default. Refresh preserves valid selections and drafts; disappeared models require explicit reselection. Quality operations retain owned pauses and explicit release.

FIRST VIEWPORT: A wide left column places the credential selector immediately left of the question selector, with the editable prompt below both. The caption carries version and edited-state feedback; there is no reset button. A narrow right column contains model, reasoning and answer count, followed by the run summary and a full-width start button. Answers remain below the configuration. Narrow screens stack question work before model settings and start; phone selectors stack without reducing readable type.

FORM: seed: none (pinned). User: “凭证选择放在题目左边编辑框也是放在左边开始测试放在思考设置和模型选择下面”. Preserve the CPA component world and existing model-directory behavior. DOM and keyboard order follow credential, question, prompt, model, reasoning, count and start. Preserve mounted drafts across answers, history, refresh and expanded reading; honor reduced motion.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance.

## Verification boundary

Use an isolated actual CPA host with the rebuilt native library, fake upstream and fake credentials for submissions, cancellation, history, safe preview, pause rejection and manual release. Theme following reads only the recognizable same-origin parent's public applied theme attribute, with standalone fallback. Check all three themes, standalone/embedded, required desktop/tablet/mobile widths, sidebar states and zoom. Preserve the plugin's own endpoint-scoped management-key persistence and explicit clearing, original model content, stopped dependent rules after release and served-source hashes. Account passwords, TOTP secrets and OAuth callbacks must remain outside browser storage.

The user has authorized updating the existing local CPA instance on port 8317. After validating the new library, update that instance with configuration, credentials, databases and pause ownership preserved; keep a previous-library/state backup. All development submissions and regression tests still use isolated fake upstreams and fake credentials. Existing demonstrations remain separate. Current evidence belongs in `artifacts/ui-polish-4.9.0/`; earlier reports and the immutable 4.3 candidate remain historical and do not certify this revision. Document only checks actually executed.

UI 4.7.2 removes the reset action and the default start/configuration and connection guidance beneath the settings. The action stays disabled until authorized and valid; connected credential/model errors remain visible.

UI 4.7.3 hides the initial reasoning helper and puts the start group 12px below options with no auto spacer or horizontal separator.

UI 4.8.0 aligns Start with the editor bottom while keeping version feedback in its own grid row. The normal answer panel shrinks to content; expanded reading uses available space. Results and history scroll below a fixed toolbar, empty history omits pagination, and prose has an 80ch reading measure. This supersedes the 4.7.3 fixed-action-gap layout.

UI 4.8.1 compacts the disconnected model/action stack by giving the desktop editor a 128px base; both columns still grow with real guidance and retain bottom alignment. The disconnected connection action moves to the right of the primary tabs, with supporting text in its existing dialog and no banner row. Header controls wrap naturally on narrow screens; embedded phone tabs and Connect share a row.

UI 4.9.0 refines the whole plugin within the CPA visual family: lighter warm working surfaces, flat bordered cards, stronger neutral primary actions, shared navigation icons/status tags, readable answer insets, compact rule and audit lists, tighter editor groups, and an explicit warning surface for manual-release consequences. Prior manual arrangement and compact action/header geometry remain binding.


UI 6.0.0 expands the separate OpenAI tab into account management. Its default account list shows masked identity, CPA binding, locally observed health, inspection timing, automatic-login permission and latest login task. “Accounts”, “Temporary login” and “Login tasks” are local views, preserving the existing manual/browser login workflow. Bulk import uses email----password----2FA and saves encrypted credentials without starting a login. Login material is cleared after submission, navigation and disconnect; it never enters browser storage.

The account toolbar keeps inspection and import adjacent. Periodic inspection is a compact disclosure with explicit save; automatic authorization requires both scheduled inspection and a separate permission for each bound account. Disabling inspection also clears the automatic-authorization checkbox. First login still requires candidate review before CPA binding. Status text distinguishes locally observed auth failures from live account validity, and manual inspection never initiates login. Desktop uses a readable account table; phones use labeled rows without document overflow. Task status and task link occupy separate lines.

Current account UI evidence belongs to `artifacts/account-manager-6.0.0/`. The original 21 offline login-workbench checks were adapted for the three local views and remain active. Account checks cover encrypted import, parser fidelity, explicit settings, first binding, manual inspection, secret clearing, deletion, three themes and mobile widths. All account UI verification uses loopback fake accounts and fake credentials. No raster assets were added. Prior UI evidence remains historical and does not certify these changes.


## OpenAI UI7.0.0 direction (supersedes6.0 composition)

# OpenAI account workspace redesign

Mode: Operate. This scope replaces the rejected OpenAI workspace composition while retaining the CPA shell, shared three-theme tokens, and existing model-testing and quality workflows.

## Direction contract

THESIS: The account directory is the working surface. Full identities and immediate actions lead; settings and editing occupy a focused utility rail instead of competing above the list.

OWN-WORLD: CPA system typography, restrained warm/white/dark surfaces, neutral primary buttons, existing status colors, crisp rules and aligned fields. Remove the giant enclosing card and redundant headings from the login workspace.

STORY: Find an account by email or CPA identifier, inspect its recorded state, open its full login material, or authorize it. Import and periodic inspection are distinct actions; manual candidate confirmation remains explicit.

FIRST VIEWPORT: A compact heading and local navigation sit above a broad searchable four-column directory and a 320px utility rail. The rail presents inspection settings by default and switches to import, account details or login target selection. Tasks use a persistent master list and detail pane. Phones stack the working panel ahead of the directory when editing; full identities wrap without truncation.

FORM: seed: none (pinned). User: “OpenAI 登录相关的针对ui彻底重构美化重新调整布局…不需要任何脱敏”; the established CPA component world and task scope remain fixed, and the user has delegated layout decisions. Account emails, identity IDs, passwords and TOTP inputs are readable in authenticated management views. No raster assets.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance.

## Verification

Use only offline fake accounts and authentication servers. Verify account import/edit, full-value rendering, source switching and stale detail clearing, search/filter/pagination, candidate confirmation, async settings conflict, three themes and narrow screens. Review desktop1280x720 and1440x900 plus390/360px captures. Current evidence belongs under artifacts/login-redesign-7.0.0. Keep the global design tokens and other workspaces unchanged. Update the existing8317 host and plugin only after tests, backup and source/artifact consistency checks; preserve all stored user state.
# OpenAI content workspace — UI 8.0.0

Mode: Operate. Local composition replacement within the established CPA visual system. User again asks “重新优化登陆标签页内容ui，内容重新排版重构”, after explicitly delegating layout decisions and requesting full unmasked information. The previous utility-rail composition is rejected evidence.

## Direction contract

THESIS: Give each task its own working width. The default account directory owns the full content area; settings and editors no longer compress every account row.

OWN-WORLD: Retain CPA's warm light, white and warm dark roles, system type, restrained neutral actions and named status colors. Use flat underlined local navigation, fine table rules and generous readable fields. No new artwork, fonts or decoration.

STORY: Find an account, inspect its status, open its complete login material or authorize it. Import and editing replace the directory with a focused wide form and an explicit return. Periodic inspection has its own local view. Temporary login separates authorization choices from credential entry. Task review keeps the selected identity beside its full result.

FIRST VIEWPORT: A concise workspace title leads four local views: account directory, temporary login, login tasks and inspection settings. The default directory has one compact toolbar for search/filter/import/inspect actions and a full-width four-column table. No permanent side rail. Active editors use available width with clearly grouped fields and visible footer actions. Desktop temporary login places settings in a narrow left column and credential entry on the right within one form; task history uses a 300px master list with a broad detail region. Phone follows DOM order with full-width controls and normal scrolling.

FORM: seed: none (pinned). This is an existing CPA Operate surface; prior user instruction “根据你自己的判断…详细优化” delegates composition choices and “不需要任何脱敏” requires readable values. Scope is content organization within the established product, not a new visual identity. Signature interaction: switching between directory and editor preserves the directory query while keeping only the active task surface visible. Selection and polling preserve reading position. Motion uses existing focus/hover transitions and reduced-motion behavior only.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance.

## Verification

Use synthetic accounts and loopback fixtures only. Cover all four views, import/editor/back flows, secret clearing, async reads, settings drafts, search/filter/pagination, candidate identity, all three themes, desktop1280x720 and1440x900 plus390/360/768 widths. Keep save/start/confirm visible at desktop. Preserve backend semantics, explicit first commit and independent switches. Evidence: artifacts/login-content-8.0.0. Update local8317 plugin only after checks and backup; host and login runtime remain unchanged.


## Account directory layout — UI 8.2.0

Mode: Operate. Refine the existing CPA world, referencing the read-only toSub2-go-port account toolbar and batch-import dialog. The directory remains the primary working surface: title/count and import/inspect actions lead, counted filters and search form the next row, then full identities, CPA status, activity and actions align in four columns. One bordered surface integrates toolbar, table and pagination. Related metadata stays close; 20px surface insets and 12–16px control spacing distinguish groups.

Import is an explicit native modal requested by the user, about 720px wide. It preserves the underlying query/filter/scroll, puts format guidance above a large textarea, shows nonempty line count and local validation, and keeps Cancel/Import in a fixed dialog footer. Escape/cancel clear inputs and restore the exact opener; an in-flight save prevents duplicate submission and user dismissal. Auth loss or navigation clears and closes it. On phones the dialog fills available width with safe margins and a scrollable body; account rows become labeled stacked groups with complete wrapping values.

Keep full unmasked values, plaintext persistence, no migrations, explicit candidate review and separate automation permissions. No new product behavior, artwork or fonts. Verify synthetic data only, three themes, desktop1280×720/1440×900 and360/390/768 widths, keyboard focus, modal lifecycle, error/busy states and installed local CPA8317. Evidence: artifacts/account-layout-8.2.0/.


## Sparse directory correction — UI 8.2.1

User screenshot rejects the single-account directory's large blank bordered body. This evidence supersedes the8.2.0 full-height directory choice. Keep the CPA Operate world and import modal, but let the list panel fit its content; long lists alone shrink into bounded local scrolling. The email still leads, status and authorization mode share a line, reason follows once, and a truly empty activity cell has one muted line. Rows align their contents vertically. Single-page, unfiltered results omit the redundant pager. Empty and no-result states also fit their contents; editing/settings retain their own bounded workspaces. Verify the supplied screenshot's one-unbound-account case explicitly alongside dense data, mobile,3themes and import lifecycle. Evidence: artifacts/account-compact-8.2.1/.


## Account actions without page replacement — UI 8.3.0

Mode: Operate. User rejects the page changes for2FA login and account detail and requests reference to toSub2-go-port. Preserve the current compact directory and CPA themes. The reference uses in-place dialogs and leaves login progress in account rows. This implementation simplifies the default path further: a stored-account row starts2FA with its default new/bound target and stays in the directory. No task tab is selected automatically. The row shows submission and latest login status; the operator explicitly opens its task to inspect or confirm a candidate.

Account details use a760px native dialog over the directory, with full email/password/TOTP values, grouped settings, local loading/error/retry and a fixed save/close footer. An optional disclosure preserves selecting an existing CPA target for unbound accounts. Closing clears plaintext and restores row focus without changing search/page/scroll; save/delete/login submission blocks dismissal, while feature navigation and authentication loss force cleanup. Explicit candidate confirmation and separate automation permission remain unchanged. Verify direct starts and no duplicate requests, errors, stale detail and async saves, keyboardfocus, sparse/dense lists,3themes and phone layouts with synthetic fixtures. Evidence: artifacts/account-actions-8.3.0/.

## Management connection persistence — UI 8.3.1

The user wants refresh to retain the CPA connection. Save the plugin's successfully authenticated management key as plaintext in its own localStorage entry, scoped by origin and management API path. Restore on reload, reopen and browser history restoration. Explicit disconnect and HTTP 401 clear it; temporary network failure retains it. Explain storage denial and continue with a page-memory connection. Keep existing connection layout; no design refactor is needed. Never read the parent CPA authentication store or persist account passwords, TOTP secrets or OAuth callbacks. Automatic reconnection only reads management data and does not start model or login tasks.

## Account diagnosis and proxies — UI 8.4.0

Mode: Operate. User asks to reference toSub2-go-port's account layout, diagnose failed 2FA login and support proxy configuration. Refine the pinned CPA component world and preserve direct row login, modal import/detail, complete readable values, explicit candidate confirmation and the independent automation switches.

The task path is identity → latest authorization result and observed CPA state → actual login network → explicit action. The table uses those four columns. A failed task receives a concise stop-step/reason with a separate task link; existing CPA health stays distinct. Historic interaction failures lacking diagnostics explicitly state missing evidence. Network cells show the complete effective address and source; inherited invalid or unsupported proxy configuration shows its validation error and blocks login rather than pretending to be direct. Sparse directories end at their row, dense directories retain bounded table scrolling, and phones reflow into labeled account groups.

Proxy Settings is visible in the directory toolbar. The fourth local view, Proxy and Inspection, groups the default login network alongside periodic checks. Per-account proxy overrides live in the existing account modal; temporary login and batch import expose a compact proxy disclosure. Empty means inheritance, `direct` means direct connection. The browser-authorization helper states that an external browser uses its own network while the host applies this proxy during token exchange. Settings save explicitly, preserve drafts and detect concurrent revision conflicts. No save triggers a login, model call or business-proxy change. Proxy fields are not encrypted or persisted in browser storage.

Desktop settings retain a fixed save footer through the native details content flex boundary; long settings scroll inside. Phone settings use normal document flow. Fixed UI diagnosis text maps only allowed error enums, with unknown historical evidence stated honestly. Verify proxy forwarding for all four entry points, inheritance and clearing, inherited proxy validation errors, task diagnosis and old records, modal lifecycle, three themes, 1280×720, 1440px, 360/390px and tablet layouts with synthetic fixtures. Evidence: `artifacts/account-proxy-8.4.0/`. Final source-bound checks and actual installation reports establish completion; earlier screenshots and checks remain historical.


## Direction contract — account directory UI 8.5.0

THESIS: A compact account console following the user-selected toSub2 reference. Identity, observed CPA state, last authorization, network and deliberate actions occupy separate readable columns.

OWN-WORLD: Inherit the CPA themes and system typography. Use white/card-colored rows, a soft neutral header, restrained separators, content-width outlined action buttons and consistent 16px stroke icons.

STORY: Scan account totals, filter by attention or binding, read the actual account state, then login or inspect details in place. Existing authentication, proxy and persistence behavior remains authoritative.

FIRST VIEWPORT: Four compact 80px summary entries sit above one directory surface. A single toolbar joins title, search, proxy, inspection and import. The inset table has five columns, an approximately 88px ordinary row, a fixed compact action column and no empty full-height panel. On phones the summaries form a two-column grid and rows become labeled groups.

FORM: seed: none (pinned), user: “参考目标项目重新重构ui美化”, reference toSub2-go-port account-table-compact-20261104-wide.jpg. Reuse only implemented data; binding never implies usability.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance.

### Implemented account directory 8.5.0

The directory now uses four live summary actions (all, attention, linked and active login tasks), one title/search/action toolbar, and five independent columns: account, observed CPA state, latest authorization, login network and actions. The account panel has a 16px table inset and content-sized height for sparse data. Desktop actions are horizontal; phones stack labeled cells and use 44px controls. Summary entries are 84px normally and 64px at short desktop heights. The 1554×605 dense fixture exposes the complete first row and second-row actions above fixed pagination. Unknown state stays neutral even after a committed authorization; binding is not an availability claim. Full values wrap without masking.

The scope is a component/layout replacement within the existing CPA design system. Compared against DESIGN.md: page/card/input/foreground/border and status colors, three themes, system font, 8px control/12px surface radii, focus treatment and modal contracts are retained. DESIGN.md and the token sidecar are preserved; detector schema drift is pre-existing and is not repaired here. Component-specific sizes (15px toolbar title, 25px normal statistic) are documented here rather than redefining the global type scale. Static wrapper-padding warnings were checked against rendered child-cell and modal insets.

Evidence: artifacts/account-redesign-8.5.0/account-ui-final/report.json (43 checks, 89 captures); verification-final (16 source-matched records, six unchanged host records reused). No real accounts or model calls were used in verification. Fresh independent finish review is recorded separately in finish-review.md.

Final independent review: ship, with a non-blocking phone observation that the first account actions require a short document scroll at 390×844. Final captures and source hashes are recorded in account-ui-final; no production edits followed those captures. The review covers visual layout, not real OpenAI authentication.

## Direction contract — compact account directory 8.5.1

User rejected 8.5.0 as bloated with a fresh screenshot. This supersedes the four-card overview and inset account panel. The account directory remains an Operate surface within existing CPA themes. Remove the four summary cards, icons and oversized counts; retain the same live filters as compact inline buttons. Remove repeated directory title/count and outer account card. Align compact filters, search and actions in one desktop toolbar immediately above a flat five-column table. Ordinary cells use 10px vertical padding; unlimited identity/proxy strings wrap naturally. Mobile uses a single filter row, full-width search and 44px actions. Preserve honest CPA versus login state, task/candidate workflow and unmasked content. Check user's 1471×510 crop context plus existing1554×605,1280×720,1440×900,390×844 captures. Prior ship verdict is superseded by user evidence and is not acceptance for this revision.

8.5.1 implementation evidence: final account UI43 checks/89captures, source-bound changed=[]; all16 release checks pass. At1554×605 the sparse content bottom moves438→321px; at390×844 row actions move883→675px and retain44px height. No global design tokens/fonts/radii or modal flows changed. The existing DESIGN.md and sidecar are preserved; local container removal supersedes prior four-card/inset guidance. Detector wrapper-padding warnings were inspected against actual child-cell/modal insets; unrelated old sidecar advisories remain unchanged. Final independent review is documented under artifacts/account-compact-8.5.1/finish-review.md.

8.5.1 independent final disposition: ship. Reviewer opened13 final captures and verified six source hashes. Non-blocking768px horizontal-table tradeoff remains; desktop/phone compactness improved. Local install preserves pre/post state hashes and all six served assets match source.

## Direction contract — account rows UI 8.6.0

Latest user rejected the8.5.1 five-column report as still unchanged. User screenshot d5f6a8ca is the anti-reference. Remove the table model and its header, not merely padding: a semantic account list clusters full email, CPA state and recent authorization together, with proxy and direct login/profile actions at the right. Bound the directory to1120px rather than stretch five disconnected columns over1360px. No account summary cards or nested card framing. Secondary CPA ID, inspection times, automation policy and complete recent-login diagnostics expand inline through an accessible button with aria-controls/expanded. Main-row auth failures and proxy errors remain visible. Expansion and focus survive data refresh, and expanding never sends a write. Native button disclosure is used so details do not consume a permanent third row. Full content remains unmasked and wraps; mobile44px controls. Existing CPA themes and account dialogs remain. Fresh reviewer must evaluate real grouping change against the user's rejection, not certify reduced padding.

8.6.0 final evidence:45 account checks/91 captures;16 final source-matched validation records (six unchanged-host records reused). Fresh independent reviewer inspected11 final captures including the user-equivalent unbound/interaction-required/direct state, then returned ship at account-directory visual scope. No real authentication or model call is implied. DESIGN.md received an additive semantic-account-list component note without rewriting tokens or unrelated surfaces. Mechanical detector warnings were limited to existing modal/guide wrapper padding with child insets.

## Direction correction — UI 8.6.1

The user explicitly says “我没让你缩减列啊”. Restore the original five-column table and direct metadata. This instruction supersedes8.6.0 grouping/disclosure and all prior density interpretations. Preserve Account, CPA state, Recent authorization, Login network, Actions as independent columns. No disclosure, no hidden diagnostic or inspection content, no max-width narrowing. Limit changes to column proportions,10px cell padding, consistent22px top-line rhythm and compact170px action column. CPA and job states remain independent. Final checks must assert five headings/cells and directly visible user-equivalent missing-diagnostic state.

8.6.1 final:44 account checks/90 screenshots,16 source-matched verification records; fresh independentreview ship at five-column restoration scope. No future density work may reduce columns or conceal these fields without explicit user instruction.


UI 8.8.0: The latest user directly pins toSub2-go-port account display and inline logs. Preserve columns, actual metadata and full values. Use the target table density, six compact real action controls and adjacent bounded log viewport. Do not invent a new compact-list arrangement. The user explicitly removed the expiry column; log history only contains real persisted transitions.

UI 8.8.1: The user's screenshot identifies orphaned reason characters and a centered wrapped execution-mode label. Keep status and stage as unbroken heading phrases, reason beneath, and operation intent/mode as two left-aligned lines. Preserve seven columns and full values.

UI 8.9.0: The user explicitly removes the Login network column and Temporary login feature. Six table columns and three local views remain. Keep account login, task review, inline logs and proxy settings; clean up the obsolete form and references.
