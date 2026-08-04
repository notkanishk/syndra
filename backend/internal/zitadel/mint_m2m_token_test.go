package zitadel

import (
	"context"
	"strings"
	"testing"
)

func TestMintM2MToken_RejectsEmptyDomain(t *testing.T) {
	_, err := MintM2MToken(context.Background(), "", "/tmp/nonexistent")
	if err == nil {
		t.Fatal("expected error for empty domain, got nil")
	}
	if !strings.Contains(err.Error(), "domain is required") {
		t.Errorf("expected 'domain is required' in error, got: %v", err)
	}
}

func TestMintM2MToken_RejectsEmptyKeyPath(t *testing.T) {
	_, err := MintM2MToken(context.Background(), "example.zitadel.cloud", "")
	if err == nil {
		t.Fatal("expected error for empty keyPath, got nil")
	}
	if !strings.Contains(err.Error(), "keyPath is required") {
		t.Errorf("expected 'keyPath is required' in error, got: %v", err)
	}
}

func TestMintM2MToken_SurfacesLoadFailure(t *testing.T) {
	// Point at a path that doesn't exist. The error must wrap
	// LoadServiceAccountKey's failure, not swallow it — operators need to
	// see exactly why the mint failed so they can fix their env.
	_, err := MintM2MToken(context.Background(), "example.zitadel.cloud", "/tmp/syndra-nonexistent-"+t.Name())
	if err == nil {
		t.Fatal("expected error for nonexistent key file, got nil")
	}
	if !strings.Contains(err.Error(), "load service account key") {
		t.Errorf("expected wrapping prefix 'load service account key', got: %v", err)
	}
}
