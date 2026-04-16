package backend

// ProvisioningIntent mirrors the backend's intent model.
// Only fields needed by the sync service are included.
type ProvisioningIntent struct {
	ID             string  `json:"id"`
	TargetUID      string  `json:"target_uid"`
	Action         string  `json:"action"` // "add" | "remove"
	LLDAPGroup     string  `json:"lldap_group"`
	SourceProject  string  `json:"source_project"`
	SourceRole     string  `json:"source_role"`
	WebhookEventID string  `json:"webhook_event_id,omitempty"`
	IdempotencyKey string  `json:"idempotency_key"`
	Status         string  `json:"status"`
	ErrorMessage   string  `json:"error_message,omitempty"`
	CreatedAt      string  `json:"created_at"`
	AcknowledgedAt *string `json:"acknowledged_at,omitempty"`
	CompletedAt    *string `json:"completed_at,omitempty"`
}

// ShadowCredentialHash is the response from the shadow credential hash endpoint.
type ShadowCredentialHash struct {
	CredentialHash string `json:"credential_hash"`
	Algorithm      string `json:"algorithm"`
}

// UserProfile is the response from the user profile endpoint.
type UserProfile struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}
