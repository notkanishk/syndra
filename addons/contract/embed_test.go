package contract

import (
	"io/fs"
	"testing"
)

// What the kit reads must be inside the embedded tree.
func TestTheEmbeddedTreeCarriesWhatTheKitReads(t *testing.T) {
	for _, p := range []string{"transport_derivation.json", "v2/schema/manifest.json", "v2/schema/common/field.json",
		"v2/registry/errors.json", "v2/registry/constants.json", "v2/openapi.yaml"} {
		if _, err := fs.Stat(FS(), p); err != nil {
			t.Errorf("%s: %v", p, err)
		}
	}
	if _, err := fs.Stat(FS(), "v1"); err == nil {
		t.Error("v1 is embedded; the kit checks one major")
	}
}
