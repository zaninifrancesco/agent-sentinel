---
name: "Agent Sentinel — Trace Atlas"
description: "A local recording cockpit shaped by analytical cartography."
colors:
  paper: "#f3f6fb"
  sheet: "#ffffff"
  rule: "#e3e8f1"
  rule-strong: "#cbd4e3"
  ink: "#202d45"
  ink-2: "#58677f"
  ink-3: "#65758d"
  accent: "#4f5fc7"
  yellow: "#5566db"
  yellow-soft: "#edf0ff"
  signal: "#bd4356"
  signal-soft: "#fceef1"
  ok: "#197e70"
  ok-soft: "#e8f5f1"
  warn: "#996019"
  rail: "#202123"
  rail-rule: "#393a3c"
  rail-muted: "#b2b3b6"
  rail-active: "#303133"
  rail-active-rule: "#48494c"
  rail-hover: "#292a2c"
  series-0: "#6574d9"
  series-1: "#258c83"
  series-2: "#9a6bb1"
  series-3: "#5088b5"
  series-4: "#947747"
  code-key: "#4f5fc7"
  code-string: "#19776b"
  code-number: "#a0652f"
  overlay: "#101c35a6"
  dark-paper: "#101827"
  dark-sheet: "#172234"
  dark-rule: "#29364a"
  dark-rule-strong: "#40506a"
  dark-ink: "#e3eaf7"
  dark-ink-2: "#a9b8cf"
  dark-ink-3: "#91a3be"
  dark-accent: "#a1acff"
  dark-yellow: "#7e8dea"
  dark-yellow-soft: "#29365a"
  dark-signal: "#f18b9e"
  dark-signal-soft: "#422938"
  dark-ok: "#6acbb6"
  dark-ok-soft: "#173e3b"
  dark-warn: "#e9b573"
  dark-series-0: "#8997ed"
  dark-series-1: "#50b8aa"
  dark-series-2: "#bb8dd0"
  dark-series-3: "#70a5ce"
  dark-series-4: "#bc9d65"
  dark-code-key: "#a1acff"
  dark-code-string: "#6acbb6"
  dark-code-number: "#e9b573"
typography:
  headline:
    fontFamily: "\"Archivo Variable\", ui-sans-serif, system-ui, -apple-system, \"Segoe UI\", sans-serif"
    fontSize: "21px"
    fontWeight: 650
    lineHeight: 1.25
    letterSpacing: "-0.025em"
  section:
    fontFamily: "\"Archivo Variable\", ui-sans-serif, system-ui, -apple-system, \"Segoe UI\", sans-serif"
    fontSize: "18px"
    fontWeight: 600
    lineHeight: "28px"
    letterSpacing: "-0.025em"
  title:
    fontFamily: "\"Archivo Variable\", ui-sans-serif, system-ui, -apple-system, \"Segoe UI\", sans-serif"
    fontSize: "16px"
    fontWeight: 600
    lineHeight: "24px"
  body:
    fontFamily: "\"Archivo Variable\", ui-sans-serif, system-ui, -apple-system, \"Segoe UI\", sans-serif"
    fontSize: "13px"
    fontWeight: 400
    lineHeight: 1.5
  label:
    fontFamily: "\"Archivo Variable\", ui-sans-serif, system-ui, -apple-system, \"Segoe UI\", sans-serif"
    fontSize: "12px"
    fontWeight: 500
    lineHeight: "16px"
  micro:
    fontFamily: "\"Archivo Variable\", ui-sans-serif, system-ui, -apple-system, \"Segoe UI\", sans-serif"
    fontSize: "11px"
    fontWeight: 400
    lineHeight: 1.5
  code:
    fontFamily: "ui-monospace, \"SF Mono\", Menlo, Consolas, monospace"
    fontSize: "11px"
    fontWeight: 400
    lineHeight: 1.9
rounded:
  span: "3px"
  badge: "5px"
  control: "6px"
  navigation: "7px"
  payload: "8px"
  panel: "12px"
spacing:
  tight: "4px"
  inline: "8px"
  compact: "12px"
  gutter: "16px"
  panel: "18px"
  workspace: "20px"
  header: "24px"
components:
  button-export:
    backgroundColor: "{colors.accent}"
    textColor: "{colors.sheet}"
    rounded: "{rounded.control}"
    padding: "8px 16px"
    typography: "{typography.label}"
  button-action:
    backgroundColor: "{colors.sheet}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "8px 12px"
    height: "36px"
    typography: "{typography.label}"
  button-icon:
    textColor: "{colors.ink-2}"
    rounded: "{rounded.control}"
    size: "32px"
  field-input:
    backgroundColor: "{colors.sheet}"
    textColor: "{colors.ink}"
    rounded: "{rounded.control}"
    padding: "8px 12px"
    height: "36px"
    typography: "{typography.label}"
  navigation-active:
    backgroundColor: "{colors.rail-active}"
    textColor: "#ffffff"
    rounded: "{rounded.navigation}"
    padding: "9px 11px"
    height: "40px"
  filter-selected:
    backgroundColor: "{colors.yellow-soft}"
    textColor: "{colors.accent}"
    rounded: "{rounded.badge}"
    padding: "5px 9px"
  profile-panel:
    backgroundColor: "{colors.sheet}"
    textColor: "{colors.ink}"
    rounded: "{rounded.panel}"
    padding: "18px"
  map-interval:
    rounded: "{rounded.span}"
    height: "18px"
  payload-section:
    backgroundColor: "{colors.sheet}"
    textColor: "{colors.ink}"
    rounded: "{rounded.payload}"
    typography: "{typography.code}"
---

# Design System: Agent Sentinel — Trace Atlas

## Overview

**Creative North Star: "Trace Atlas"**

Trace Atlas treats a recording as navigable evidence. A graphite navigation rail with hairline boundaries anchors a cool, layered workspace; compact measurements and aligned rows make dense captured data readable. Archivo carries the interface, with monospace reserved chiefly for captured arguments, identifiers and JSON.

Analytical cartography supplies the signature: one labelled waterfall row combines each call’s identity and captured interval, registered with the inspector. Tool series remain consistent across these regions. Borders and tonal changes give each region a clear job while the inspector preserves room for the evidence.

**Key Characteristics:**
- Cool paper, white panels and a stable graphite navigation rail with a fine right-hand boundary.
- Periwinkle selection shared across waterfall intervals, rows and filters.
- Tool series identify categories; amber and explicit status marks identify reported errors.
- Compact tabular measurements, readable captured payloads and restrained rounded geometry.

## Colors

Cool white and slate neutrals support a graphite frame, periwinkle selection and categorical tool colours. Frontmatter records the light and dark values extracted from the stylesheet; `dark-` entries are theme counterparts, not extra accents.

### Primary
- **Periwinkle Accent** (`accent`): active inspector tabs, focus outlines, selected waterfall interval outline.
- **Selection Periwinkle** (`yellow`, `yellow-soft`): the source retains legacy variable names; these now represent indigo and pale periwinkle. The soft surface marks selected rows, filters and search matches.

### Secondary
- **Tool Series** (`series-0` through `series-4`): indigo, teal, purple, blue and muted ochre. Shell uses teal, read_file indigo, edit_file purple and Grep blue; other names map deterministically into the same series. These are category colours, not a sequential scale.
- **Success Teal** (`ok`, `ok-soft`): successful status marks and positive historical data.

### Tertiary
- **Error Amber** (`warn`): reported tool errors, error counts and error intervals on the map.
- **Historical Rose** (`signal`, `signal-soft`): recorded blocked/rejected states, historical risk and selected protocol warnings. Rose is not a prohibition on the cartographic series.
- **Syntax Colours** (`code-key`, `code-string`, `code-number`): distinguish JSON keys, strings and scalar values without interpreting captured text as markup.

### Neutral
- **Cool Paper** (`paper`): workspace ground, payload labels and recessed measurement bands.
- **Sheet** (`sheet`): inspector, controls and panels; becomes a slate surface in dark mode.
- **Slate Rules** (`rule`, `rule-strong`): panel divisions and stronger field/table boundaries.
- **Ink** (`ink`, `ink-2`, `ink-3`): primary data, supporting labels and tertiary context.
- **Graphite Rail** (`rail`, `rail-rule`, `rail-muted`, `rail-active`, `rail-active-rule`, `rail-hover`): neutral navigation frame with fine outlined selection, shared across both themes.
- **Overlay Navy** (`overlay`): translucent dimming behind search and export dialogs.

### Named Rules

**The Registered Selection Rule.** A selected call uses the same identity in the labelled waterfall row and inspector; periwinkle communicates selection rather than tool category.

## Typography

**Display Font:** no separate display face or oversized display role is used.
**Body Font:** self-hosted Archivo Variable with the source’s sans-serif fallback stack.
**Label/Mono Font:** Archivo for interface labels; the platform monospace stack for captured arguments, identifiers and JSON.

**Character:** compact, precise and calm. Weight distinguishes tool names and measurements; tabular figures keep changing numeric values aligned.

### Hierarchy
- **Headline:** workspace headings (650, 21px, 1.25 line height, −0.025em); mobile uses 18px.
- **Section:** profile introduction (600, 18px, 28px line height, tight tracking).
- **Title:** selected tool and profile panel headings (600, 16px, 24px line height).
- **Body:** default interface (400, 13px, 1.5 line height); profile explanatory text also uses 14px.
- **Label:** buttons and inspector tabs (500, 12px, 16px line height); export’s filled action is semibold.
- **Micro:** map lane names and supporting metadata (11px); dense column labels, field names and argument summaries use 10px. Preserve those roles rather than shrinking primary content.
- **Code:** payload JSON (400, 11px, 1.9 line height), with wrapping and two-space tab width. Captured argument summaries are 10–12px according to context.
- **Figures:** tabular numerals throughout; measurement-strip values are 14px at weight 650 and reduce to 12px on mobile.

### Named Rules

**The Separate Measures Rule.** Captured request-to-response intervals and source-reported tool duration are distinct measurements. Keep their labels and visual positions distinct; unavailable values remain unavailable.

## Layout

The shell fills the dynamic viewport height. The desktop rail is 218px, expands to 226px from 1536px, and narrows to 188px below 1024px; its 68px collapsed state shows icon navigation with tooltips, and the preference persists locally. A white recording header and compact measurement strip sit above the workspace. Common gutters are 16–24px, with 8–12px gaps inside controls and metadata groups; panel spacing uses the extracted 18–20px steps rather than a single invented grid unit.

The call inspector splits an integrated labelled waterfall from a detail column with a 340px minimum and a 1.2 : 0.8 proportion. From 1536px, detail has a 440px minimum and a 1 : 0.8 proportion. Search and status filters span the workspace above the split. The waterfall fills the remaining pane height; there is no separate top map or duplicate call list, and no height-specific layout.

Below 1024px, opening detail replaces the waterfall, with an explicit back action. Below 768px, the rail becomes a top navigation strip; recording identity moves out of the dense header, the heading reduces in size and panel margins become 12px. Tool profile uses a two-column 3 : 2 layout from 1280px, with a 1600px maximum content width.

Waterfall and inspector payload regions scroll independently. The waterfall has one row per call with a sticky label column (166px; 132px below 768px), a proportional interval region and a sticky tool-time column (68px; 58px below 768px). Headers, selection metadata, inspector tabs and payload search stay outside the payload scroller. Search and export use bounded, internally scrollable dialogs with contained focus and focus restoration.

## Elevation & Depth

The workspace is flat and layered by tone, hairline borders and containment. Panels do not cast ambient shadows. Dialogs dim the workspace with translucent navy and have no backdrop blur. Selected waterfall rows use a pale-periwinkle surface and their intervals receive a 1px accent outline offset by 2px. Event-log selected rows use an inset bottom rule.

### Shadow Vocabulary
- **Selected row rule:** `inset 0 -1px var(--color-rule)`.

### Named Rules

**The Bordered Surface Rule.** Use cool tonal steps and hairline borders for panel hierarchy. Reserve rings for selection and focus, rather than adding floating panel shadows.

## Shapes

Rounded rectangles structure the workspace: panel corners are 12px, payload blocks 8px, navigation items 7px, controls 6px and compact filters 5px. Small map intervals use 3px corners and 6px series dots use a 2px radius. These are precise containers, not pill-shaped cards. Open waterfall intervals use a hollow dashed rectangular point marker; circles also serve status dots and historical journey stops. SVG icons carry interface actions and status, while native HTML/CSS geometry carries the map.

## Components

### Buttons
Restrained controls with explicit purpose. Outlined actions use sheet fill, stronger rule border, 12px medium labels, 8px × 12px padding and a 36px minimum height. Export’s filled action uses accent fill, sheet text, semibold labels and 8px × 16px padding; its hover lowers opacity to 90%. Icon actions are 32px square and move from secondary ink to ink on a paper hover surface. Disabled controls use 45% opacity. Global focus is a 2px accent outline offset by 3px.

### Chips and filters
Status filters have 5px corners, 5px × 9px padding and 11px labels. Selected filters use pale periwinkle, accent text, a rule border and semibold weight. Historical risk chips remain available only with historical data: filled rose for critical, rose outline for high and neutral outlines for medium/low. Their uppercase treatment labels recorded risk; it does not establish a general heading style.

### Cards / Containers
Inspector workspace and profile panels use sheet fill, a rule border and 12px corners. The waterfall is integrated into the workspace rather than a separately bordered map panel. Profile panels have 18px padding, reducing to 14px on mobile. Payload blocks use an 8px outline container, cool label strip and 12px × 14px code padding. Panel hierarchy comes from boundaries and interior rhythm rather than shadow.

### Inputs / Fields
Search fields and selects use sheet fill, a stronger rule border, 6px corners and 12px text, with a 36px minimum height. The call search adds a leading SVG search icon. Payload search uses a compact rule outline and reports match counts. Accent caret and global visible focus connect fields to the selection system.

### Navigation
The graphite rail contains 40px minimum-height links with an 8px radius and 12px labels. Active links use a charcoal fill and fine outline; hover uses a slightly lighter neutral. Small line SVG icons accompany the text, and Timeline carries a compact count badge. The header control collapses the desktop rail to a 68px icon strip and remembers the preference. On mobile, the expanded links form a horizontal strip; the collapse control folds them away. Inspector tabs use a 2px accent underline; inactive tabs use secondary ink.

### Call timeline (signature)
Captured calls share one horizontal time axis, with one selectable row per call (56px minimum). Fixed labels keep the tool name and monospace argument preview readable independently of interval width; tool duration occupies a separate column. Captured intervals use proportional 18px-high bars with 3px corners. Zero-length intervals use 8px point markers; unknown ends remain hollow dashed markers, and missing timestamps show “Time unavailable” in a selectable row. Tool colours match the profile. Error bars have a dashed amber border. Selection colours the full row and outlines its interval, opening that same call in the inspector. Calls never share packed tracks.

Fit shows the current page window; 4×, 16× and 64× zoom preserve that time scale inside the scrolling canvas, with labels and tool time sticky at either edge. Arrow keys move through page rows and reveal the selected interval. A grey strip indicates unattributed time. Matched coverage and peak concurrency describe the whole selected scope, independently of filters and page. Reported tool duration remains separate from captured timing. Paging continues to pin the recording revision.

### Inspector and event list
The Event log retains ruled list columns and focused-list J/K and arrow navigation. The call waterfall uses all four arrow keys for row selection. Status marks combine SVG icons with accessible text; tool names are semibold and argument previews are monospace.

The inspector keeps tool name, captured time, reported duration, response JSON size and status in fixed positions. A compact text row labels the observed interval. Tool and argument summary share a header row; metrics stay in a small responsive grid. Arguments/result lead, with Patch only when actual patch data exists and Event JSON secondary. Safe text-node syntax colouring and match highlighting preserve readable captured data. Capture limitations and historical metadata use disclosure sections.

## Do's and Don'ts

### Do:
- **Do** keep each tool’s series colour consistent across the waterfall and profile.
- **Do** pair status colour with a readable label or SVG status icon.
- **Do** align times, counts and sizes with tabular figures.
- **Do** keep the selected call and its captured payload readable at laptop and mobile sizes.
- **Do** retain visible focus and remove transitions under reduced motion.
- **Do** label retained JSON size and reported duration precisely.

### Don't:
- **Don't** use tool-series colour as the only indication of an error or selection.
- **Don't** describe captured intervals as model thinking time, or retained JSON as original output or context usage.
- **Don't** replace unavailable measurements with a measured zero.
- **Don't** add decorative metric tiles, imagery or panel shadows to this evidence-led cockpit.
- **Don't** turn historical policy metadata into live approval controls.
