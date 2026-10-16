---
version: 1
slug: "web-src-app-tsx"
primary_target: "web/src/App.tsx"
related_targets: ["web/src/Analytics.tsx","web/src/TraceMap.tsx","web/src/Timeline.tsx","web/src/Detail.tsx","web/src/index.css"]
---

# Surface brief: Agent Sentinel cockpit

## Scope and mode
Operate. Extend the existing Trace Atlas cockpit with analytics as the default view; keep passive capture, replay, export and historical trace inspection. This is an ordinary extension of the incumbent visual system, not a new world or token change. Landing page is out of scope.

## Audience and task
Developers monitoring observed agent activity across recordings, projects and time periods, then investigating the sessions and calls behind those measures. Analytics leads; the timeline and inspector supply call evidence. Offline assets, keyboard access, truthful unknowns and mobile list/detail navigation are required.

## Direction contract
THESIS: Monitor observed agent usage across recordings and reach the calls behind each metric. Within a recording, time and selected call stay registered across the map, list and inspector. Refuse the undifferentiated event table and decorative metric tiles.
OWN-WORLD: Trace Atlas, analytical cartography. Graphite navigation rail with hairline borders and outlined active state, cool white layered workspaces, periwinkle selection, teal tool series and amber errors. Archivo UI type, mono only for captured data; precise rounded controls and restrained borders.
STORY: Choose period, agent and project; inspect the daily trend and breakdowns; open a historical session; then locate a call in the recording map and read its arguments and result. Daily selection narrows the session list. Opening a session shows its captured scope while live capture continues. Filters expose reported errors and missing results without inventing diagnoses.
FIRST VIEWPORT: Desktop rail at left with Analytics first and selected by default, followed by Sessions, Timeline, Tool profile and Event log. The analytics workspace starts with period/agent/project filters and UTC period metadata, then measures with explicit denominators and sample counts, followed by the daily calls/errors chart. Breakdowns and sessions continue below. Phone filters and measures wrap into columns; breakdowns stack and wide session tables scroll within their container. After session drill-down, the recording header and compact measurement strip sit above the paged call timeline. At desktop heights up to 850px, the map sits above the list and the inspector spans the full workspace height. Mobile detail replaces the map and list with a back action.
SIGNATURE: Sequential calls reuse tracks; captured overlaps add tracks. Tool names, colours and zoom keep individual calls legible. Selecting a map interval registers that same call in the list and inspector. Intervals use captured request/response timestamps; reported tool duration is a separate value. Selection transitions over 160ms; reduced motion is immediate.
FORM: User selected the assigned fifth grounded direction, Trace Atlas, seed 94272183, code-led. Donor disciplines: dense but readable data; fixed detail positions; evident selection; meaningful inspector space; one job per region; aligned durations.
FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Constraints and unresolved decisions
No native model context or billing data. Retained JSON is stored payload size, not token usage or cost. Analytics combines the local JSONL archive and current recording; empty days do not establish capture coverage. Error rates show known-outcome denominators and duration percentiles show sample counts. Session lists show 25 rows per page; native sessions and connections remain scoped to their recording. A recording may contain several conversations. Waterfall and list show a 50-row page, with one lane per call. Snapshot cursors retain access to all captured calls; payloads load for the selected detail. Scope metrics and coverage come from Go and remain independent of list filters. Native usage import, automatic diagnostics and session comparison remain planned. No generated decision comp or raster asset exists for this code-led extension.
