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

// Language is Tag as a language.Tag, for code that collates by it; Und when unset.
func Language() language.Tag {
	if Tag == "" {
		return language.Und
	}
	return language.MustParse(Tag)
}

func load(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", nil
	}
	refuse := func(why string) (string, error) {
		return "", fmt.Errorf("LOCALE=%q %s; refusing to start rather than guess a locale", s, why)
	}
	t, err := language.Parse(s)
	if err != nil {
		return refuse("is not a BCP 47 language tag (RFC 5646; for example en-GB)")
	}
	lower := strings.ToLower(s)
	if lower == "und" || strings.HasPrefix(lower, "und-") || lower == "root" || strings.HasPrefix(lower, "x-") {
		return refuse("names no language (the root and private-use-only tags format in nothing); name one, for example en-GB")
	}
	// The UI canonicalises with Intl.getCanonicalLocales, and the two disagree
	// on legacy tags (Go maps tl to fil and mo to ro-MD; ICU keeps tl, maps mo
	// to ro; grandfathered tags like i-klingon, zh-min-nan, en-GB-oed are an
	// error in ICU only). So only a tag already in canonical form is accepted:
	// letter case is normalised, anything else must be written as it is
	// canonical, and then both sides name the same locale.
	if c := t.String(); !strings.EqualFold(c, s) {
		return refuse(fmt.Sprintf("is not in canonical form; write %q", c))
	}
	return t.String(), nil
}
