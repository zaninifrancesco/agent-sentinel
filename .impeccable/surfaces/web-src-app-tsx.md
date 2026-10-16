---
version: 1
slug: "web-src-app-tsx"
primary_target: "web/src/App.tsx"
related_targets: ["web/src/Analytics.tsx","web/src/TraceMap.tsx","web/src/Timeline.tsx","web/src/Detail.tsx","web/src/index.css"]
---

# Surface brief: Agent Sentinel cockpit

## Scope and mode
Operate. Extend the existing Trace Atlas cockpit with Timeline as the default call-analysis view and Statistics as a separate compact aggregate view; keep passive capture, replay, export and historical trace inspection. This is an ordinary extension of the incumbent visual system, not a new world or token change. Landing page is out of scope.

## Audience and task
Developers inspecting individual agent calls, comparing captured intervals and reading their arguments/results. Sessions provides history; Statistics provides general aggregates. Offline assets, keyboard access, truthful unknowns and mobile timeline/detail navigation are required.

## Direction contract
THESIS: Call analysis is the primary task. Make every call readable and inspectable in the home waterfall; interval geometry never carries the only readable identity.
OWN-WORLD: Trace Atlas, analytical cartography. Graphite navigation rail with hairline borders and outlined active state, cool white layered workspaces, periwinkle selection, teal tool series and amber errors. Archivo UI type, mono only for captured data; precise rounded controls and restrained borders.
STORY: Open Timeline, filter/search calls, compare captured intervals, select a labelled row and read its arguments/results. Use Sessions for history and Statistics for period/agent/project aggregates and session drill-down, preserving live capture.
FIRST VIEWPORT: Timeline first and selected by default; navigation continues Sessions, Statistics, Tool profile and Event log. Recording identity, selected-scope metrics, search and filters precede an integrated labelled waterfall beside the inspector from 1024px. Below 1024px, the full-width waterfall leads to detail with a Back action. No duplicate map/list or height-specific layout. Statistics keeps its daily trend compact and separate; the captured 1516×844 view shows an approximately 198px trend panel.
SIGNATURE: One row per call (56px minimum), fixed readable tool/argument labels (166px; 132px on phone), proportional interval bars and a separate tool-time column (68px; 58px on phone). Shared time axis, zoom and arrow-key selection preserve precise inspection. Point, zero and unknown-timestamp rows stay selectable. Intervals use captured timestamps; available tool duration remains separate. No track packing or separate overview map.
FORM: User selected the assigned fifth grounded direction, Trace Atlas, seed 94272183, code-led. Donor disciplines: dense but readable data; fixed detail positions; evident selection; meaningful inspector space; one job per region; aligned durations.
FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Constraints and unresolved decisions
No native model context or billing data. Retained JSON is stored payload size, not token usage or cost. Statistics combines the local JSONL archive and current recording; empty days do not establish capture coverage. Error rates show known-outcome denominators and duration percentiles show sample counts. Session lists show 25 rows per page; native sessions and connections remain scoped to their recording. A recording may contain several conversations. The waterfall shows a 50-row page, with one lane per call. Snapshot cursors retain access to all captured calls; payloads load for the selected detail. Scope metrics and coverage come from Go and remain independent of list filters. Native usage import, automatic diagnostics and session comparison remain planned. No generated decision comp or raster asset exists for this code-led extension.
