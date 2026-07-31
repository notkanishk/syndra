package models

import "time"

// Project represents a downstream application in Zitadel
type Project struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// Bundle represents a group of roles (e.g. "Student", "Lab Assistant")
type Bundle struct {
	ID               string    `json:"id"`
	Name             string    `json:"name"`
	Description      string    `json:"description"`
	IsWelcome        bool      `json:"is_welcome"`
	Roles            []string  `json:"roles"` // The underlying raw roles it applies
	ConfirmationMode string    `json:"confirmation_mode"`
	CreatedAt        time.Time `json:"created_at"`

	// HolderCount is how many people currently hold this bundle. Computed on
	// read, never persisted. Editing a bundle changes access for all of them
	// at once, so the number belongs next to the name rather than a click away.
	HolderCount int `json:"holder_count"`
}

// BundleRole represents a specific Zitadel role mapped to a bundle
type BundleRole struct {
	BundleID  string `json:"bundle_id"`
	ProjectID string `json:"zitadel_project_id"`
	RoleKey   string `json:"zitadel_role_key"`
}

// MappingRule defines absolute policy logic
// IF SourceProject + SourceRole THEN ADD TargetProject + TargetRole
type MappingRule struct {
	ID               string    `json:"id"`
	SourceProject    string    `json:"source_project"`
	SourceRole       string    `json:"source_role"`
	TargetProject    string    `json:"target_project"`
	TargetRole       string    `json:"target_role"`
	ConfirmationMode string    `json:"confirmation_mode"`
	CreatedAt        time.Time `json:"created_at"`

	// HolderCount is how many people currently hold the target role because of
	// this rule. Computed on read, never persisted — "this affects 7 people" is
	// the difference between editing a rule and gambling with one.
	HolderCount int `json:"holder_count"`
}

// ClaimProfile defines how roles map to JWT output for a specific project
type ClaimProfile struct {
	ID         string    `json:"id"`
	ProjectID  string    `json:"project_id"`
	ClaimName  string    `json:"claim_name"`  // e.g., "x-custom-group"
	FormatType string    `json:"format_type"` // e.g., "csv", "array"
	CreatedAt  time.Time `json:"created_at"`
}

// AuditLog tracks who granted what and when
type AuditLog struct {
	ID         string    `json:"id"`
	ActorID    string    `json:"actor_id"`
	TargetID   string    `json:"target_id"`
	Action     string    `json:"action"`
	ResourceID string    `json:"resource_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type UserProfile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Title    string `json:"title"`
	Team     string `json:"team"`
	Status   string `json:"status"`
	Avatar   string `json:"avatar"`
}

type ProjectRole struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	// Group is the identity provider's own grouping for the role ("Safety-gated",
	// "Open bench"). It is what separates "can cut unsupervised" from "may enter
	// and watch" at a glance, so it is its own field rather than being folded
	// into Description — which it used to be, making every upstream role render
	// its group where its description belonged.
	Group       string `json:"group,omitempty"`
	Description string `json:"description"`
}

type ProjectCatalog struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Kind        string        `json:"kind"`
	Description string        `json:"description"`
	Roles       []ProjectRole `json:"roles"`
}

type ApplicationCatalog struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ProjectID   string `json:"project_id"`
	Description string `json:"description"`
	Consumer    string `json:"consumer"`
	ClaimName   string `json:"claim_name"`
	FormatType  string `json:"format_type"`
}

type RoleGrant struct {
	ProjectID string `json:"project_id"`
	RoleKey   string `json:"role_key"`
}

type RoleReason struct {
	Kind           string `json:"kind"`
	Description    string `json:"description"`
	BundleID       string `json:"bundle_id,omitempty"`
	BundleName     string `json:"bundle_name,omitempty"`
	TriggerProject string `json:"trigger_project,omitempty"`
	TriggerRole    string `json:"trigger_role,omitempty"`
}

type EffectiveRole struct {
	ProjectID   string       `json:"project_id"`
	ProjectName string       `json:"project_name"`
	RoleKey     string       `json:"role_key"`
	IsSource    bool         `json:"is_source"`
	Reasons     []RoleReason `json:"reasons"`
}

type ProjectAccessView struct {
	ProjectID         string          `json:"project_id"`
	ProjectName       string          `json:"project_name"`
	SourceRoles       []EffectiveRole `json:"source_roles"`
	DerivedRoles      []EffectiveRole `json:"derived_roles"`
	EffectiveRoleKeys []string        `json:"effective_role_keys"`
}

type UserAccessView struct {
	User         UserProfile         `json:"user"`
	Bundles      []Bundle            `json:"bundles"`
	Projects     []ProjectAccessView `json:"projects"`
	CleanupHints []string            `json:"cleanup_hints"`
}

// UserListItem is one row of the People index. Beyond the counts, it carries
// the "needs attention" trio — the one thing about this person that might need
// an operator today. The index exists to surface those; without them it is a
// plain directory, and a directory is not worth a top-level destination.
type UserListItem struct {
	User               UserProfile `json:"user"`
	BundleCount        int         `json:"bundle_count"`
	BundleNames        []string    `json:"bundle_names"`
	EffectiveRoleCount int         `json:"effective_role_count"`
	ProjectCount       int         `json:"project_count"`
	KeyProjects        []string    `json:"key_projects"`

	// Needs attention. Each is a count so the UI can render the semantic
	// colour it belongs to: expiring is amber, open requests accent,
	// unexplained red.
	ExpiringCount    int `json:"expiring_count"`
	OpenRequestCount int `json:"open_request_count"`
	UnexplainedCount int `json:"unexplained_count"`

	// SoonestExpiry is the nearest expiry among this person's direct grants
	// inside the watch window, so the row can say "1 expires in 2 days"
	// rather than only "1 expiring".
	SoonestExpiry *time.Time `json:"soonest_expiry,omitempty"`
}

type ApplicationView struct {
	Application       ApplicationCatalog `json:"application"`
	ConsumedRoles     []string           `json:"consumed_roles"`
	AssignedUserCount int                `json:"assigned_user_count"`
}

// ApplicationSimulation is the dry run of a real token. CustomClaims is the
// exact map the Actions v2 path would append for this (user, project) pair —
// same profile resolution, same shaper, nothing invented for display.
//
// A project's token carries every claim key configured on that project, since
// Zitadel's function trigger does not say which application the token is for.
// OwnedClaims narrows that to the keys THIS application reads; ClaimOwners
// attributes the rest, so a sibling app's key is never mistaken for a bug.
type ApplicationSimulation struct {
	Application  ApplicationCatalog     `json:"application"`
	User         UserProfile            `json:"user"`
	RawRoles     []string               `json:"raw_roles"`
	CustomClaims map[string]interface{} `json:"custom_claims"`
	OwnedClaims  []string               `json:"owned_claims"`
	ClaimOwners  []ClaimKeyOwner        `json:"claim_owners"`
}

// ClaimKeyOwner is one key present in an issued token and the profile that put
// it there. Kind is roles | attribute | static; Source names the attribute
// when Kind is attribute.
type ClaimKeyOwner struct {
	Key           string `json:"key"`
	OwnerLabel    string `json:"owner_label"`
	ApplicationID string `json:"application_id,omitempty"`
	Kind          string `json:"kind"`
	Source        string `json:"source,omitempty"`
}

type ProjectSummary struct {
	Project        ProjectCatalog `json:"project"`
	MemberCount    int            `json:"member_count"`
	BundleCount    int            `json:"bundle_count"`
	RuleInCount    int            `json:"rule_in_count"`
	RuleOutCount   int            `json:"rule_out_count"`
	ActiveRoleKeys []string       `json:"active_role_keys"`
	SampleMembers  []string       `json:"sample_members"`
}

type CatalogResponse struct {
	Users        []UserProfile        `json:"users"`
	Projects     []ProjectCatalog     `json:"projects"`
	Applications []ApplicationCatalog `json:"applications"`
}

type BundleImpact struct {
	BundleID  string        `json:"bundle_id"`
	RoleCount int           `json:"role_count"`
	Users     []UserProfile `json:"users"`
}

type DirectGrant struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	ProjectID string     `json:"project_id"`
	RoleKey   string     `json:"role_key"`
	GrantedBy string     `json:"granted_by"`
	Reason    string     `json:"reason"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	Source    string     `json:"source"`               // direct | bundle | rule | external_backfill | lifecycle_cascade
	SourceRef string     `json:"source_ref,omitempty"` // bundle_id / rule_id when source ∈ {bundle, rule}
}

// PendingPropagation is one buffered MkAuth-mediated Zitadel grant mutation.
// `applied` is terminal success (synchronous 2xx); there is no `confirmed` state
// (design Decision 1: the self-mutation guard drops MkAuth's own grant events,
// so no webhook round-trip can confirm a propagation).
type PendingPropagation struct {
	ID             string     `json:"id"`
	OpType         string     `json:"op_type"` // add | revoke | replace
	UserID         string     `json:"user_id"`
	ProjectID      string     `json:"project_id"`
	RoleKeys       []string   `json:"role_keys"`
	Source         string     `json:"source"`               // direct | bundle | rule | external_backfill | lifecycle_cascade
	SourceRef      string     `json:"source_ref,omitempty"` // bundle/rule id for cascade rows; drives worklist attribution
	CascadeID      string     `json:"cascade_id,omitempty"` // shared by every write one triggering event produced
	ZitadelGrantID string     `json:"zitadel_grant_id,omitempty"`
	Status         string     `json:"status"` // pending | in_flight | applied | failed
	Attempts       int        `json:"attempts"`
	LastError      string     `json:"last_error,omitempty"`
	InitiatedBy    string     `json:"initiated_by"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

// CascadeSummary is one applied cascade-originated outbox row, surfaced for the
// operator's "Recent cascades" feed (Task 22). "Applied" here means the
// projection reached Zitadel — not specifically that it drained automatically
// (the outbox does not persist auto-vs-operator-resumed per row; see
// db.GetRecentCascades doc comment).
type CascadeSummary struct {
	ID          string     `json:"id"`
	OpType      string     `json:"op_type"` // add | revoke | replace
	UserID      string     `json:"user_id"`
	ProjectID   string     `json:"project_id"`
	RoleKeys    []string   `json:"role_keys"`
	Source      string     `json:"source"`               // bundle | rule | lifecycle_cascade
	SourceRef   string     `json:"source_ref,omitempty"` // originating bundle/rule id
	CascadeID   string     `json:"cascade_id,omitempty"`
	Status      string     `json:"status"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// CascadeGroup is Change history's unit: every write one triggering event
// produced, collapsed into one entry. An operator reads consequence, not a
// diff — "8 applied", "2 waiting", "no writes" is the whole vocabulary, and a
// half-applied cascade has to be visible AS a half-applied cascade.
type CascadeGroup struct {
	CascadeID string     `json:"cascade_id"`
	Source    string     `json:"source"`               // bundle | rule | lifecycle_cascade
	SourceRef string     `json:"source_ref,omitempty"` // originating bundle/rule id
	Applied   int        `json:"applied"`
	Waiting   int        `json:"waiting"`
	Failed    int        `json:"failed"`
	UserIDs   []string   `json:"user_ids"`
	Writes    []CascadeSummary `json:"writes"`
	StartedAt time.Time  `json:"started_at"`
	SettledAt *time.Time `json:"settled_at,omitempty"`
}

// DriftItem is one out-of-band grant discrepancy awaiting operator triage.
// zitadel_only: exists in Zitadel, no MkAuth intent. mkauth_only: MkAuth
// expects it (direct grant), Zitadel lacks it. No item resolves automatically.
type DriftItem struct {
	ID                string     `json:"id"`
	UserID            string     `json:"user_id"`
	ProjectID         string     `json:"project_id"`
	RoleKeys          []string   `json:"role_keys"`
	ZitadelGrantID    string     `json:"zitadel_grant_id,omitempty"`
	DetectedAt        time.Time  `json:"detected_at"`
	DetectionSource   string     `json:"detection_source"` // webhook | reconciliation_sweep
	DriftType         string     `json:"drift_type"`       // zitadel_only | mkauth_only
	Status            string     `json:"status"`           // pending_triage | attributed | revoked | marked_external
	ResolvedAt        *time.Time `json:"resolved_at,omitempty"`
	ResolvedBy        string     `json:"resolved_by,omitempty"`
	ResolutionPayload string     `json:"resolution_payload_json,omitempty"`

	// Evidence — who made this upstream and when. Nullable by design: the
	// reconciliation sweep compares grant sets and genuinely cannot know the
	// actor, and an invented one is worse than an absent one. The triage row
	// says "we don't know who" rather than guessing.
	UpstreamActor     string     `json:"upstream_actor,omitempty"`
	UpstreamCreatedAt *time.Time `json:"upstream_created_at,omitempty"`
	LastSeenAt        *time.Time `json:"last_seen_at,omitempty"`
}

// DriftTriageItem is a DriftItem enriched for the triage queue: enough context
// on the row to answer "what is this, and what happens if I revoke it" without
// a click. Computed on read; nothing here is persisted.
type DriftTriageItem struct {
	DriftItem

	// RoleGroup is the catalogue group of the drifting role ("Safety-gated",
	// "Open bench"). Empty when the role is not in the catalogue at all —
	// see RoleInCatalogue, which is its own kind of finding.
	RoleGroup string `json:"role_group,omitempty"`

	// RoleInCatalogue is false when the role no longer exists in MkAuth.
	// Adopting such a row would recreate a retired role, so the UI says so.
	RoleInCatalogue bool `json:"role_in_catalogue"`

	// UserStatus mirrors the directory ("active", "departed", …) and
	// UserIsServiceAccount marks machine accounts, for which "adopt" is the
	// wrong verb and "owned elsewhere" is almost always the right one.
	UserStatus           string `json:"user_status,omitempty"`
	UserIsServiceAccount bool   `json:"user_is_service_account"`

	// OtherItemsForUser is how many OTHER pending items this same person has.
	// "Marta has 2 more items" is the context that changes a revoke decision.
	OtherItemsForUser int `json:"other_items_for_user"`
}

// DriftSummary feeds the red dashboard callout + sidebar dot: pending count
// plus a top-N preview for the "top-3 + Triage all →" callout.
type DriftSummary struct {
	Count int         `json:"count"`
	Top   []DriftItem `json:"top,omitempty"`
}

// ExternalGrantExclusion is an operator "this is legitimately external" marker
// keyed by (user, project, role) — future detections for the triple are filtered.
type ExternalGrantExclusion struct {
	UserID    string    `json:"user_id"`
	ProjectID string    `json:"project_id"`
	RoleKey   string    `json:"role_key"`
	MarkedBy  string    `json:"marked_by"`
	MarkedAt  time.Time `json:"marked_at"`
	Reason    string    `json:"reason,omitempty"`
}

type AccessRequest struct {
	ID            string     `json:"id"`
	RequesterID   string     `json:"requester_id"`
	ProjectID     string     `json:"project_id"`
	RoleKey       string     `json:"role_key"`
	Justification string     `json:"justification"`
	DurationDays  *int       `json:"duration_days,omitempty"`
	Status        string     `json:"status"`
	ReviewerID    string     `json:"reviewer_id,omitempty"`
	ReviewNote    string     `json:"review_note,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	ResolvedAt    *time.Time `json:"resolved_at,omitempty"`
}

type GovernanceSummary struct {
	PendingRequests    []AccessRequest           `json:"pending_requests"`
	ExpiringGrants     []DirectGrant             `json:"expiring_grants"`
	CleanupHints       []string                  `json:"cleanup_hints"`
	PendingPropagation PendingPropagationSummary `json:"pending_propagation"`
	Drift              DriftSummary              `json:"drift"`
}

// PendingPropagationSummary surfaces the outbox depth + reachability so the UI
// can render the amber "N changes awaiting Zitadel" callout and gate the
// operator's "Resume now" action on whether Zitadel is reachable at all.
type PendingPropagationSummary struct {
	Count            int  `json:"count"`
	ZitadelReachable bool `json:"zitadel_reachable"`
}

type TopologyNode struct {
	ID          string            `json:"id"`
	Label       string            `json:"label"`
	Kind        string            `json:"kind"`
	ProjectID   string            `json:"project_id,omitempty"`
	Description string            `json:"description,omitempty"`
	Meta        map[string]string `json:"meta,omitempty"`
}

type TopologyEdge struct {
	ID     string            `json:"id"`
	Source string            `json:"source"`
	Target string            `json:"target"`
	Kind   string            `json:"kind"`
	Label  string            `json:"label"`
	Meta   map[string]string `json:"meta,omitempty"`
}

type TopologyGraph struct {
	Nodes []TopologyNode `json:"nodes"`
	Edges []TopologyEdge `json:"edges"`
}

// Role represents a role created/managed through MkAuth, stored locally.
type Role struct {
	ID                string    `json:"id"`
	ProjectID         string    `json:"project_id"`
	RoleKey           string    `json:"role_key"`
	DisplayName       string    `json:"display_name"`
	Description       string    `json:"description"`
	Group             string    `json:"group"`
	ClonedFromProject string    `json:"cloned_from_project,omitempty"`
	ClonedFromRole    string    `json:"cloned_from_role,omitempty"`
	CreatedBy         string    `json:"created_by"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ProvisioningIntent represents a pending infrastructure mutation
// to be consumed by the Sync Service for LLDAP group management.
type ProvisioningIntent struct {
	ID             string     `json:"id"`
	TargetUID      string     `json:"target_uid"`
	Action         string     `json:"action"`
	LLDAPGroup     string     `json:"lldap_group"`
	SourceProject  string     `json:"source_project"`
	SourceRole     string     `json:"source_role"`
	WebhookEventID string     `json:"webhook_event_id,omitempty"`
	IdempotencyKey string     `json:"idempotency_key"`
	Status         string     `json:"status"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	AcknowledgedAt *time.Time `json:"acknowledged_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

// ShadowCredential stores a secondary Argon2id-hashed password for LLDAP/Samba access.
type ShadowCredential struct {
	ID             string     `json:"id"`
	UserID         string     `json:"user_id"`
	CredentialHash string     `json:"credential_hash,omitempty"`
	Algorithm      string     `json:"algorithm"`
	SaltParams     string     `json:"salt_params,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	RotatedAt      *time.Time `json:"rotated_at,omitempty"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty"`
}

// ShadowCredentialAudit records credential lifecycle events.
type ShadowCredentialAudit struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Action    string    `json:"action"`
	ActorID   string    `json:"actor_id"`
	IPAddress string    `json:"ip_address"`
	CreatedAt time.Time `json:"created_at"`
}

// ShadowCredentialStatus is the user-facing view (no hash exposed).
type ShadowCredentialStatus struct {
	HasCredential bool       `json:"has_credential"`
	Algorithm     string     `json:"algorithm,omitempty"`
	CreatedAt     *time.Time `json:"created_at,omitempty"`
	UpdatedAt     *time.Time `json:"updated_at,omitempty"`
	RotatedAt     *time.Time `json:"rotated_at,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
}

// CatalogRole is the computed view for the global role inventory.
//
// Group and the ClonedFrom pair come from the local roles table when MkAuth
// created the role. Both are load-bearing on screen: Group is what separates
// "Safety-gated" from "Open bench" at a glance, and clone provenance is how an
// operator knows two similar roles are deliberately related rather than an
// accidental duplicate.
type CatalogRole struct {
	ProjectID         string `json:"project_id"`
	ProjectName       string `json:"project_name"`
	RoleKey           string `json:"role_key"`
	DisplayName       string `json:"display_name"`
	Description       string `json:"description"`
	Group             string `json:"group,omitempty"`
	ClonedFromProject string `json:"cloned_from_project,omitempty"`
	ClonedFromRole    string `json:"cloned_from_role,omitempty"`
	BundleCount       int    `json:"bundle_count"`
	RuleCount         int    `json:"rule_count"`
	AssignedUserCount int    `json:"assigned_user_count"`
	IsUnused          bool   `json:"is_unused"`
	Source            string `json:"source"`        // "mkauth" | "demo" | "referenced"
	DisplayLabel      string `json:"display_label"` // "Printing Lab: admin"
}
