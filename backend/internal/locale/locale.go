// Package locale is the deployment's display locale (the LOCALE setting): a
// BCP 47 language tag (RFC 5646, for example en-GB) that the UI formats dates
// and numbers in. The backend never formats with it; it only validates it and
// hands it on (/healthz, next to time_zone), so there is one source for it.
package locale

import (
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/text/language"
)

// Tag is the canonical tag from LOCALE, read once at start; "" when LOCALE is
// unset, in which case nothing is sent and the UI uses its runtime's default.
// A LOCALE that is not a well-formed tag refuses to start, as an invalid TZ does.
var Tag = mustLoad(os.Getenv("LOCALE"))

func mustLoad(s string) string {
	tag, err := load(s)
	if err != nil {
		log.Fatalf("[STARTUP] %v", err)
	}
	return tag
}

func load(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	// ParseBase-style leniency would accept "en_GB" and trailing junk; Parse
	// reports a malformed tag as an error, and the underscore form is not RFC 5646.
	t, err := language.Parse(s)
	if err != nil || strings.Contains(s, "_") {
		return "", fmt.Errorf("LOCALE=%q is not a BCP 47 language tag (for example en-GB); refusing to start rather than guess a locale", s)
	}
	return t.String(), nil
}
