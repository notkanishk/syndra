package merge

import (
	"go/ast"
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

// ── 2. The outcome list cannot rot ──────────────────────────────────────────
//
// Every policy check below iterates `AllOutcomes`, so a check is only as
// complete as that list. A list kept by hand is a list that silently stops
// covering things — which is exactly what the first version of this file did
// while claiming otherwise: it repeated today's outcomes in a local slice, so a
// new constant would not have failed anything.
//
// Derived from this package's own `const` block instead, the same way
// `addonop` derives its coverage from `addons.AllOutcomes`.
func TestAllOutcomesIsEveryDeclaredOutcome(t *testing.T) {
	declared := declaredOutcomes(t)
	if len(declared) == 0 {
		t.Fatal("no Outcome constants found in the source — this guard has lost its subject")
	}

	listed := map[Outcome]bool{}
	for _, o := range AllOutcomes {
		listed[o] = true
	}
	for _, o := range declared {
		if !listed[o] {
			t.Errorf("outcome %q is declared and missing from AllOutcomes. Every policy "+
				"check in this file iterates that list, so an outcome absent from it is "+
				"an outcome nothing checks.", o)
		}
	}
	for o := range listed {
		if !containsOutcome(declared, o) {
			t.Errorf("AllOutcomes carries %q, which is not a declared constant", o)
		}
	}
}

// declaredOutcomes reads the package's own `const` block for every value typed
// Outcome. Source rather than reflection, because an untyped constant nobody
// referenced is invisible at runtime and visible here.
func declaredOutcomes(t *testing.T) []Outcome {
	t.Helper()
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, ".", func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	var out []Outcome
	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			for _, decl := range file.Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.CONST {
					continue
				}
				// A grouped const block carries the type on the first spec and
				// omits it on the rest, so the type is remembered across specs.
				typed := false
				for _, spec := range gen.Specs {
					vs, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					if vs.Type != nil {
						ident, isIdent := vs.Type.(*ast.Ident)
						typed = isIdent && ident.Name == "Outcome"
					}
					if !typed {
						continue
					}
					for _, v := range vs.Values {
						if lit, ok := v.(*ast.BasicLit); ok && lit.Kind == token.STRING {
							out = append(out, Outcome(strings.Trim(lit.Value, `"`)))
						}
					}
				}
			}
		}
	}
	return out
}

func containsOutcome(all []Outcome, want Outcome) bool {
	for _, o := range all {
		if o == want {
			return true
		}
	}
	return false
}

// accountLevel names the outcomes that describe the ACCOUNT rather than one of
// its fields. `Classify` never assigns these to a field and `Findings` reports
// them from `Absent` before it looks at fields at all, so the checks below have
// to ask about them differently rather than skip them.
var accountLevel = map[Outcome]bool{DeletedUpstream: true}

// ── 3. Auto-resolution has a ceiling, and it is not "anything unambiguous" ──
//
// `fast_forward` resolves unattended because it means Syndra moved and the
// target did not — the system applying its own decision. That reasoning does
// NOT generalise. Git auto-merges whatever does not textually conflict; access
// control must not, because the thing being merged is authority. A rule of
// "auto-resolve when the merge is unambiguous" would silently ratify privilege
// somebody obtained outside the system.
//
// The permitted set is declared here as an INTENTION, separate from the switch
// that implements it, so widening the switch fails. Completeness comes from
// `AllOutcomes`, so a new outcome fails too.
func TestOnlyTheseOutcomesEverResolveUnattended(t *testing.T) {
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

	for _, o := range AllOutcomes {
		if _, answered := permitted[o]; !answered {
			t.Fatalf("outcome %q has no answer to 'may an unattended pass act on it'. "+
				"That is a decision somebody makes in a diff a reviewer reads, not a "+
				"default it inherits.", o)
		}
	}
	if len(permitted) != len(AllOutcomes) {
		t.Errorf("the policy table has %d entries for %d outcomes — one of them answers "+
			"for something that cannot occur", len(permitted), len(AllOutcomes))
	}

	for _, o := range AllOutcomes {
		if accountLevel[o] {
			continue // asserted below; these never appear as a field
		}
		s := Subject{SubjectID: "u1", Fields: []FieldOutcome{{Field: "group", Outcome: o}}}
		if got := s.Convergeable(); got != permitted[o] {
			t.Errorf("outcome %q: convergeable=%v, want %v. Widening this set is how an "+
				"unattended pass starts granting access nobody approved.", o, got, permitted[o])
		}
	}

	absent := Subject{SubjectID: "u1", Absent: true}
	if absent.Convergeable() {
		t.Error("a subject whose account is gone must never converge unattended: the only " +
			"way to 'resolve' it is to create the account somebody deleted")
	}
}

// ── 4. Nothing may be neither resolved nor surfaced ─────────────────────────
//
// The hole the enumeration above does not close on its own. An outcome that
// does not converge and does not raise a finding is a difference the system has
// quietly decided to forget, which is worse than either resolving it or
// reporting it. `deleted_upstream` is exactly that if it ever reaches a FIELD —
// unreachable, because `Classify` only ever sets it on the subject, and this is
// where that stops being an accident and becomes a declared property.
//
// The other direction too: an outcome that BOTH converges and raises a finding
// would be resolved by a pass and leave its row standing, which reads as a
// decision that did not take.
func TestEveryOutcomeEitherResolvesOrSurfaces(t *testing.T) {
	for _, o := range AllOutcomes {
		if accountLevel[o] {
			absent := Subject{SubjectID: "u1", Absent: true}
			found := absent.Findings()
			if len(found) != 1 || found[0].Outcome != o {
				t.Errorf("account-level outcome %q must raise its own finding, got %v", o, found)
			}
			if absent.Convergeable() {
				t.Errorf("account-level outcome %q raises a finding and also converges", o)
			}
			continue
		}

		s := Subject{SubjectID: "u1", Fields: []FieldOutcome{{Field: "group", Outcome: o}}}
		converges, surfaces := s.Convergeable(), len(s.Findings()) > 0
		switch {
		case !converges && !surfaces:
			t.Errorf("outcome %q neither converges nor raises a finding. A difference that "+
				"is not resolved and not reported has been forgotten. Either let a pass act "+
				"on it, or make it a finding, or declare it account-level.", o)
		case converges && surfaces:
			t.Errorf("outcome %q both converges and raises a finding. A pass would resolve "+
				"it and leave the row standing, which reads as a decision that did not take.", o)
		}
	}
}
