---
version: 1
slug: "web-src-app-tsx"
primary_target: "web/src/App.tsx"
related_targets: []
---

# Surface brief: Agent Sentinel cockpit (web/src)

## Scope and mode
Operate. One surface: the live cockpit (timeline, detail, approval dock, command palette). Replaces the existing dark "SaaS dashboard + blue accent" look; function, data and keyboard shortcuts are unchanged.

## Audience, task, constraints
A developer with an AI coding agent running, cockpit on a second screen. Jobs: see what the agent is doing, and decide fast on a held call. Must work offline (self-hosted fonts, no CDN), keep statuses pending / awaiting / ok / error / blocked / rejected readable, label cost as estimate.

## Direction contract
THESIS: The session is a timetable and the held call is a train held at the platform. Everything reads in columns (time, tool, rule, duration) and one clock tells how long the human has left. It refuses the dark-dashboard-with-neon-accent default.
OWN-WORLD: Swiss station timetable. Warm white paper ground, near-black ink, timetable yellow for the selected or held row, signal red reserved for what needs the human or was refused. Archivo (variable, narrow numerals for the time column), tabular figures everywhere, hairline rules, square corners, no shadows except one soft one under overlays. Light theme.
STORY: Glance: nothing is red, the agent is running. A call stops: the row turns yellow, every other row steps back, a station clock starts its red second hand over the approval window. The developer reads the rule and the quoted trigger, approves or rejects, the table resumes.
FIRST VIEWPORT: Header 44px (wordmark, session, connection, budget as a plain line with a thin rule gauge). Left 5/12: timetable with sticky column header and rows. Right 7/12: selected call as a journey (Request, Policy, Human, Response as stops on a vertical line) above tabbed payload. Bottom dock (only when held): yellow bar with the station clock left, tool + rule + trigger centre, Reject and Approve right.
SIGNATURE: the station clock in the dock. The red hand sweeps one revolution over the approval timeout (from /api/config) and the remaining time is also written as text; at zero the call is refused by the server.
FORM: Fahrplan, assigned index 4 of my list, challenger-donated raises: DIM-THE-REST (streaming wall) and ONE RESERVED COLOUR (orienteering). Seed key f5401336.
FINISH: unreviewed and undocumented is unfinished; this build ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance

## Unresolved
Dark variant (not built). Export modal restyled only, feature on hold.
