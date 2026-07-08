// Backend response shapes (mirror backend/internal/zitadel & handlers) shared
// across the Zitadel diagnostics section components.

export interface HealthResponse {
  status: "ok" | "disabled" | "error";
  mode: "live" | "local-policy-only";
  domain?: string;
  projects_total?: number;
  latency_ms?: number;
  error?: string;
}

export interface Paginated<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

export interface ZitadelUser {
  id: string;
  userName: string;
  displayName: string;
  email: string;
  state: string;
}

export interface ZitadelProject {
  id: string;
  name: string;
  state: string;
}

export interface ProjectRole {
  key: string;
  displayName: string;
  group: string;
}

export interface UserGrant {
  id: string;
  userId: string;
  projectId: string;
  roleKeys: string[];
}

// Mirrors backend/internal/handlers/rotation_status.go:ActionRotationStatus.
// last_rotated_at and age_days are omitted by the backend when status is
// "disabled" or "unknown"; they're modelled as optional so render paths
// don't have to check magic sentinel values.
//
// Status ladder (backend-owned, precedence top to bottom):
//   - disabled : ZITADEL_ACTION_SIGNING_KEY unset on the backend —
//                signature verification is off. Any age reading would be
//                misleading, so the panel MUST render this as a misconfig.
//   - unknown  : key installed but ROTATED_AT unset/malformed/in the future.
//   - ok / warn / stale : age-based against the configured threshold.
export interface RotationStatus {
  key_installed: boolean;
  last_rotated_at?: string;
  age_days?: number;
  threshold_days: number;
  status: "ok" | "warn" | "stale" | "unknown" | "disabled";
  rotate_command: string;
}
