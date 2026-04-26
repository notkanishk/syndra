> **Status:** Integrated | [< Index](../../../../INDEX.md)

## ADDED Requirements

### Requirement: Topology canvas MUST support pan, zoom, and reset

The God Mode topology canvas MUST let admins navigate large graphs by panning and zooming the viewport, with a reset affordance to return to the default view.

#### Scenario: Pan via mouse drag
- **WHEN** an admin presses and drags the empty canvas surface (not a node)
- **THEN** the viewport MUST translate to follow the cursor
- **AND** node `<button>` elements MUST stop mousedown propagation so clicking a node does not start a pan

#### Scenario: Zoom via Cmd/Ctrl + scroll
- **WHEN** an admin scrolls the wheel while holding Cmd or Ctrl over the canvas
- **THEN** the viewport scale MUST adjust (clamped between 0.4 and 2.5)
- **AND** scrolling without a modifier MUST NOT zoom (so vertical scroll on the surrounding page remains usable)

#### Scenario: Overlay zoom controls
- **WHEN** the canvas renders
- **THEN** an overlay control panel MUST be visible with `+`, `−`, and "Reset" buttons
- **AND** the panel MUST display the current zoom percentage
- **AND** a small "Drag to pan · ⌘/Ctrl + scroll to zoom" hint MUST be visible to teach the gesture

### Requirement: Inspector MUST link nodes to their detail pages

The topology Inspector MUST surface a deeplink to the relevant detail page for the selected node so admins can pivot from visual exploration to focused editing without manual navigation.

#### Scenario: View details deeplink
- **WHEN** an admin selects a node in the canvas
- **THEN** the Inspector MUST display a "View details →" link
- **AND** application/bundle/project/role nodes MUST link to `/applications`, `/bundles`, `/projects`, or `/projects` respectively
