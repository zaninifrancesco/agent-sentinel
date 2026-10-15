---
version: 1
slug: "web-src-app-tsx"
primary_target: "web/src/App.tsx"
related_targets: ["web/src/TraceMap.tsx","web/src/Timeline.tsx","web/src/Detail.tsx","web/src/index.css"]
---

# Surface brief: Agent Sentinel cockpit

## Scope and mode
Operate. Replace the cockpit's visual world; keep passive capture, replay, export and historical trace inspection. Landing page is out of scope.

## Audience and task
Developers debugging individual agent tool calls. Timeline and call evidence lead; aggregate performance is secondary. Offline assets, keyboard access, truthful unknowns and mobile list/detail navigation are required.

## Direction contract
THESIS: A recording is navigable evidence, with time and selected call registered across the screen. Refuse the undifferentiated event table and decorative metric tiles.
OWN-WORLD: Trace Atlas, analytical cartography. Graphite navigation rail with hairline borders and outlined active state, cool white layered workspaces, periwinkle selection, teal tool series and amber errors. Archivo UI type, mono only for captured data; precise rounded controls and restrained borders.
STORY: Locate a call in the recording map, inspect the sequence, then read its arguments and result. Filters expose errors and missing results without inventing diagnoses.
FIRST VIEWPORT: Desktop rail at left; recording header and compact measurement strip above a tool-lane capture map. Below: filtered call list left, generous inspector right. At desktop heights up to 850px, the map sits above the list and the inspector spans the full workspace height. Mobile map and list lead, detail replaces them with a back action. Profile and Event log stay in standard navigation.
SIGNATURE: Selecting a map interval registers that same call in the list and inspector. Intervals use captured request/response timestamps; reported tool duration is a separate value. Selection transitions over 160ms; reduced motion is immediate.
FORM: User selected the assigned fifth grounded direction, Trace Atlas, seed 94272183, code-led. Donor disciplines: dense but readable data; fixed detail positions; evident selection; meaningful inspector space; one job per region; aligned durations.
FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Constraints and unresolved decisions
No native model context or billing data. A recording may contain several conversations. Map shows at most the latest 120 calls and five named tool lanes plus Other; list retains every call. No generated decision comp exists for this code-led choice.
