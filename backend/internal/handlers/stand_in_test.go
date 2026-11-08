package handlers

import (
	"context"
	"errors"
	"slices"
	"testing"

	"syndra/internal/claimcheck"
	"syndra/internal/claimtel"
	"syndra/internal/zitadel"
)

// Staging's stand-in (ZITADEL_FAKE_WORLD) has no actions, event listener or
// claims setup. The checks that read them, and the key rotation, must neither
// raise an alarm nor try against it; against a real Zitadel nothing changes.

func asStandIn(t *testing.T) {
	t.Helper()
	orig := zitadel.MgmtClient
	t.Cleanup(func() { zitadel.MgmtClient = orig })
	zitadel.MgmtClient = &zitadel.WorldClient{}
	if !zitadel.IsStandIn() {
		t.Fatal("the stand-in is not the stand-in")
	}
}

type settled struct{ kind, checks, seen []string }

func captureSettles(t *testing.T) *settled {
	t.Helper()
	s := &settled{}
	dbSettleUnseenClaimFindings = func(_ context.Context, kind string, checks, seen []string) error {
		s.kind = append(s.kind, kind)
		s.checks, s.seen = checks, seen
		if seen == nil {
			t.Error("seen is nil: ANY(NULL) settles nothing")
		}
		return nil
	}
	return s
}

func TestStandIn_ConfigCheckReadsNothingAndSettlesWhatWasRaised(t *testing.T) {
	sink := stubClaimFindings(t)
	keyringPlane(t, &fakeActions{err: errors.New("read must not happen")})
	stubConfigReads(t)
	set := captureSettles(t)
	asStandIn(t)

	if err := RunClaimConfigCheck(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(sink.ups) != 0 {
		t.Fatalf("raised against the stand-in: %v", sink.kinds())
	}
	if !slices.Equal(set.kind, []string{claimtel.KindConfigDrift, claimtel.KindCannotCheck}) ||
		!slices.Equal(set.checks, claimcheck.Report{}.Ran()) || len(set.checks) != 9 || len(set.seen) != 0 {
		t.Fatalf("settled %+v", set)
	}
}

func TestStandIn_Off_ConfigCheckStillRaises(t *testing.T) {
	sink := stubClaimFindings(t)
	keyringPlane(t, &fakeActions{err: errors.New("zitadel down")})
	stubConfigReads(t)
	if err := RunClaimConfigCheck(context.Background()); err != nil {
		t.Fatal(err)
	}
	if sink.kinds()[claimtel.KindCannotCheck+" R1"] == 0 {
		t.Fatalf("a failed read against a real Zitadel raised nothing: %v", sink.kinds())
	}
}

func TestStandIn_KeyringReadsNothing(t *testing.T) {
	sink := stubClaimFindings(t)
	keyringPlane(t, &fakeActions{err: errors.New("zitadel down")})

	asStandIn(t)
	if err := followKeyring(context.Background()); err != nil || len(sink.ups) != 0 {
		t.Fatalf("stand-in: err %v, raised %v", err, sink.kinds())
	}
}

func TestStandIn_Off_KeyringFailureRaises(t *testing.T) {
	sink := stubClaimFindings(t)
	keyringPlane(t, &fakeActions{err: errors.New("zitadel down")})
	if err := followKeyring(context.Background()); err == nil || sink.kinds()[claimtel.KindCannotCheck+" R1"] == 0 {
		t.Fatalf("real Zitadel: err %v, raised %v", err, sink.kinds())
	}
}

// A due key, in the window, with the test sign-in passing: against the
// stand-in nothing is rotated (nothing to rotate); against Zitadel it is.
func TestStandIn_AutoRotationHasNothingToRotate(t *testing.T) {
	g := newRotationRig(t)
	withOldKeyDue(t)
	freshProbe(t, true)

	asStandIn(t)
	if c := autoConfig(); c.On || c.OffReason != "stand_in" {
		t.Fatalf("stand-in config %+v", c)
	}
	if err := RunActionKeyAutoRotate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := g.ev.count("zitadel:rotate:"); n != 0 {
		t.Fatalf("rotated %d keys against the stand-in", n)
	}

	zitadel.MgmtClient = nil // the same rig against a real Zitadel (asStandIn's cleanup restores)

	if c := autoConfig(); !c.On {
		t.Fatalf("real Zitadel config %+v", c)
	}
	if err := RunActionKeyAutoRotate(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := g.ev.count("zitadel:rotate:"); n != 2 {
		t.Fatalf("rotated %d keys against Zitadel, want 2", n)
	}
}

// On the stand-in, operators sign in as the development identities: a demo id
// as an audit row's actor is who signed in, not sample data. Against a real
// Zitadel (and in the demo) it still counts. Demo projects and users as
// targets count either way (db.TestCountDemoResidue_ActorsOnlyWhenAsked_Live).
func TestStandIn_DemoResidueLeavesOutActors(t *testing.T) {
	orig := dbCountDemoResidue
	t.Cleanup(func() { dbCountDemoResidue = orig })
	var gotActors []bool
	dbCountDemoResidue = func(_ context.Context, projects, users []string, actors bool) (int, error) {
		if len(projects) == 0 || len(users) == 0 {
			t.Error("demo projects and users must still be counted as targets")
		}
		gotActors = append(gotActors, actors)
		return 0, nil
	}

	_, _ = countDemoResidue(context.Background())
	asStandIn(t)
	_, _ = countDemoResidue(context.Background())
	if !slices.Equal(gotActors, []bool{true, false}) {
		t.Fatalf("countActors real then stand-in: %v, want [true false]", gotActors)
	}
}
