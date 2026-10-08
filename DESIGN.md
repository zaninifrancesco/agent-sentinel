---
name: Agent Sentinel Cockpit
description: A live timetable of an AI agent's actions, with one red hand that counts down the call waiting for you.
colors:
  paper: "#f4f2ea"
  sheet: "#fbfaf5"
  rule: "#d9d5c7"
  rule-strong: "#b3ae9c"
  ink: "#15140f"
  ink-2: "#46433a"
  ink-3: "#6b675b"
  timetable-yellow: "#ffd92e"
  yellow-soft: "#fff0a3"
  signal-red: "#d5001c"
  signal-soft: "#fbe4e4"
  go-green: "#1b7040"
  go-soft: "#e1f0e5"
  amber-ink: "#9a4d00"
typography:
  display:
    fontFamily: "Archivo Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "24px"
    fontWeight: 700
    lineHeight: 1.2
  title:
    fontFamily: "Archivo Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "18px"
    fontWeight: 700
    lineHeight: 1.3
  body:
    fontFamily: "Archivo Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.45
  label:
    fontFamily: "Archivo Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "12px"
    fontWeight: 500
    lineHeight: 1.3
  figure:
    fontFamily: "Archivo Variable, ui-sans-serif, system-ui, sans-serif"
    fontSize: "13px"
    fontWeight: 500
    fontFeature: "tnum"
  code:
    fontFamily: "ui-monospace, SFMono-Regular, Menlo, monospace"
    fontSize: "12px"
    lineHeight: 1.6
rounded:
  sm: "2px"
spacing:
  row-y: "8px"
  gutter: "16px"
  pane: "24px"
components:
  row-selected:
    backgroundColor: "{colors.yellow-soft}"
    textColor: "{colors.ink}"
  row-held:
    backgroundColor: "{colors.timetable-yellow}"
    textColor: "{colors.ink}"
  button-approve:
    backgroundColor: "{colors.ink}"
    textColor: "{colors.sheet}"
    rounded: "{rounded.sm}"
    padding: "8px 14px"
  button-reject:
    backgroundColor: "{colors.sheet}"
    textColor: "{colors.ink}"
    rounded: "{rounded.sm}"
    padding: "8px 14px"
  chip-critical:
    backgroundColor: "{colors.signal-red}"
    textColor: "{colors.sheet}"
    rounded: "{rounded.sm}"
    padding: "0 6px"
  chip-high:
    backgroundColor: "{colors.paper}"
    textColor: "{colors.signal-red}"
    rounded: "{rounded.sm}"
    padding: "0 6px"
  approval-dock:
    backgroundColor: "{colors.timetable-yellow}"
    textColor: "{colors.ink}"
    padding: "12px 24px"
---

# Design System: Agent Sentinel Cockpit

## Overview

**Creative North Star: "The Station Timetable"**

The cockpit is a departure board for an agent's actions. A developer glances at it on a second screen while the agent works, so the page reads like Swiss station signage: a warm paper ground, near-black ink, ruled rows, tabular times down the left edge. It lends the world its type, palette, density and one signature move; it does not copy a railway's decoration.

The signature is the station clock in the approval dock. When a call is held for a human decision, its red second hand makes one full turn over the approval timeout and the remaining time is written beside it. Everything else steps back while a call waits, so the eye lands on the yellow row and the clock.

**Key Characteristics:**
- Light, warm-white ground with ink-black type; one yellow that means "this is the held or selected row".
- Red is reserved for what needs the human or was refused.
- Square corners, hairline rules, no shadows; hierarchy by weight and position.
- Dense but ruled: rows are tables, not cards.

## Colors

A paper-and-ink palette with two signal colours, each with exactly one job.

### Primary
- **Timetable Yellow** (#ffd92e): the held row and the approval dock. It means "waiting for you".
- **Soft Yellow** (#fff0a3): the selected row and the active palette entry.

### Secondary
- **Signal Red** (#d5001c): critical or high risk, blocked and rejected calls, the clock hand, and a budget close to its limit. Nothing else.

### Neutral
- **Paper** (#f4f2ea): app ground, headers, the detail pane.
- **Sheet** (#fbfaf5): the timetable and payload blocks, one step brighter than paper.
- **Ink** (#15140f): text, the top bar, the approve button, 2px focus outlines.
- **Ink 2 / Ink 3** (#46433a / #6b675b): secondary text and quiet metadata.
- **Rule / Rule Strong** (#d9d5c7 / #b3ae9c): row hairlines and pane dividers.
- **Go Green** (#1b7040) and **Amber Ink** (#9a4d00): completed check marks and tool errors; diff additions sit on its soft tint (#e1f0e5).

### Named Rules
**The One Reserved Colour Rule.** Red appears only where the human must look or something was refused. If red is on screen, something needs attention.
**The Dim-the-Rest Rule.** While any call awaits approval, every other row recedes to 60% opacity; hover and focus restore it.

## Typography

**Display / Body / Label Font:** Archivo Variable (with ui-sans-serif, system-ui)
**Code Font:** system monospace, only for payloads, arguments and rule ids.

**Character:** A grotesque with a narrow width axis, the voice of station signage. Weight does the hierarchy; the time column uses the condensed width with tabular figures so it never jitters.

### Hierarchy
- **Display** (700, 24px, 1.2): the selected call's tool name in the detail header.
- **Title** (700, 18px, 1.3): the held call's name in the approval dock.
- **Body** (400, 14px, 1.45): rows, journey text, controls. The document base is 13px.
- **Label** (500, 12px): column headers, field labels, tabs.
- **Figure** (500, 13px, tabular, 78% width): times, durations, counts, the clock readout.

### Named Rules
**The Weight-Not-Icon Rule.** Tool calls are semibold, session and raw rows are regular and grey. Rows do not carry per-kind icons.

## Layout

A full-height column: ink top bar, a slim status strip, two panes, then the approval dock when needed. The panes split 5 : 7 from `lg` up (timetable left, detail right) and stack below it. The timetable is a grid with fixed columns (time 72px, status 20px, event flexible, policy 200px, took 64px); below `sm` the policy column is dropped. Spacing is a tight 8px row rhythm inside 16 to 24px gutters. The dock is part of the layout, never a floating overlay, so it cannot cover the rows it asks about.

## Elevation & Depth

Flat. There are no shadows. Depth is a step from paper to sheet and a 1px or 2px rule; the dock is separated by a 2px ink top border. Overlays (palette, export) dim the page with ink at 40% and carry no blur.

## Shapes

Square with a 2px corner on controls and chips. Timetable rows have no radius at all. The only circles are the journey stops and the station clock.

## Components

### Timetable row
Grid row with a hairline below. Selected: soft yellow. Held: full yellow, with a 2px inner ink outline when it is also selected. Refused calls strike their title through in red. Risk is written as a chip in the policy column.

### Approval dock (signature)
Yellow band with a 2px ink top border: station clock (64px, 60 ticks, red hand with a round tip, one turn over the approval timeout), remaining time in tabular figures, tool name, risk chip, rule id, the reason, and "+N waiting". Reject is outlined, Approve is ink-filled; ⌘↵ and Esc are shown on the buttons. An optional note field opens inline.

### Risk chip
Critical: filled red, light text. High: red outline and text. Medium and low: ink outline in greys. Uppercase, 11px, semibold, 2px corners.

### Tabs
Underline tabs: 2px ink underline on the active tab, grey text otherwise. Used for the event filter and the detail view (Payload, Diff, Raw JSON).

### Journey
A vertical hairline with a stop for each step of a call (Requested, Policy, Human, Answered). Stops are filled ink when done, yellow when held, red when refused.

## Do's and Don'ts

### Do:
- **Do** keep times and durations in tabular figures.
- **Do** write remaining time as text next to the clock hand.
- **Do** let the held row, and only it, carry full yellow.
- **Do** keep focus visible with the 2px ink outline.

### Don't:
- **Don't** use red for decoration, hover or branding.
- **Don't** add shadows, blur or rounded cards.
- **Don't** colour a status by hue alone; every status also has a glyph or a word.
- **Don't** put a sparkline or a hero metric on the status strip.
