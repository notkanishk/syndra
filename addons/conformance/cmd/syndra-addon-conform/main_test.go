package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A hook that hangs is killed at its timeout, a child holding its output
// included, and fails; the kit then goes on to the matching start.
func TestAHangingHookTimesOut(t *testing.T) {
	began := time.Now()
	err := shell("-stop-app", "sleep 30 & sleep 30", 200*time.Millisecond)(context.Background())
	if err == nil {
		t.Fatal("a hook that hangs returned no error")
	}
	if d := time.Since(began); d > 5*time.Second {
		t.Fatalf("the hook returned after %s, want its timeout", d)
	}
}

// A hook's command runs, and a failing one says so.
func TestAHookRunsItsCommand(t *testing.T) {
	f := filepath.Join(t.TempDir(), "down")
	if err := shell("-stop-app", "touch '"+f+"'; exit 1", time.Minute)(context.Background()); err == nil {
		t.Error("a hook that exits 1 returned no error")
	}
	if _, err := os.Stat(f); err != nil {
		t.Errorf("the hook did not run: %v", err)
	}
	if shell("-stop-app", " ", time.Minute) != nil {
		t.Error("an empty command is a hook")
	}
}
