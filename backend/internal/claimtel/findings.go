package claimtel

import (
	"strings"
	"time"
)

// Findings: one row per condition (kind, subject), never one per event, with
// when it was first and last seen, how often, and one sample. The correlator
// and the configuration reconciler share the table (claim_findings).

// Finding kinds.
const (
	KindNotCalled     = "CLAIMS_NOT_CALLED"     // subject: the client id
	KindNotServed     = "CLAIMS_NOT_SERVED"     // subject: the client id; sample.outcome
	KindEventsMissing = "SIGNIN_EVENTS_MISSING" // subject: "-"
	KindConfigDrift   = "CONFIG_DRIFT"          // subject: "R1:endpoint", "R5:<app id>", …
	KindCannotCheck   = "CANNOT_CHECK"          // subject: the check that could not read, "R1"
)

// ResolveAfter is how many clean observations in a row settle a finding. A
// sign-in can pass by luck, so the correlator wants three; a configuration
// read is deterministic, so one clean read settles it.
const (
	ResolveAfterSignIns = 3
	ResolveAfterReads   = 1
)

// Observation is one look at a condition: it held (Fail) or it did not.
type Observation struct {
	Kind, Subject string
	Fail          bool
	At            time.Time
	Sample        map[string]any
}

// FindingUpdate is a batch of observations of one (kind, subject), folded.
type FindingUpdate struct {
	Kind, Subject       string
	Fails               int64
	FirstFail, LastFail time.Time
	Sample              map[string]any // the last failure's
	// TrailingClean is the clean observations after the last failure, or all
	// of them when none failed.
	TrailingClean int
}

// Fold collapses observations into one update per (kind, subject), in the
// order each first appears. Observations must be in time order.
func Fold(obs []Observation) []FindingUpdate {
	idx := map[[2]string]int{}
	var out []FindingUpdate
	for _, o := range obs {
		k := [2]string{o.Kind, o.Subject}
		i, ok := idx[k]
		if !ok {
			i = len(out)
			idx[k] = i
			out = append(out, FindingUpdate{Kind: o.Kind, Subject: o.Subject})
		}
		u := &out[i]
		if !o.Fail {
			u.TrailingClean++
			continue
		}
		if u.Fails == 0 {
			u.FirstFail = o.At
		}
		u.Fails++
		u.LastFail, u.Sample, u.TrailingClean = o.At, o.Sample, 0
	}
	return out
}

// Observe turns checked sign-ins into observations. A sign-in that was
// called is a clean look at CLAIMS_NOT_CALLED for its app; one that was
// delivered is clean for CLAIMS_NOT_SERVED too. A first sign-in before access
// is counted, not alarmed, and says nothing about the app either way.
func Observe(checked []Checked) []Observation {
	var obs []Observation
	for _, ch := range checked {
		sample := map[string]any{"sign_in_at": ch.At.UTC(), "user_id": ch.UserID, "expected": ch.Expected}
		switch ch.Verdict {
		case NotCalled:
			sample["missing"] = ch.Missing
			obs = append(obs, Observation{Kind: KindNotCalled, Subject: ch.ClientID, Fail: true, At: ch.At, Sample: sample})
		case NotServed:
			sample["outcome"] = string(ch.Outcome)
			obs = append(obs,
				Observation{Kind: KindNotCalled, Subject: ch.ClientID, At: ch.At},
				Observation{Kind: KindNotServed, Subject: ch.ClientID, Fail: true, At: ch.At, Sample: sample})
		case Delivered:
			obs = append(obs,
				Observation{Kind: KindNotCalled, Subject: ch.ClientID, At: ch.At},
				Observation{Kind: KindNotServed, Subject: ch.ClientID, At: ch.At})
		case FirstSignIn:
			obs = append(obs, Observation{Kind: KindNotCalled, Subject: ch.ClientID, At: ch.At})
		}
	}
	return obs
}

// FindingSeverity is how /healthz/claims reads an open finding. Hard on
// occurrence: Zitadel not reaching Syndra, any configuration drift, and a
// call refused for a hard reason. Soft: a degraded answer, a check that could
// not read, and the detector going blind.
func FindingSeverity(kind string, sample map[string]any) Severity {
	switch kind {
	case KindNotCalled, KindConfigDrift:
		return Hard
	case KindNotServed:
		if o, _ := sample["outcome"].(string); Outcome(o).Severity() == Hard {
			return Hard
		}
		return Soft
	}
	return Soft
}

// FindingReason names an open finding in /healthz/claims without any id: the
// kind, and for a configuration finding its check ("CONFIG_DRIFT:R2").
func FindingReason(kind, subject string) string {
	if kind == KindConfigDrift || kind == KindCannotCheck {
		check, _, _ := strings.Cut(subject, ":")
		return kind + ":" + check
	}
	return kind
}
