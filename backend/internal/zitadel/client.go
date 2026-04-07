package zitadel

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/zitadel/zitadel-go/v3/pkg/client"
	"github.com/zitadel/zitadel-go/v3/pkg/client/management"
	"github.com/zitadel/zitadel-go/v3/pkg/zitadel"
)

var MgmtClient *management.Client

// InitClient establishes the Service Account connection to Zitadel's Management API
func InitClient() error {
	domain := os.Getenv("ZITADEL_DOMAIN")
	keyPath := os.Getenv("ZITADEL_MACHINE_KEY_PATH")

	if domain == "" || keyPath == "" {
		fmt.Println("ZITADEL_DOMAIN or ZITADEL_MACHINE_KEY_PATH not fully provided; skipping strict Zitadel client injection.")
		return nil
	}

	conf := zitadel.New(domain)
	
	// Create a Management Service Client utilizing the Service Account JSON Key
	c, err := management.NewClient(
		conf,
		client.WithAuth(client.DefaultServiceAccount(keyPath)),
	)
	if err != nil {
		return fmt.Errorf("failed to construct zitadel management client: %v", err)
	}

	MgmtClient = c
	log.Println("Zitadel Management Client initialized successfully.")
	return nil
}
