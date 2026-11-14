---
name: "Model Quality"
description: "Precise, warm-neutral working surfaces for comparing model evidence and managing credential restrictions."
colors:
  page: "#faf9f5"
  surface: "#fffdf9"
  surface-raised: "#ffffff"
  surface-muted: "#f0eee8"
  hover: "#e9e6df"
  ink: "#2d2a26"
  muted: "#68625b"
  border: "#e3e1db"
  border-strong: "#b5afa7"
  accent: "#726d67"
  accent-hover: "#5f5a54"
  on-accent: "#ffffff"
  focus: "#8a3a30"
  good: "#065f46"
  good-bg: "#e2f3e9"
  warn: "#92400e"
  warn-bg: "#fcf0d8"
  bad: "#8a3a30"
  bad-bg: "#f9e8e3"
  white-page: "#ffffff"
  white-surface: "#ffffff"
  white-surface-raised: "#ffffff"
  white-surface-muted: "#f6f6f6"
  white-hover: "#eeeeee"
  white-border: "#e5e5e5"
  dark-page: "#151412"
  dark-surface: "#1d1b18"
  dark-surface-raised: "#24211d"
  dark-surface-muted: "#262320"
  dark-hover: "#2e2a26"
  dark-ink: "#f6f4f1"
  dark-muted: "#c9c3bb"
  dark-border: "#3a3530"
  dark-border-strong: "#777067"
  dark-accent: "#c9c3bb"
  dark-accent-hover: "#e0d9d0"
  dark-on-accent: "#1d1b18"
  dark-focus: "#f1b0a6"
  dark-good: "#6ee7b7"
  dark-good-bg: "#143b2f"
  dark-warn: "#fcd34d"
  dark-warn-bg: "#3d301b"
  dark-bad: "#f1b0a6"
  dark-bad-bg: "#442820"
typography:
  headline:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "26px"
    fontWeight: 650
    lineHeight: 1.3
    letterSpacing: "-0.02em"
  headline-mobile:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "23px"
    fontWeight: 650
    lineHeight: 1.3
    letterSpacing: "-0.02em"
  title:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "18px"
    fontWeight: 650
    lineHeight: 1.5
  section:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "16px"
    fontWeight: 600
    lineHeight: 1.5
  body:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.6
  label:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "14px"
    fontWeight: 550
    lineHeight: 1.6
  supporting:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "12px"
    fontWeight: 400
    lineHeight: 1.7
  status:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "11px"
    fontWeight: 550
    lineHeight: 1.6
  answer:
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace"
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.8
  code:
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, Consolas, monospace"
    fontSize: "12px"
    fontWeight: 400
    lineHeight: 1.6
  metric:
    fontFamily: "-apple-system, BlinkMacSystemFont, \"Segoe UI\", \"PingFang SC\", \"Microsoft YaHei\", sans-serif"
    fontSize: "24px"
    fontWeight: 600
    lineHeight: 1.4
rounded:
  tag: "4px"
  control: "6px"
  surface: "8px"
  dialog: "12px"
spacing:
  tight: "4px"
  status-gap: "6px"
  compact: "8px"
  small: "12px"
  medium: "16px"
  field: "18px"
  inset: "20px"
  section: "24px"
  panel: "28px"
  wide: "32px"
  large: "36px"
  workspace: "48px"
  empty: "64px"
components:
  button-primary:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.on-accent}"
    rounded: "{rounded.surface}"
    padding: "8px 15px"
  button-primary-hover:
    backgroundColor: "{colors.accent-hover}"
  button-subtle:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.surface}"
    padding: "8px 15px"
  button-danger:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.bad}"
    rounded: "{rounded.surface}"
    padding: "8px 15px"
  button-text:
    textColor: "{colors.muted}"
    rounded: "{rounded.surface}"
    padding: "7px 9px"
  input:
    backgroundColor: "{colors.surface-raised}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "10px 12px"
  navigation:
    textColor: "{colors.muted}"
    padding: "12px 2px"
  model-chip:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "7px 10px"
  status-tag:
    backgroundColor: "{colors.surface-muted}"
    textColor: "{colors.muted}"
    rounded: "{rounded.tag}"
    padding: "3px 8px"
    typography: "{typography.status}"
  result-card:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.surface}"
  disclosure:
    textColor: "{colors.ink}"
    padding: "16px 0"
  dialog:
    backgroundColor: "{colors.surface}"
    textColor: "{colors.ink}"
    rounded: "{rounded.dialog}"
    padding: "28px"
---

# Design System: Model Quality

## Overview

**Creative North Star: "The Evidence Workbench"**

Model Quality uses a quiet, exact working language: warm neutrals establish the workspace, white or raised dark fields carry answers, and clear separators keep related facts aligned. Compact system typography leaves room for model output and operational evidence. The personality is deliberate and restrained, with emphasis earned by the next action or a named state.

This system inherits the color families of CPA Management Center while independently defining its navigation, composition, and components. Native controls and disclosures keep familiar behavior. Broad sections stay flat; modal depth is reserved for focused inspection or consequential confirmation. The product mark is inline vector artwork; no raster asset is shipped.

**Key Characteristics:**

- Warm light, white, and warm dark themes with semantic color roles.
- Compact system sans for interface text, monospace for answers and source.
- Fine separators and restrained rounded corners instead of elevated card grids.
- Persistent navigation, scoped disclosures, and explicit labels for independent states.
- Alignment for comparison, with readable single-column reflow on narrow screens.

This records the finished implementation in `plugin/ui/index.html`, `style.css`, `app.js`, and `view.js`. Surface-specific strategy and mode remain in `.impeccable/surfaces/plugin-ui-index-html.md`; product semantics remain in `PRODUCT.md`.

## Colors

The palette is warm and low-chroma, with a neutral primary action and restrained semantic green, amber, and red. Frontmatter values are normative. Unprefixed color tokens are the light theme; `white-` and `dark-` entries are exact overrides of the matching base role. Roles without a white override inherit the light value. The UI starts in light and cycles light → dark → white; it does not persist a theme preference.

### Primary

- **Warm Stone Action** (`accent`, `accent-hover`, `on-accent`): filled primary actions, with warm dark text on the light-toned dark-theme button.
- **Clay Focus** (`focus`): keyboard location, independent of the action fill.

### Semantic states

- **Deep Green Evidence** (`good`, `good-bg`): named passing or positive states.
- **Amber Attention** (`warn`, `warn-bg`): named warning, incomplete evidence, and stale-data states.
- **Clay Error** (`bad`, `bad-bg`): named failures, inline errors, and the outlined consequential-action button.

### Neutral

- **Warm Paper** (`page`), **Working Paper** (`surface`), and **Answer White** (`surface-raised`): the page, grouped content, and control/answer fields.
- **Quiet Inset** (`surface-muted`) and **Hover Wash** (`hover`): contextual grouping and interaction feedback.
- **Warm Ink** (`ink`) and **Readable Secondary** (`muted`): primary text and supporting text. The final secondary value is the contrast adaptation in the frontmatter, not the earlier design draft.
- **Fine Divider** (`border`) and **Control Edge** (`border-strong`): content separation and actionable field boundaries.

CPA palette provenance is `router-for-me/Cli-Proxy-API-Management-Center`, fixed commit `d0126752201a35ff999bd2a1d4741215542258a8`, `src/styles/themes.scss`. `docs/UI_CN.md` records the source comparison; `plugin/NOTICE.md` retains attribution and the MIT license. Selected shades are adapted for this UI’s contrast. The static local-page comparison supports palette consistency, not a claim about the deployed frontend’s exact commit.

**The Semantic State Rule.** Pair every success, warning, and error color with a named state. Execution, answer verdict, comparability, and credential restrictions remain separately legible.

**The Contrast Adaptation Rule.** Preserve the contrast-adjusted secondary text and neutral action shades when adapting the CPA palette. Theme substitutions must preserve the foreground/background pair, not merely the hue.

The final contrast artifact checks thirteen body-text pairs with a minimum ratio of (4.83:1); it is evidence for those pairs, not a claim of formal accessibility certification. Sidecar tonal ramps are generated swatch previews only, not additional production theme tokens.

## Typography

Use the local system sans stack from the frontmatter for interface text, including Chinese fallback faces. No external font is loaded. Use the local monospace stack for source and answers. There is no separate display face or display role.

- **Headline:** the page title; the mobile override reduces its size without changing its weight or tracking.
- **Title / section:** nested section and dialog headings; model headings use a slightly smaller local size (15px).
- **Body / label:** default interface text and field labels; labels are medium weight while their control values return to regular.
- **Supporting / status:** metadata and compact named-state tags. Explanatory paragraphs have a maximum measure (78ch) and line height (1.7).
- **Answer / code:** wrapped text that preserves original line breaks; arbitrary long strings wrap rather than widening the page.
- **Metric:** compact numerical emphasis with tabular numerals. It remains subordinate to the page title.

**The Reading Hierarchy Rule.** Use the compact heading scale for interface hierarchy. Reserve monospace for answer text, source, and raw evidence; use tabular numerals for metric values and navigation counts.

## Layout

The desktop shell has a persistent top navigation (72px minimum height), a slim freshness row (56px minimum), and a centered workspace (1440px maximum width) with side gutters (48px). Page headings, local view navigation, and section boundaries establish the reading order. Spacing uses the extracted frontmatter scale with deliberate larger gaps between tasks; it is not a uniform card grid.

The current console has two primary destinations. Manual evaluation exposes setup, current task, and history as local views. Quality operations exposes attention, rules, and audit; the attention view places owned pauses and credential restrictions before metrics. Rule and global-budget editors replace the operations content in place. These are current component compositions, not mandatory layouts for unrelated future surfaces.

Setup uses two columns (1fr / 1.18fr) separated by a fine rule, with a wider gap (48px). Execution parameters use a narrow label column (220px) and a flexible control column. Ordinary field rows fit columns with a minimum width (150px). The inline submit summary keeps scope and the explicit execution action together.

The answer comparison has at most two visible model columns. CSS row subgrid aligns each model header and same-numbered sample block; the application updates the row span for the visible sample count. Individual samples hold named execution/verdict/comparability states, answer text, evidence actions, and a review disclosure. A single selected model fills the width. Frozen conditions remain directly above the comparison in a native disclosure.

| Viewport | Implemented adaptation |
| --- | --- |
| Above 1100px | Full workspace gutters, unequal setup columns, and a comparison gap (28px). |
| At or below 1100px | Gutters and header inset become (28px); setup columns become equal with a gap (24px); the execution label column becomes (160px); comparison controls wrap; rule tables retain a minimum width (880px) inside a horizontal scroll region. |
| At or below 760px | Gutters become (20px); the header wraps and primary navigation remains visible on a full-width row (48px high); setup and editor sections stack. Comparison changes to a single column, model containers become ordinary block flow, and subgrid row sharing stops. Model selectors retain two compact columns above the stacked answers. |

On mobile, the crumb is hidden while freshness and refresh remain visible. Credential and rule tables become stacked entries with each cell’s field name exposed via `data-label`. Metrics become two columns; budget fields also remain two columns. Dialog padding reduces to the inset step and their available height increases. Release confirmation spans the available button-row width. Supporting facts and review fields retain their compact two-column layouts where implemented.

**The Alignment Before Ornament Rule.** Give comparable facts shared columns and boundaries. At the mobile breakpoint, release shared row alignment and preserve each model’s reading order.

## Elevation & Depth

Ordinary work is flat: page and surface tones, one-pixel dividers, and white or raised-dark answer fields supply depth. Native dialogs alone use the soft shadow recorded in the sidecar, with a warm-black translucent backdrop. The dark theme deepens the shadow without changing its geometry. There are no decorative gradients, textured raster grounds, or entrance effects in the shipped interface.

**The Flat Workspace Rule.** Keep ordinary sections and result containers free of shadows. Use the single soft shadow and dim backdrop for native dialogs.

Button colors and borders transition over (180ms). Task progress width transitions over (220ms) using the sidecar easing. Motion only reports interaction or progress. A reduced-motion media query disables transitions and animation and restores automatic scroll behavior.

## Shapes

Use the frontmatter radius roles: compact tags, gently rounded controls, standard working surfaces, and slightly softer dialogs. Rectangular navigation has a two-pixel active underline; content divisions and input edges use one-pixel borders. The connection indicator is a small circular dot (7px) paired with readable connection text. The inline brand mark is a stroked SVG (28px), not a font glyph or raster image.

## Components

### Buttons

Firm, compact, and text-led. The primary variant uses the neutral action pair; the subtle variant uses a surface and stronger border; the danger variant uses an error-colored outline and text; the text variant uses secondary text without a visible border. Standard controls have a minimum height (40px), while compact buttons use (36px) and smaller type (12px). Hover changes fill or border, and active feedback uses the divider tone where the variant cascade permits it. Disabled buttons use reduced opacity (0.48) and a not-allowed cursor; busy buttons use a progress cursor and opacity (0.7).

### Inputs / Fields

Native input, select, textarea, and checkbox behavior is retained. Fields use the raised surface, strong border, control radius, and a minimum height (44px). Visible labels precede fields; help text follows or accompanies the label. Hover strengthens the border to secondary ink. Disabled inputs and read-only textareas use the muted surface; textareas resize vertically. Checkboxes remain native (18px) with the neutral accent. Errors appear as readable feedback near the active surface rather than color-only invalid borders.

### Navigation

Both the primary destination navigation and local view navigation use quiet text with an ink underline for the active item. Hover feedback and visible keyboard focus remain distinct from selection. Buttons are real buttons; the implementation also updates `aria-current` for the active views. The mobile header wraps while preserving direct access to both primary destinations.

### Chips and named-state tags

Selectable model chips are compact bordered buttons, wrap long model identifiers, and expose selection through `aria-pressed`. Selected chips use a muted fill and stronger edge. Noninteractive state tags are smaller rounded labels using neutral, good, warning, or bad pairs. State labels carry meaning independently of color.

### Containers and aligned answers

Result containers use a fine border, surface fill, and standard radius with a muted model header. Same-numbered samples are separated by a stronger divider. Answers are raised, bordered monospace fields with a minimum height (150px), a desktop maximum height (420px), and a mobile maximum (360px); explicit expansion removes the maximum. Scroll stays within long answers. The enclosing comparison subgrid aligns whole samples, not every internal line of text.

Empty states use a dashed boundary and centered guidance; activity and history use separated rows. Owned-pause rows place the identity/evidence summary beside the decision area until mobile reflow. These containers remain flat.

### Disclosures and editors

Use native `details` / `summary` for presets, advanced parameters, frozen conditions, human review, statistical definitions, and secondary evidence. Summaries have a minimum target height (44px), with compact rule-condition summaries locally reduced. Keep raw JSON secondary to structured evidence. Rule and budget editing are inline workspace surfaces, with focus moved to the heading and returned to the relevant opener when closed.

### Protected native dialogs

Connection, sample evidence, manual release, and isolated preview use native `dialog` with `showModal()`, readable headings, explicit close actions, native modal keyboard behavior, and scrolling within the viewport. Default width is capped at (560px); evidence, release, and preview caps are (840px), (720px), and (1100px), each with a viewport inset (32px).

The release dialog shows credential identity, ownership, action version, and the consequence that dependent rules stay stopped. Confirmation is disabled while its preview is unavailable, stale, conflicting, or unresolved. A conflicting or uncertain response exposes refresh/reconciliation feedback instead of a repeat-submit promise. The isolated generated-content preview is a noninteractive sandbox; the original escaped output remains available in the answer field.

**The Visible Focus Rule.** Keep the two-pixel focus outline with its three-pixel offset on keyboard targets, including controls, navigation, and disclosure summaries.

**The Bounded Confirmation Rule.** Keep release identity, ownership, version, consequences, and reconciliation feedback inside the release dialog. An uncertain or conflicting result must remain visibly unresolved until refreshed.

The skip link targets the main workspace. Status feedback uses polite live announcements. Native control semantics, labels, keyboard focus, reduced motion, and the responsive structure are implemented capabilities; no formal accessibility certification is asserted.

## Do's and Don'ts

### Do:

- Do use the semantic CSS variables so all three themes retain their intended contrast.
- Do show precise state text beside colored tags and keep unknown evidence explicit.
- Do preserve native controls, visible focus, the skip link, and status announcements.
- Do use separators, spacing, and shared alignment to organize dense information.
- Do check desktop comparison, tablet tables, and mobile field labels when extending a component.
- Do disable transitions and animation when reduced motion is requested.

### Don't:

- Don’t turn secondary text back into the superseded lighter gray.
- Don’t replace independent execution, verdict, and restriction labels with one colored success indicator.
- Don’t hide primary navigation behind a mobile drawer.
- Don’t treat a rule editor as a modal; the implemented editing surface stays in the workspace.
- Don’t give ordinary content cards the dialog shadow.
- Don’t display model-generated HTML as interactive parent-page content.
