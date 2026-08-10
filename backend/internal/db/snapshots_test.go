package db

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"
)

// `internal/db` has no live-database harness, so these assert what can be
// asserted without one: that every refusal happens BEFORE the transaction is
// touched, and that the statement says what the schema requires.
//
// The ordering is not a stylistic preference here — the transaction handed in is
// nil, so a check placed after the first use of it panics instead of failing,
// which is what makes "refused before anything is written" a real assertion
// rather than a comment.

func TestASnapshotIsRefusedBeforeTheTransactionIsTouched(t *testing.T) {
	for _, tc := range []struct {
		name    string
		subject string
		target  string
		actor   string
		state   map[string]json.RawMessage
		because string
	}{
		{
			name: "no subject", target: "truenas", actor: "op_1",
			state:   map[string]json.RawMessage{},
			because: "a snapshot with no subject records an intent about nobody",
		},
		{
			name: "no target", subject: "u1", actor: "op_1",
			state:   map[string]json.RawMessage{},
			because: "the foreign key would answer this, and it would answer it mid-transaction",
		},
		{
			name: "the built-in target", subject: "u1", target: TargetZitadel, actor: "op_1",
			state:   map[string]json.RawMessage{},
			because: "Zitadel's intent is the outbox row's own columns, so a snapshot is a second copy nothing reads",
		},
		{
			name: "no actor", subject: "u1", target: "truenas",
			state:   map[string]json.RawMessage{},
			because: "an audit record that outlives its plan and cannot say who decided it",
		},
		{
			name: "nil state", subject: "u1", target: "truenas", actor: "op_1",
			because: "nil encodes as JSON null, which the drain reads back as no approved desired state and fails the row on",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := WriteDesiredStateSnapshotTx(context.Background(), nil, tc.subject, tc.target, tc.actor, tc.state)
			if !errors.Is(err, ErrInvalidSnapshot) {
				t.Fatalf("want ErrInvalidSnapshot (%s), got %v", tc.because, err)
			}
		})
	}
}

// An empty set is a legitimate instruction — manage nothing — and must not be
// confused with nil. Asserted by the refusal NOT firing, which is the only way
// to tell a check that is right from one that is merely strict.
func TestAnEmptyDesiredStateIsNotTheSameAsNoneAtAll(t *testing.T) {
	// Reaching the nil transaction IS the assertion: it is the only observable
	// difference between "validated and allowed through" and "refused", in a
	// package with no database to reach.
	defer func() {
		if recover() == nil {
			t.Error("an empty set was refused before the write; empty means manage nothing, which is a legitimate instruction")
		}
	}()
	_, err := WriteDesiredStateSnapshotTx(context.Background(), nil, "u1", "truenas", "op_1", map[string]json.RawMessage{})
	if errors.Is(err, ErrInvalidSnapshot) {
		t.Fatalf("an empty set must reach the database: %v", err)
	}
}

// The insert must not name `version`. The column is allocated by a trigger under
// a pair-scoped lock, and a supplied value is replaced — so naming it would let
// a future edit pass one and believe it took, on the field the stale-version
// rejection compares against.
func TestTheSnapshotInsertDoesNotSupplyItsOwnVersion(t *testing.T) {
	src := readSource(t, "snapshots.go")
	insert := between(t, src, "INSERT INTO desired_state_snapshots", "RETURNING")
	if strings.Contains(insert, "version") {
		t.Errorf("the insert names `version`, which the trigger allocates:\n%s", insert)
	}
	for _, col := range []string{"subject_id", "target", "state_json", "created_by"} {
		if !strings.Contains(insert, col) {
			t.Errorf("the insert does not name %s", col)
		}
	}
	if !strings.Contains(src, "RETURNING id::text, version") {
		t.Error("the writer must read back the allocated version, or nothing can cite it")
	}
}

func readSource(t *testing.T, name string) string {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(raw)
}

func between(t *testing.T, src, from, to string) string {
	t.Helper()
	i := strings.Index(src, from)
	if i < 0 {
		t.Fatalf("source does not contain %q", from)
	}
	rest := src[i:]
	j := strings.Index(rest, to)
	if j < 0 {
		t.Fatalf("source does not contain %q after %q", to, from)
	}
	return rest[:j]
}
