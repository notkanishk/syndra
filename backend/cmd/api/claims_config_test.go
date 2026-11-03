package main

import (
	"strings"
	"testing"
)

// §4f: a claims configuration that would miss silently refuses the start.
func TestClaimsConfigProblems(t *testing.T) {
	key := "AbCdEfGh12345678AbCdEfGh12345678"
	live := map[string]string{"ZITADEL_DOMAIN": "zitadel.example.edu", "ZITADEL_ACTION_SIGNING_KEY": key, "SYNDRA_EXTERNAL_URL": "https://syndra.example.edu"}
	with := func(over map[string]string) map[string]string {
		out := map[string]string{}
		for k, v := range live {
			out[k] = v
		}
		for k, v := range over {
			out[k] = v
		}
		return out
	}
	canary := map[string]string{"SYNDRA_CANARY_KEY_PATH": "/etc/syndra/canary-key.json", "SYNDRA_CANARY_PROJECT_ID": "123", "SYNDRA_CANARY_CLIENT_ID": "456"}
	cases := []struct {
		name string
		env  map[string]string
		want string // a fragment of the one problem, "" for none
	}{
		{"live and right", live, ""},
		{"live with the canary set", with(canary), ""},
		{"demo mode, nothing set", map[string]string{}, ""},
		{"demo mode, a local http URL", map[string]string{"SYNDRA_EXTERNAL_URL": "http://localhost:8080"}, ""},
		{"a short key", with(map[string]string{"ZITADEL_ACTION_SIGNING_KEY": "short"}), "under 32"},
		{"a key with a pasted newline", with(map[string]string{"ZITADEL_ACTION_SIGNING_KEY": key + "\n"}), "non-printable"},
		{"a key with a space", with(map[string]string{"ZITADEL_ACTION_SIGNING_KEY": key[:16] + " " + key[16:]}), "non-printable"},
		{"live, no external URL", with(map[string]string{"SYNDRA_EXTERNAL_URL": ""}), "SYNDRA_EXTERNAL_URL is empty"},
		{"live, http", with(map[string]string{"SYNDRA_EXTERNAL_URL": "http://syndra.example.edu"}), "not https"},
		{"not a URL", with(map[string]string{"SYNDRA_EXTERNAL_URL": "syndra.example.edu"}), "not an http(s) URL"},
		{"the canary half set", with(map[string]string{"SYNDRA_CANARY_PROJECT_ID": "123"}), "half set"},
		{"the canary two of three", with(map[string]string{"SYNDRA_CANARY_PROJECT_ID": "123", "SYNDRA_CANARY_CLIENT_ID": "456"}), "SYNDRA_CANARY_KEY_PATH not"},
		{"the canary without Zitadel", canary, "needs ZITADEL_DOMAIN"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := claimsConfigProblems(func(k string) string { return tc.env[k] })
			switch {
			case tc.want == "" && len(got) != 0:
				t.Fatalf("refused a good config: %v", got)
			case tc.want != "" && (len(got) != 1 || !strings.Contains(got[0], tc.want)):
				t.Fatalf("got %v, want one problem naming %q", got, tc.want)
			}
			for _, p := range got {
				if strings.Contains(p, key) {
					t.Fatalf("a problem quotes the key: %q", p)
				}
			}
		})
	}
}
