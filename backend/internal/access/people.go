package access

import "strings"

// Left reports whether a directory status means the person is no longer a
// member. The one definition: bulk refusals, the "left" insights group, the
// usage report and the mapping cohort all ask it. There were two lists, and
// they disagreed on "alumni" and "deactivated".
//
// What Zitadel emits (directory.normalizeUserState) is active, inactive,
// initial, locked, deleted, unspecified. "departed", "alumni" and
// "deactivated" are kept for a directory that is not Zitadel, or a fixture;
// they cost nothing. "initial" is NOT here: invited and not yet signed in is a
// person arriving, not one who has gone.
//
// Only a positive report counts. An unknown or empty status is a person still
// here, so a failed directory read can never drop everyone from a count.
func Left(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "inactive", "locked", "deleted", "departed", "alumni", "deactivated":
		return true
	}
	return false
}
