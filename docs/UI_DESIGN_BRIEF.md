# Model Quality console — UI 10.0

Mode: Operate. The administrator uses the CPA iframe on a desktop for model output and account operations, sometimes opening the standalone page or a phone. This redesign is explicitly delegated by the user: “根据你自己的详细了解对当前插件的ui进行整体的布局调整，重新美化”. Layout choices are made within that authorization, preserving the current product behavior and CPA integration.

## Direction contract

THESIS: Each workspace gives the current task a clear working area: configuration beside answers, observation above credential restrictions, account controls above the directory.

OWN-WORLD: CPA-compatible warm light, white and dark surfaces; system type; deep muted teal for primary actions and selected controls; warm neutral dividers and explicit success, caution and failure colors. Border-defined panels, readable 13–14px data, 24px workspace titles, 44px phone controls.

STORY: Choose a workspace, see its purpose and current state, then act within one clearly scoped surface. Full identities, test evidence, manual authorization, historical ownership and unknown results remain legible.

FIRST VIEWPORT: A compact shared navigation bar precedes a visible page heading. Manual testing uses a 340px configuration column beside a broad answer/history surface. Quality starts with its existing four statistics then credential restrictions and historical actions. Accounts use a directory header for actions and a separate filter/search band. Native secret dialogs have fixed headings/actions around a scrolling body. Phones stack in DOM order.

FORM: Task-zoned workspace, candidate 7; seed:96d896e1c5d88447. The user delegated design decisions. Code-led; no image comp. Signature interaction: selecting an account's operation opens its log directly beneath the row while keeping identity visible. Expand reading removes the configuration column without discarding its draft. State changes use 160ms ease-out color and a single short page-entry transition; reduced motion removes movement.

FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Constraints and evidence

Keep six account columns, three login subviews, one-action authorized login, full values, secret clearing, recoverable reads, and independent histories. No external assets, fonts or frontend dependencies. Keep CPA unchanged. Verification uses synthetic fixture data, three themes, desktop, phone and same-origin iframe. Current evidence belongs to artifacts/ui-redesign-0.17.0/. The prior direction and UI source are preserved in its source-before directory.
