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
	// read, never persisted. Publishing a version can change access for all of
	// them at once, so the number belongs next to the name rather than a click
	// away.
	HolderCount int `json:"holder_count"`

	// LatestVersion is the highest published version number, and StaleHolders
	// how many people are pinned to something older than it. Both computed on
	// read.
	//
	// StaleHolders is the number the list exists to show. A bundle where
	// everybody is current and one where eleven people are two versions back
	// are different objects, and only one of them means "the edit you made
	// last term never reached anyone".
	LatestVersion int `json:"latest_version"`
	StaleHolders  int `json:"stale_holders"`

	// UnpublishedChanges is how many role additions or removals sit in the
	// working copy and have not been published. Zero means the bundle and its
	// latest version agree.
	UnpublishedChanges int `json:"unpublished_changes"`

	// PinnedVersion is set only when the bundle was read for one person: which
	// version of it THEY hold. Zero elsewhere.
	//
	// It is on the person's copy of the bundle rather than looked up separately
	// because "Ada has Lab Tech" and "Ada has Lab Tech v2" are the same fact,
	// and the screens that say the first should not have to fetch again to say
	// the second.
	PinnedVersion int `json:"pinned_version,omitempty"`
}

// BundleVersion is one published snapshot of a bundle's roles.
//
// Immutable once written: an operator saying "they are on v2" has to be able to
// look up what v2 was, and a snapshot that can be edited afterwards answers a
// different question than the one they asked.
type BundleVersion struct {
	ID          string    `json:"id"`
	BundleID    string    `json:"bundle_id"`
	Version     int       `json:"version"`
	Note        string    `json:"note"`
	PublishedBy string    `json:"published_by"`
	PublishedAt time.Time `json:"published_at"`

	// HolderCount is how many people are pinned to THIS version. Computed on read.
	HolderCount int `json:"holder_count"`
	// LatestVersion is the bundle's highest version, so a reader can tell how
	// far behind this one is without a second lookup.
	LatestVersion int `json:"latest_version,omitempty"`
	// Roles is populated only where a caller asked for the contents.
	Roles []BundleRole `json:"roles,omitempty"`
}

// BundleHolder is one person's pin: which version of a bundle they hold.
type BundleHolder struct {
	BundleID   string    `json:"bundle_id"`
	UserID     string    `json:"user_id"`
	VersionID  string    `json:"version_id"`
	Version    int       `json:"version"`
	AssignedAt time.Time `json:"assigned_at"`
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
	// CascadeID names the cascade this event set off, matching CascadeGroup.CascadeID. Empty for
	// events that cascaded to nobody and for every row written before migration 000023 — the
	// console renders those without a lineage link rather than guessing one.
	CascadeID string `json:"cascade_id,omitempty"`
}

type UserProfile struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Title  string `json:"title"`
	Team   string `json:"team"`
	Status string `json:"status"`
	Avatar string `json:"avatar"`
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

// AllowanceBand is the third band beside Source and Derived (design §6).
//
// Rendered distinctly on purpose. A subject can hold a role whose access they
// do not have, and that is a trap unless it is visible — a role-holder list
// that shows somebody as holding access they are suspended from is worse than
// not showing the list at all. So "why does this person have access to X"
// answers with exactly one of: the role gives it, a rule derived it, or
// somebody explicitly decided — and this band carries the third, with actor and
// time attached to the person rather than erased into an absence.
type AllowanceBand struct {
	ID        string `json:"id"`
	Target    string `json:"target"`
	Field     string `json:"field"`
	Value     string `json:"value"`
	Direction string `json:"direction"`
	ActorID   string `json:"actor_id"`
	Reason    string `json:"reason"`
	// InForce and ReviewDue are derived at read time rather than stored, so
	// neither can go stale in a column while the date it depends on passes.
	InForce   bool   `json:"in_force"`
	ReviewDue bool   `json:"review_due"`
	CreatedAt string `json:"created_at"`
	// Ended says when and how it stopped applying: a date that arrived, or a
	// person who lifted it. Empty while it still applies. Lapsed, lifted and in
	// force are three states an operator asks about differently.
	Ended   string `json:"ended,omitempty"`
	EndedBy string `json:"ended_by,omitempty"`
}

type UserAccessView struct {
	User     UserProfile         `json:"user"`
	Bundles  []Bundle            `json:"bundles"`
	Projects []ProjectAccessView `json:"projects"`
	// Allowances is the third band. Present whether or not any is in force,
	// because a suspension that ended is part of the answer to "what has been
	// decided about this person" and erasing it would leave the band looking
	// like it had never been used.
	Allowances   []AllowanceBand `json:"allowances"`
	CleanupHints []string        `json:"cleanup_hints"`
}

// UserListItem is one row of the People index. Beyond the counts, it carries
// the "needs attention" trio — the one thing about this person that might need
// an operator today. The index exists to surface those; without them it is a
// plain directory, and a directory is not worth a top-level destination.
type UserListItem struct {
	User        UserProfile `json:"user"`
	BundleCount int         `json:"bundle_count"`
	BundleNames []string    `json:"bundle_names"`
	// BundleVersions maps bundle name → the version THIS person is pinned to.
	// Alongside BundleNames rather than replacing it: the chips render from the
	// names, and the People filter narrows "in the Lab Tech bundle" down to "on
	// v2 of it" from the same row without a second request.
	BundleVersions     map[string]int `json:"bundle_versions,omitempty"`
	EffectiveRoleCount int            `json:"effective_role_count"`
	ProjectCount       int            `json:"project_count"`
	KeyProjects        []string       `json:"key_projects"`
	// KeyProjectIDs is the same set addressed by id rather than display name.
	// Names are what an operator reads; ids are what a link can carry without
	// breaking when a project is renamed, and what a role-scoped filter needs
	// to be exact. Both are sent because the People filter shows one and
	// matches on the other.
	KeyProjectIDs []string `json:"key_project_ids"`

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

// GrantExpiryAcknowledgement is an operator's recorded decision to let a grant lapse on its date.
//
// It changes nothing about the access: the expiry sweep still removes the grant when the date
// arrives. What it changes is the queue — "nobody has looked at this" and "somebody looked and
// decided" stop being indistinguishable.
type GrantExpiryAcknowledgement struct {
	By   string    `json:"by"`
	At   time.Time `json:"at"`
	Note string    `json:"note,omitempty"`
}

// ExpiringGrant is a direct grant approaching its expiry, with the acknowledgement that currently
// applies to it, if any.
type ExpiringGrant struct {
	DirectGrant
	// Acknowledged is set only while the acknowledgement is still ABOUT this grant — it was made
	// against a specific expiry date, and a grant whose date has since moved is a different
	// question that nobody has answered yet. Nothing invalidates the stored row; the read
	// compares, so a stale acknowledgement simply stops being one.
	Acknowledged *GrantExpiryAcknowledgement `json:"acknowledged,omitempty"`
}

// PendingPropagation is one buffered Syndra-mediated Zitadel grant mutation.
// `applied` is terminal success (synchronous 2xx); there is no `confirmed` state
// (design Decision 1: the self-mutation guard drops Syndra's own grant events,
// so no webhook round-trip can confirm a propagation).
type PendingPropagation struct {
	ID string `json:"id"`
	// Target is which system this row converges. Carried on the row itself
	// because the drain that dispatches it must know: a TrueNAS row pushed
	// through the Zitadel path has no project and no roles to send.
	Target         string     `json:"target"`
	OpType         string     `json:"op_type"` // add | revoke | replace | apply
	UserID         string     `json:"user_id"`
	ProjectID      string     `json:"project_id"`
	RoleKeys       []string   `json:"role_keys"`
	Source         string     `json:"source"`               // direct | bundle | rule | external_backfill | lifecycle_cascade
	SourceRef      string     `json:"source_ref,omitempty"` // bundle/rule id for cascade rows; drives worklist attribution
	CascadeID      string     `json:"cascade_id,omitempty"` // shared by every write one triggering event produced
	ZitadelGrantID string     `json:"zitadel_grant_id,omitempty"`
	Status         string     `json:"status"` // pending | in_flight | applied | failed | superseded | abandoned
	Attempts       int        `json:"attempts"`
	LastError      string     `json:"last_error,omitempty"`
	InitiatedBy    string     `json:"initiated_by"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
}

// CascadeSummary is one cascade-originated outbox write, as it appears inside
// the CascadeGroup that Change history renders. `Status` is the row's own —
// applied, waiting or failed — because a group is only readable as a group when
// each write in it says which it was.
//
// It once backed a second, flat feed of its own (GET /propagations/cascades).
// That endpoint is gone: one entry per cascade is the readable unit, and one
// row per write was the same data with the causation removed.
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
	CascadeID string           `json:"cascade_id"`
	Source    string           `json:"source"`               // bundle | rule | lifecycle_cascade
	SourceRef string           `json:"source_ref,omitempty"` // originating bundle/rule id
	Applied   int              `json:"applied"`
	Waiting   int              `json:"waiting"`
	Failed    int              `json:"failed"`
	UserIDs   []string         `json:"user_ids"`
	Writes    []CascadeSummary `json:"writes"`
	StartedAt time.Time        `json:"started_at"`
	SettledAt *time.Time       `json:"settled_at,omitempty"`
}

// DriftItem is one out-of-band grant discrepancy awaiting operator triage.
// target_only: exists on the target, no Syndra intent. syndra_only: Syndra
// expects it (direct grant), the target lacks it. No item resolves
// automatically. Which target drifted is the row's `target` column, not part of
// the drift type — `zitadel_only` was the pre-add-on name and would be a false
// statement on any target that is not Zitadel.
type DriftItem struct {
	ID string `json:"id"`

	// Target names what drifted. Every other field on this row is a statement
	// about that target — a role key means nothing without knowing whose role
	// catalogue it belongs to, and "unexplained access" is unexplained
	// somewhere in particular.
	Target string `json:"target"`

	UserID            string     `json:"user_id"`
	ProjectID         string     `json:"project_id"`
	RoleKeys          []string   `json:"role_keys"`
	ZitadelGrantID    string     `json:"zitadel_grant_id,omitempty"`
	DetectedAt        time.Time  `json:"detected_at"`
	DetectionSource   string     `json:"detection_source"` // webhook | reconciliation_sweep
	DriftType         string     `json:"drift_type"`       // target_only | syndra_only
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

	// RoleInCatalogue is false when the role no longer exists in Syndra.
	// Adopting such a row would recreate a retired role, so the UI says so.
	// Read it only together with RoleCatalogueApplies.
	RoleInCatalogue bool `json:"role_in_catalogue"`

	// RoleCatalogueApplies is false on a target that has no role catalogue at
	// all. RoleInCatalogue is then meaningless rather than false: nothing was
	// retired, because there was never a catalogue to retire it from. Without
	// this the UI would report every add-on drift row as a retired role.
	RoleCatalogueApplies bool `json:"role_catalogue_applies"`

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
	// Target scopes the exclusion. "This grant is legitimately external" is
	// true of one target at a time: the same triple on another target is a
	// different grant, made by different hands, and silencing it here would
	// silence a finding nobody looked at.
	Target    string    `json:"target"`
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

// Role represents a role created/managed through Syndra, stored locally.
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
// Group and the ClonedFrom pair come from the local roles table when Syndra
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
	Source            string `json:"source"` // "syndra" | "demo" | "referenced"
}
