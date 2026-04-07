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
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Roles       []string  `json:"roles"` // The underlying raw roles it applies
	CreatedAt   time.Time `json:"created_at"`
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
	ID            string    `json:"id"`
	SourceProject string    `json:"source_project"`
	SourceRole    string    `json:"source_role"`
	TargetProject string    `json:"target_project"`
	TargetRole    string    `json:"target_role"`
	Version       int       `json:"version"`
	CreatedAt     time.Time `json:"created_at"`
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
	Location string `json:"location"`
}

type ProjectRole struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
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
	Kind          string `json:"kind"`
	Description   string `json:"description"`
	BundleID      string `json:"bundle_id,omitempty"`
	BundleName    string `json:"bundle_name,omitempty"`
	TriggerProject string `json:"trigger_project,omitempty"`
	TriggerRole   string `json:"trigger_role,omitempty"`
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
	User         UserProfile        `json:"user"`
	Bundles      []Bundle          `json:"bundles"`
	Projects     []ProjectAccessView `json:"projects"`
	CleanupHints []string          `json:"cleanup_hints"`
}

type UserListItem struct {
	User               UserProfile `json:"user"`
	BundleCount        int         `json:"bundle_count"`
	EffectiveRoleCount int         `json:"effective_role_count"`
	KeyProjects        []string    `json:"key_projects"`
}

type ApplicationView struct {
	Application       ApplicationCatalog `json:"application"`
	ConsumedRoles     []string           `json:"consumed_roles"`
	AssignedUserCount int                `json:"assigned_user_count"`
}

type ApplicationSimulation struct {
	Application  ApplicationCatalog      `json:"application"`
	User         UserProfile             `json:"user"`
	RawRoles     []string                `json:"raw_roles"`
	CustomClaims map[string]interface{}  `json:"custom_claims"`
}

type ProjectSummary struct {
	Project         ProjectCatalog `json:"project"`
	MemberCount     int            `json:"member_count"`
	BundleCount     int            `json:"bundle_count"`
	RuleInCount     int            `json:"rule_in_count"`
	RuleOutCount    int            `json:"rule_out_count"`
	ActiveRoleKeys  []string       `json:"active_role_keys"`
	SampleMembers   []string       `json:"sample_members"`
}

type CatalogResponse struct {
	Users        []UserProfile        `json:"users"`
	Projects     []ProjectCatalog     `json:"projects"`
	Applications []ApplicationCatalog `json:"applications"`
}

type BundleImpact struct {
	BundleID   string        `json:"bundle_id"`
	RoleCount  int           `json:"role_count"`
	Users      []UserProfile `json:"users"`
}

type DirectGrant struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	ProjectID  string     `json:"project_id"`
	RoleKey    string     `json:"role_key"`
	GrantedBy  string     `json:"granted_by"`
	Reason     string     `json:"reason"`
	ExpiresAt  *time.Time `json:"expires_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type AccessRequest struct {
	ID           string     `json:"id"`
	RequesterID  string     `json:"requester_id"`
	ProjectID    string     `json:"project_id"`
	RoleKey      string     `json:"role_key"`
	Justification string    `json:"justification"`
	DurationDays *int       `json:"duration_days,omitempty"`
	Status       string     `json:"status"`
	ReviewerID   string     `json:"reviewer_id,omitempty"`
	ReviewNote   string     `json:"review_note,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
}

type GovernanceSummary struct {
	PendingRequests []AccessRequest `json:"pending_requests"`
	ExpiringGrants  []DirectGrant   `json:"expiring_grants"`
	CleanupHints    []string        `json:"cleanup_hints"`
}
