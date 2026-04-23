// Command mkauth-token prints a freshly minted Zitadel M2M access token to
// stdout. Intended to be invoked by the operator-facing Actions v2 shell
// scripts (`zitadel/actions/register.sh`, `zitadel/actions/rotate.sh`) when
// ZITADEL_MACHINE_KEY_PATH is set instead of a pre-minted ZITADEL_M2M_TOKEN.
//
// Usage:
//
//	cd backend
//	ZITADEL_DOMAIN=... ZITADEL_MACHINE_KEY_PATH=... go run ./cmd/mkauth-token
//
// Exit codes:
//
//	0 — token printed to stdout, trailing newline.
//	1 — missing/invalid env; load, assertion-build, or token exchange failed.
//	    Error message on stderr.
//
// Reads ZITADEL_DOMAIN and ZITADEL_MACHINE_KEY_PATH from the environment. No
// other side effects — no DB, no Redis, no ambient state. Safe to run from
// an operator host that does not have the MkAuth runtime dependencies
// installed, as long as the Go toolchain and the SA key file are reachable.
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"mkauth/internal/zitadel"
)

const tokenMintTimeout = 30 * time.Second

func main() {
	domain := os.Getenv("ZITADEL_DOMAIN")
	keyPath := os.Getenv("ZITADEL_MACHINE_KEY_PATH")

	if domain == "" {
		fmt.Fprintln(os.Stderr, "mkauth-token: ZITADEL_DOMAIN is required (set in .env or export)")
		os.Exit(1)
	}
	if keyPath == "" {
		fmt.Fprintln(os.Stderr, "mkauth-token: ZITADEL_MACHINE_KEY_PATH is required (set in .env or export)")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), tokenMintTimeout)
	defer cancel()

	token, err := zitadel.MintM2MToken(ctx, domain, keyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mkauth-token: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(token)
}
