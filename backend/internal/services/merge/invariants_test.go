package merge

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The two properties this package must not lose, asserted rather than assumed.
//
// It arrived as part of the TrueNAS work and is the reusable half of it: three
// values, one verdict, no I/O. The next add-on should get it for free, and the
// day it does not is the day one of these fails.

// ── 1. It belongs to the platform, not to a target ──────────────────────────
//
// `Classify` is a pure function over three maps. Nothing in it may learn the
// name of a target, the name of a field a particular target manages, or the
// vocabulary of any one product. The moment it does, the second add-on needs a
// second classifier, and the two will disagree about what a conflict is.
func TestTheClassifierKnowsNoTargetByName(t *testing.T) {
	// Products and protocols this repo integrates with or plausibly will.
	forbidden := []string{
		"truenas", "zitadel", "smb", "nfs", "ldap", "lldap",
		"zfs", "samba", "proxmox", "google", "workspace",
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		checked++
		lower := strings.ToLower(string(body))
		for _, word := range forbidden {
			if strings.Contains(lower, word) {
				t.Errorf("%s mentions %q. This package is the merge model for EVERY target; "+
					"a name here is the first step to a second classifier that disagrees "+
					"with this one about what a conflict is.", name, word)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no source files read — this guard has stopped looking at anything")
	}
}

// And it does no I/O. A classifier that can read is a classifier somebody will
// make read something target-specific.
func TestTheClassifierReachesNothing(t *testing.T) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	// Everything a pure function over three maps could possibly need.
	allowed := map[string]bool{
		"bytes": true, "encoding/json": true, "sort": true, "strings": true, "fmt": true,
	}
	for _, pkg := range pkgs {
		for path, file := range pkg.Files {
			for _, imp := range file.Imports {
				name := strings.Trim(imp.Path.Value, `"`)
				if !allowed[name] {
					t.Errorf("%s imports %q. This package decides; it does not fetch, "+
						"store, or call. Everything it needs arrives as an argument.",
						filepath.Base(path), name)
				}
			}
		}
	}
}

// ── 2. Auto-resolution has a ceiling, and it is not "anything unambiguous" ──
//
// `fast_forward` resolves unattended because it means Syndra moved and the
// target did not — the system applying its own decision. That reasoning does
// NOT generalise. Git auto-merges whatever does not textually conflict; access
// control must not, because the thing being merged is authority. A rule of
// "auto-resolve when the merge is unambiguous" would silently ratify privilege
// somebody obtained outside the system.
//
// So the convergeable set is enumerated here as an intention, and a new outcome
// joining it has to be added deliberately, in a diff a reviewer sees.
func TestOnlyTheseOutcomesEverResolveUnattended(t *testing.T) {
	// Every outcome the classifier can produce, and whether a pass with nobody
	// watching may act on it.
	permitted := map[Outcome]bool{
		// Nobody moved. Acting is a no-op.
		Unchanged: true,
		// Syndra moved, the target did not. The system applying its own
		// decision — the one case where unattended action adds no authority.
		FastForward: true,
		// Both moved to the same value. The target already agrees; recording
		// the base grants nothing that was not already true.
		AlreadyMerged: true,
		// Never observed, so no difference can be attributed. Converges exactly
		// as it did before a base existed, which is the pre-merge behaviour and
		// not a new licence.
		NoBase: true,

		// ─ and these never do ─
		//
		// The target moved and Syndra did not: a hand edit. Resolving it
		// unattended is the silent revert this whole mechanism exists to stop.
		TheirsOnly: false,
		// Both moved differently. There is no answer that is not a choice.
		Conflict: false,
		// The account is gone. The only outcome a sweep could "resolve" by
		// CREATING something, which is the worst thing an unattended pass could
		// decide to do.
		DeletedUpstream: false,
	}

	all := []Outcome{Unchanged, FastForward, AlreadyMerged, TheirsOnly, Conflict, NoBase, DeletedUpstream}
	for _, o := range all {
		if _, listed := permitted[o]; !listed {
			t.Fatalf("outcome %q is not accounted for. A new outcome must be an explicit "+
				"decision about whether a pass with nobody watching may act on it.", o)
		}
	}

	for _, o := range all {
		if o == DeletedUpstream {
			continue // account-level; asserted separately below
		}
		s := Subject{SubjectID: "u1", Fields: []FieldOutcome{{Field: "group", Outcome: o}}}
		if got := s.Convergeable(); got != permitted[o] {
			t.Errorf("outcome %q: convergeable=%v, want %v. Widening this set is how an "+
				"unattended pass starts granting access nobody approved.", o, got, permitted[o])
		}
	}

	// The account-level one, which bypasses the field loop.
	absent := Subject{SubjectID: "u1", Absent: true}
	if absent.Convergeable() {
		t.Error("a subject whose account is gone must never converge unattended: the only " +
			"way to 'resolve' it is to create the account somebody deleted")
	}
}

// Every outcome that cannot converge must produce a FINDING. An outcome that
// neither resolves nor surfaces is a difference the system has decided to
// forget, which is worse than either.
func TestEveryUnresolvableOutcomeSurfaces(t *testing.T) {
	for _, o := range []Outcome{TheirsOnly, Conflict} {
		s := Subject{SubjectID: "u1", Fields: []FieldOutcome{{Field: "group", Outcome: o}}}
		if len(s.Findings()) != 1 {
			t.Errorf("outcome %q converges nowhere and raises nothing — it has been "+
				"silently dropped", o)
		}
	}
	absent := Subject{SubjectID: "u1", Absent: true}
	if f := absent.Findings(); len(f) != 1 || f[0].Outcome != DeletedUpstream {
		t.Errorf("a missing account must raise its own finding, got %v", f)
	}
}
