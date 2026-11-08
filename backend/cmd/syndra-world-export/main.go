// Command syndra-world-export writes a copy of a live Zitadel's users,
// projects, roles, applications and grants to stdout as a world file, for
// staging's stand-in (ZITADEL_FAKE_WORLD).
//
// It only reads: zitadel.ExportWorld takes a zitadel.WorldReader, which has no
// write method to call. Run on production, nightly:
//
//	docker compose exec -T backend ./syndra-world-export > world.json
//
// Reads ZITADEL_DOMAIN and ZITADEL_MACHINE_KEY_PATH like the backend. Exit 1
// with nothing on stdout when any read fails: a half world is never written.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"syndra/internal/zitadel"
)

func main() {
	if os.Getenv("ZITADEL_DOMAIN") == "" || os.Getenv("ZITADEL_FAKE_WORLD") != "" {
		fail("needs ZITADEL_DOMAIN and ZITADEL_MACHINE_KEY_PATH, and no ZITADEL_FAKE_WORLD")
	}
	if err := zitadel.InitClient(); err != nil {
		fail(err.Error())
	}
	if zitadel.MgmtClient == nil {
		fail("needs ZITADEL_DOMAIN and ZITADEL_MACHINE_KEY_PATH")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	w, err := zitadel.ExportWorld(ctx, zitadel.MgmtClient)
	if err != nil {
		fail(err.Error())
	}
	b, err := json.Marshal(w)
	if err != nil {
		fail(err.Error())
	}
	fmt.Fprintf(os.Stderr, "syndra-world-export: %d users, %d projects, %d grants\n", len(w.Users), len(w.Projects), len(w.Grants))
	os.Stdout.Write(append(b, '\n'))
}

func fail(msg string) {
	fmt.Fprintln(os.Stderr, "syndra-world-export:", msg)
	os.Exit(1)
}
