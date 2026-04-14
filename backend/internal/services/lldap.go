package services

import "strings"

// FlattenLLDAPGroup computes the LLDAP group name from a stable project ID and role key.
// Pattern: lowercase({projectID}_{roleKey}).
// Uses the immutable project ID (not the mutable display name) so that project
// renames never orphan existing LLDAP group memberships or cause collisions.
// Example: "printing" + "member" → "printing_member"
func FlattenLLDAPGroup(projectID, roleKey string) string {
	id := strings.ToLower(projectID)
	key := strings.ToLower(roleKey)
	return id + "_" + key
}
