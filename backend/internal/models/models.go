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
