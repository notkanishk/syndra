## Why

MkAuth has moved beyond the original bundle-and-mapping prototype into a usable admin surface with seeded demo data, access requests, governance summaries, and a topology graph. The docs need to reflect the actual product shape so future work is built against the same contract the app now exposes.

## What Changes

* Documents the current v1 feature surface instead of the earlier bundle-only milestone.
* Adds formal specs for the seeded demo catalog, access governance workflows, and the topology graph.
* Clarifies that production Zitadel keys, live networking, and rollout hardening remain later work.
* Keeps the architecture narrative aligned with the current UI, API, and LXC deployment workflow.

## Capabilities

### New Capabilities
* `demo-catalog`: Seeded users, projects, applications, and read models for local testing and UI population.
* `access-governance`: Direct grants with expiry, access requests, approvals, and governance summaries.
* `topology-graph`: A visual graph and API that expose projects, roles, bundles, applications, and mapping rules.

### Modified Capabilities

* None.

## Impact

* Affects backend read models, seed data, and the control-plane API.
* Affects the Next.js dashboard navigation and graph/access views.
* Establishes the spec baseline for future implementation changes and doc updates.
