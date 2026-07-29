package backend

// ProvisioningIntent mirrors the backend's intent model.
// Only fields the sync service actually reads are declared — the decoder
// ignores the rest of the backend's payload.
type ProvisioningIntent struct {
	ID         string `json:"id"`
	TargetUID  string `json:"target_uid"`
	Action     string `json:"action"` // "add" | "remove"
	LLDAPGroup string `json:"lldap_group"`
}

// ShadowCredentialHash is the response from the shadow credential hash endpoint.
type ShadowCredentialHash struct {
	CredentialHash string `json:"credential_hash"`
}

// UserProfile is the response from the user profile endpoint.
type UserProfile struct {
	UserID      string `json:"user_id"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
}
