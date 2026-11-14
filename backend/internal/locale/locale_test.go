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
	} {
		if got, err := load(tc.in); err != nil || got != tc.want {
			t.Errorf("load(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
		}
	}
}

func TestLoadRefusesWhatIsNotATag(t *testing.T) {
	for _, in := range []string{"en_GB", "english", "en-GB-", "not a tag", "en-GBR1234567"} {
		if _, err := load(in); err == nil || !strings.Contains(err.Error(), in) {
			t.Errorf("load(%q) err = %v; want a refusal naming it", in, err)
		}
	}
}
