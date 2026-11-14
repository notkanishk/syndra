package conformance

import (
	"os"
	"testing"

	"syndra/addons/reference"
)

// The kit's self-tests host the reference add-on in-process and state its zone
// (test config; no zone is written in the add-on): a zone that is not UTC, so a
// "UTC days" defect differs from the add-on's own days.
func TestMain(m *testing.M) {
	reference.UseZone("Asia/Kolkata")
	os.Exit(m.Run())
}
