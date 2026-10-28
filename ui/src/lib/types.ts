// API response types — mirrors Go models returned by the backend.
// Page-level components may define their own narrower types for client-side use.

export interface Bundle {
  id: string;
  name: string;
  description: string;
  roles: string[];
  created_at: string;
}

export interface ProjectRole {
  key: string;
  label: string;
  description: string;
}

export interface ProjectCatalog {
  id: string;
  name: string;
  kind: string;
  description: string;
  roles: ProjectRole[];
}

export interface ApplicationCatalog {
  id: string;
  name: string;
  project_id: string;
  description: string;
  consumer: string;
  claim_name: string;
  format_type: string;
}

export interface UserProfile {
  id: string;
  name: string;
  email: string;
  title: string;
  team: string;
  status: string;
  avatar: string;
  kind?: "human" | "machine";
  /** Effective: a machine account, or a human-type account an operator marked. */
  service_account?: boolean;
  /** When the account was created in Zitadel (RFC3339). */
  joined?: string;
}

/** What a service account is for (A-07 §26). */
export type ServiceAccountKind = "integration" | "device" | "agent" | "shared_login";

/** GET /service-accounts: every machine user plus every marked human, by name. */
export interface ServiceAccount {
  id: string;
  name: string;
  kind: ServiceAccountKind;
  zitadel_type: "machine" | "human";
  purpose: string;
  owner_id: string;
  role_count: number;
  marked_by: string;
  marked_at: string | null;
}

export interface CatalogResponse {
  users: UserProfile[];
  projects: ProjectCatalog[];
  applications: ApplicationCatalog[];
}

/**
 * What a cascade did, as the backend reports it.
 *
 * `enqueued` is what is now WAITING, and it is the only number a surface may
 * turn into a sentence about Pending changes. It is not a count of what
 * changed: a removal that withdraws deliveries which had never been sent
 * changes a person's record and leaves nothing waiting at all, and reporting
 * that as "the changes wait under Pending changes until you send them" tells
 * an operator to go and confirm an empty screen.
 */
export interface CascadeOutcome {
  message?: string;
  cascade?: {
    enqueued: number;
    mode: string;
    no_op?: boolean;
  };
}
