package main

import "testing"

func TestLiveAuthConfigProblems(t *testing.T) {
	cases := []struct {
		name                     string
		domain, audience, apiKey string
		want                     int
	}{
		{"demo mode is left alone", "", "", "dev_auth_token_secret", 0},
		{"live and configured", "auth.example.com", "syndra", "a-real-key", 0},
		{"live, no audience", "auth.example.com", "", "a-real-key", 1},
		{"live, dev key", "auth.example.com", "syndra", "dev_auth_token_secret", 1},
		{"live, both wrong", "auth.example.com", "", "dev_auth_token_secret", 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ZITADEL_DOMAIN", tc.domain)
			t.Setenv("ZITADEL_AUDIENCE", tc.audience)
			t.Setenv("SYNDRA_API_KEY", tc.apiKey)
			if got := liveAuthConfigProblems(); len(got) != tc.want {
				t.Fatalf("got %v, want %d problems", got, tc.want)
			}
		})
	}
}
