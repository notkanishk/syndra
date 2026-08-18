// Package merge classifies one subject's state on a target as a three-way
// merge: what Syndra wants, what the target has, and what the target reported
// after the last successful apply.
//
// It answers the question reconciliation could not: WHO changed a value. A
// two-way diff produces no conflict, only a winner, and the winner has always
// been Syndra — so a hand edit on a target is silently reverted and nothing
// anywhere records that it happened.
//
// Nothing here writes, reads a database, or calls a target. It is a pure
// function over three maps, because the decisions it encodes are the ones most
// worth testing exhaustively and least worth testing through a sweep.
package merge

import (
	"bytes"
	"encoding/json"
	"sort"
)

// Outcome is what one managed field's three values say happened.
type Outcome string

const (
	// Unchanged: nobody moved. Write nothing.
	Unchanged Outcome = "unchanged"
	// FastForward: Syndra moved and the target did not. The ordinary case, and
	// the only one an unattended pass may resolve by writing.
	FastForward Outcome = "fast_forward"
	// AlreadyMerged: both moved to the same value. Somebody did by hand exactly
	// what Syndra was going to do. Record the base; write nothing.
	AlreadyMerged Outcome = "already_merged"
	// TheirsOnly: the target moved and Syndra did not. This is a hand edit, and
	// it is the most common of these states — the one a two-way diff reports as
	// "the target is wrong" and reverts.
	TheirsOnly Outcome = "theirs_only"
	// Conflict: both moved, differently. Never resolved without a person.
	Conflict Outcome = "conflict"
	// NoBase: nothing was ever observed for this subject, so no cause can be
	// determined for any difference. Not a finding — an absence of evidence —
	// and it converges as it did before this mechanism existed.
	NoBase Outcome = "no_base"
	// DeletedUpstream: the thing the binding names is not on the target. An
	// account-level state rather than a field one, and the only one of these
	// that a sweep could "resolve" by creating something.
	DeletedUpstream Outcome = "deleted_upstream"
)

// SubjectFinding is one difference an unattended pass may not resolve, carrying
// the three values that produced it.
//
// All three, always: "what was it before" is the question an operator asks
// first, and a finding that cannot answer it sends them to read the target's
// own history — which for most targets does not exist.
//
// Field is empty for an account-level finding (`deleted_upstream`), which is
// also what makes the dedup key work: one standing finding per subject, target
// and field, with the account-level one occupying its own slot rather than
// colliding with a field.
type SubjectFinding struct {
	SubjectID string          `json:"subject_id"`
	Field     string          `json:"field,omitempty"`
	Outcome   Outcome         `json:"outcome"`
	Base      json.RawMessage `json:"base,omitempty"`
	Ours      json.RawMessage `json:"ours,omitempty"`
	Theirs    json.RawMessage `json:"theirs,omitempty"`
}

// FieldOutcome is one field's classification with the three values that produced
// it, so a finding can answer "what was it before" without a second read.
type FieldOutcome struct {
	Field   string          `json:"field"`
	Outcome Outcome         `json:"outcome"`
	Base    json.RawMessage `json:"base,omitempty"`
	Ours    json.RawMessage `json:"ours,omitempty"`
	Theirs  json.RawMessage `json:"theirs,omitempty"`
}

// Subject is the whole classification for one subject on one target.
type Subject struct {
	SubjectID string         `json:"subject_id"`
	Fields    []FieldOutcome `json:"fields"`
	// Absent says the account the binding names is no longer on the target.
	// Classified before any field is, because a field comparison against an
	// account that does not exist is a comparison against nothing.
	Absent bool `json:"absent,omitempty"`
}

// Convergeable reports whether an unattended pass may apply this subject.
//
// The rule is stricter than "apply the fast-forward fields", and the reason is
// mechanical rather than philosophical: an apply carries the WHOLE managed set
// for a subject, not one field. A subject with a fast-forward on `group` and a
// conflict on `enabled` cannot have the first applied without the second being
// overwritten — so the sweep converges a subject only when every managed field
// is one an unattended pass may resolve.
//
// The alternative — applying a partial set — is worse than it looks. Desired
// state is level-triggered: the fields it omits are the ones Syndra does not
// manage, so a partial apply would say "Syndra no longer manages `enabled`",
// which is a policy statement nobody made.
func (s Subject) Convergeable() bool {
	if s.Absent {
		// Never. The plan for an absent account says "create", and an
		// unattended pass acting on it recreates an account somebody deleted.
		// This is the case that already bit: stub-era bindings queueing a
		// create every six hours against a production NAS.
		return false
	}
	for _, f := range s.Fields {
		switch f.Outcome {
		case Unchanged, FastForward, AlreadyMerged, NoBase:
		default:
			return false
		}
	}
	return true
}

// NeedsWrite reports whether converging this subject would actually change
// anything, so a pass does not queue work for a subject already in step.
func (s Subject) NeedsWrite() bool {
	for _, f := range s.Fields {
		if f.Outcome == FastForward || f.Outcome == NoBase {
			return true
		}
	}
	return false
}

// Findings are the differences an unattended pass may not resolve.
//
// Three kinds, and all three are findings rather than sweep output: a value the
// target moved and Syndra did not, a value both moved differently, and a thing
// the target no longer has. The first was the one most likely to be left
// ephemeral — it is what a hand edit looks like, and it is the most common of
// the three.
func (s Subject) Findings() []SubjectFinding {
	out := make([]SubjectFinding, 0)
	if s.Absent {
		return append(out, SubjectFinding{SubjectID: s.SubjectID, Outcome: DeletedUpstream})
	}
	for _, f := range s.Fields {
		if f.Outcome == Conflict || f.Outcome == TheirsOnly {
			out = append(out, SubjectFinding{
				SubjectID: s.SubjectID, Field: f.Field, Outcome: f.Outcome,
				Base: f.Base, Ours: f.Ours, Theirs: f.Theirs,
			})
		}
	}
	return out
}

// Classify compares one subject's three states, field by field.
//
// `ours` is the desired state as resolved from policy; `theirs` is what the
// target reports now; `base` is what the target reported after the last
// successful apply, or nil for a subject that has never had one.
//
// MANAGED FIELDS ONLY, and the managed set is `ours`: a field Syndra does not
// manage for this subject is not "unchanged", it is out of scope. Comparing it
// would raise findings about values nobody here decided — the target's own
// business, reported as drift.
func Classify(subjectID string, ours, theirs, base map[string]json.RawMessage) Subject {
	out := Subject{SubjectID: subjectID, Fields: make([]FieldOutcome, 0, len(ours))}
	fields := make([]string, 0, len(ours))
	for f := range ours {
		fields = append(fields, f)
	}
	// Sorted, so a finding's field order is stable across passes and two
	// classifications of the same state are equal documents.
	sort.Strings(fields)

	for _, field := range fields {
		o := ours[field]
		t, seen := theirs[field]
		b, based := base[field]

		fo := FieldOutcome{Field: field, Ours: o, Theirs: t, Base: b}
		switch {
		case !based:
			// No base for this field — either the subject has none at all, or
			// the field was not managed when the last base was recorded. Both
			// mean no cause can be determined, and inventing one would either
			// fabricate agreement or manufacture a conflict.
			fo.Outcome = NoBase
			if seen && sameValue(o, t) {
				// Except when they already agree, which needs no cause: there
				// is nothing to attribute and nothing to write.
				fo.Outcome = Unchanged
			}
		case !seen:
			// The target no longer reports a field it once did. Treated as
			// theirs-only rather than as absent state: something changed there
			// and Syndra did not do it, which is exactly what that outcome
			// means.
			fo.Outcome = TheirsOnly
		case sameValue(o, t):
			// They agree. Whether they were always this way or both moved here
			// decides whether the base needs updating, and nothing else.
			if sameValue(b, t) {
				fo.Outcome = Unchanged
			} else {
				// Checked BEFORE theirs-only, deliberately: somebody who made
				// the change Syndra was going to make has not drifted, and
				// telling them they have is how a system trains people to
				// ignore it.
				fo.Outcome = AlreadyMerged
			}
		case sameValue(b, t):
			// The target still holds what it last reported; Syndra moved.
			fo.Outcome = FastForward
		case sameValue(b, o):
			// Syndra still wants what it last saw; the target moved.
			fo.Outcome = TheirsOnly
		default:
			// Both moved, differently, and nothing here can say which is right.
			fo.Outcome = Conflict
		}
		out.Fields = append(out.Fields, fo)
	}
	return out
}

// Absent is the classification for a binding whose account is not on the target.
//
// Its own constructor rather than a flag threaded through Classify, because
// there are no field values to compare: the account is gone, and a comparison
// against nothing would report every managed field as changed by somebody.
func Absent(subjectID string) Subject {
	return Subject{SubjectID: subjectID, Absent: true}
}

// sameValue compares two JSON values for MEANING, not for bytes.
//
// Set semantics for arrays, because `group` is a set: `["a","b"]` and
// `["b","a"]` are the same membership, and a target that returns them in its own
// order would otherwise produce a conflict on every pass — which is the shape of
// false finding that empties a triage queue of its credibility.
//
// Everything else compares by canonical encoding, so `true` equals `true` and
// key order in an object does not matter.
func sameValue(a, b json.RawMessage) bool {
	if len(a) == 0 || len(b) == 0 {
		return len(a) == len(b)
	}
	ca, aok := canonical(a)
	cb, bok := canonical(b)
	if !aok || !bok {
		// One of them is not valid JSON. Compared as raw bytes rather than
		// declared equal: an unparseable value is not evidence of agreement.
		return bytes.Equal(a, b)
	}
	return bytes.Equal(ca, cb)
}

func canonical(raw json.RawMessage) ([]byte, bool) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, false
	}
	if list, ok := v.([]any); ok {
		encoded := make([]string, 0, len(list))
		for _, item := range list {
			e, err := json.Marshal(item)
			if err != nil {
				return nil, false
			}
			encoded = append(encoded, string(e))
		}
		sort.Strings(encoded)
		out, err := json.Marshal(encoded)
		if err != nil {
			return nil, false
		}
		return out, true
	}
	out, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	return out, true
}
