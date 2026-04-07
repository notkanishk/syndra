package zitadel

import (
	"fmt"
	"log"
	"os"
)

// MgmtClient is a placeholder interface — real Zitadel integration
// uses the Management API gRPC client. Stubbed until service account
// credentials are provisioned in the deployment environment.
var MgmtClient interface{} = nil

// InitClient establishes the Service Account connection to Zitadel's Management API.
// If ZITADEL_DOMAIN or ZITADEL_MACHINE_KEY_PATH are not set, it short-circuits
// gracefully — the system operates in local-policy-only mode.
func InitClient() error {
	domain := os.Getenv("ZITADEL_DOMAIN")
	keyPath := os.Getenv("ZITADEL_MACHINE_KEY_PATH")

	if domain == "" || keyPath == "" {
		fmt.Println("ℹ️  ZITADEL_DOMAIN or ZITADEL_MACHINE_KEY_PATH not set; operating in local-policy-only mode.")
		return nil
	}

	// TODO: Wire up zitadel-go/v3 management.NewClient once service account key is provisioned.
	// The API signature is:
	//   c, err := management.NewClient(ctx, domain, keyPath, scopes, opts...)
	log.Printf("Zitadel domain=%s configured; management client init deferred to Phase 2.", domain)
	return nil
}
