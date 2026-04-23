package zitadel

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
)

// ServiceAccountKey represents a Zitadel machine user key file.
type ServiceAccountKey struct {
	Type   string `json:"type"`
	KeyID  string `json:"keyId"`
	Key    string `json:"key"`
	UserID string `json:"userId"`
}

// LoadServiceAccountKey reads and validates a Zitadel service account key file.
// Returns the parsed key metadata and the RSA private key for JWT signing.
func LoadServiceAccountKey(path string) (*ServiceAccountKey, *rsa.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("read key file: %w", err)
	}

	if len(data) == 0 {
		return nil, nil, fmt.Errorf("key file is empty: %s (0 bytes) — check that ZITADEL_MACHINE_KEY_PATH in .env points to the real JSON file (relative paths resolve against the docker-compose directory) and that the compose bind mount is not falling through to /dev/null", path)
	}

	var sa ServiceAccountKey
	if err := json.Unmarshal(data, &sa); err != nil {
		return nil, nil, fmt.Errorf("decode key file %s (%d bytes): %w", path, len(data), err)
	}

	if sa.Type != "serviceaccount" {
		return nil, nil, fmt.Errorf("unexpected key type %q; expected \"serviceaccount\"", sa.Type)
	}
	if sa.KeyID == "" || sa.Key == "" || sa.UserID == "" {
		return nil, nil, fmt.Errorf("key file missing required fields (keyId, key, userId)")
	}

	block, _ := pem.Decode([]byte(sa.Key))
	if block == nil {
		return nil, nil, fmt.Errorf("key file contains invalid PEM data")
	}

	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 as fallback — Zitadel may emit either format.
		parsed, err2 := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err2 != nil {
			return nil, nil, fmt.Errorf("parse private key: %w (pkcs8: %v)", err, err2)
		}
		rsaKey, ok := parsed.(*rsa.PrivateKey)
		if !ok {
			return nil, nil, fmt.Errorf("key file contains non-RSA private key")
		}
		privKey = rsaKey
	}

	return &sa, privKey, nil
}
