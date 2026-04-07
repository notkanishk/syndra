package zitadel

import (
	"context"
	"fmt"
	"os"

	"github.com/zitadel/zitadel-go/v3/pkg/client"
	"github.com/zitadel/zitadel-go/v3/pkg/client/management"
)

var ManagementClient *management.Client

// InitClient establishes the Service Account connection to Zitadel's API
func InitClient() error {
	domain := os.Getenv("ZITADEL_DOMAIN")
	keyPath := os.Getenv("ZITADEL_MACHINE_KEY_PATH")

	if domain == "" || keyPath == "" {
		fmt.Println("ZITADEL_DOMAIN or ZITADEL_MACHINE_KEY_PATH not fully provided; skipping Zitadel client init.")
		return nil
	}

	// Create Zitadel client using Service Account JSON Path
	c, err := client.New(
		client.WithDomain(domain),
		client.WithMachineKeyPath(keyPath),
	)
	if err != nil {
		return fmt.Errorf("failed to initialize Zitadel client: %w", err)
	}

	// Construct the Management API client
	mgmt, err := management.NewClient(context.Background(), c)
	if err != nil {
		return fmt.Errorf("failed to initialize management API: %w", err)
	}

	ManagementClient = mgmt
	fmt.Println("Connected to Zitadel API successfully.")
	return nil
}
