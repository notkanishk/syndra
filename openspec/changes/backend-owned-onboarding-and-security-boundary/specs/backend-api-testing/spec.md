## MODIFIED Requirements

### Requirement: Production-boundary regression coverage
The system MUST maintain regression coverage for backend user-token authorization on privileged actions, webhook authenticity, action-injection degraded behavior, and backend-owned onboarding mutation paths.

#### Scenario: Security-boundary behavior changes
- **WHEN** a privileged orchestration path changes
- **THEN** automated tests MUST verify backend authorization, authenticity enforcement, degraded-mode handling, idempotent retry behavior, and operator-visible failure reporting
