## ADDED Requirements

### Requirement: Topology graph API
The system MUST expose a topology graph of projects, roles, bundles, applications, and mapping rules.

#### Scenario: Fetch graph data
- **WHEN** a client requests the topology endpoint
- **THEN** the response contains nodes and edges that describe the current access model

### Requirement: Graph resolves all references
The system MUST create placeholder nodes for persisted bundle or rule references that do not exist in the seeded demo catalog.

#### Scenario: Display legacy test references
- **WHEN** the database contains a mapping rule or bundle edge for an unknown project or role
- **THEN** the topology graph still includes synthetic project and role nodes so the edge can render

### Requirement: Graph UI supports inspection
The system MUST provide a graph view that supports project filtering and node inspection.

#### Scenario: Inspect a selected node
- **WHEN** a user opens the graph view and clicks a node
- **THEN** the inspector shows the node metadata and connected relationships for the visible topology

### Requirement: Visual lanes are stable
The system MUST organize the graph into stable lanes for applications, bundles, projects, and roles.

#### Scenario: Render the graph canvas
- **WHEN** the graph page loads
- **THEN** nodes are rendered in their lane order so relationships remain visually scannable
