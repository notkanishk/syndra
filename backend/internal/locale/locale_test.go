package locale

import (
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", ""}, // unset: nothing is sent
		{"  ", ""},
		{"en-GB", "en-GB"},
		{" en-gb ", "en-GB"}, // canonical case
		{"de", "de"},
		{"sr-Latn-RS", "sr-Latn-RS"},
		{"th-TH-u-ca-gregory", "th-TH-u-ca-gregory"}, // full BCP 47: extensions
		{"en-GB-u-hc-h23", "en-GB-u-hc-h23"},
		{"EN-latn-us", "en-Latn-US"},
	} {
		if got, err := load(tc.in); err != nil || got != tc.want {
			t.Errorf("load(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
}

func TestLoadRefusesWhatIsNotATag(t *testing.T) {
	for _, in := range []string{"en_GB", "english", "en-GB-", "not a tag", "en-abcdefghij", "he-IL-u-ca-hebrew-ca-gregory"} {
		if _, err := load(in); err == nil || !strings.Contains(err.Error(), in) {
			t.Errorf("load(%q) err = %v; want a refusal naming it", in, err)
		}
	}
}

// The root and private-use-only tags are well formed and name no language.
func TestLoadRefusesTagsThatNameNoLanguage(t *testing.T) {
	for _, in := range []string{"und", "root", "x-private", "X-Whatever", "und-US"} {
		if _, err := load(in); err == nil || !strings.Contains(err.Error(), "no language") {
			t.Errorf("load(%q) err = %v; want a refusal saying it names no language", in, err)
		}
	}
}

// Go and the UI's Intl.getCanonicalLocales disagree on these (checked against
// Node's ICU), so they are refused, with the form to write, never accepted to
// mean two different locales.
func TestLoadRefusesWhereGoAndIntlDisagree(t *testing.T) {
	for _, tc := range []struct{ in, write string }{
		{"tl", "fil"},
		{"mo", "ro-MD"},
		{"iw", "he"},
		{"zh-cmn-Hans-CN", "cmn-Hans-CN"},
		{"i-klingon", "tlh"},
		{"en-GB-oed", "en-GB-oxendict"},
		{"en-u-hc-h23-ca-gregory", "en-u-ca-gregory-hc-h23"},
	} {
		if _, err := load(tc.in); err == nil || !strings.Contains(err.Error(), tc.write) {
			t.Errorf("load(%q) err = %v; want a refusal naming %q", tc.in, err, tc.write)
		}
	}
}
