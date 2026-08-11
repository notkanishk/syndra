package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// Anchoring an add-on's mutation log (design §17, change `addon-platform` 2.28).
//
// The log is hash-chained and verifies its own contents. What a chain cannot do
// is notice that it has been TRUNCATED: cut the first thousand records off,
// re-chain from what is left, and every remaining link still verifies. The only
// thing that catches that is an outside observer who remembers where the head
// was — which is this.

// Log-integrity verdicts. A closed vocabulary because it is written to a row an
// operator reads, and because the two violations are different accusations.
const (
	// AnchorFirstSighting — nothing was recorded before. Not a violation: every
	// target has a first read, and treating one as tampering would make a fresh
	// deployment look compromised.
	AnchorFirstSighting = "first_sighting"
	// AnchorExtended — the log grew and its head moved. The healthy case.
	AnchorExtended = "extended"
	// AnchorUnchanged — the log did not move and neither did its head. Also
	// healthy: an add-on that performed no writes reports the same pair.
	AnchorUnchanged = "unchanged"
	// AnchorRecordsDecreased — records that existed are gone.
	AnchorRecordsDecreased = "records_decreased"
	// AnchorHeadRewritten — the same number of records now hash to something
	// else, so the content behind them was rewritten in place.
	AnchorHeadRewritten = "head_rewritten"
)

// ErrLogAnchorViolation is a reported head that is not an extension of the
// recorded one.
//
// Its own error, because its operator action is unlike any other failure here:
// nothing is retried, nothing is fixed by waiting, and the question is who has
// access to the add-on's volume.
var ErrLogAnchorViolation = errors.New("db: the add-on's mutation log is not an extension of what was last anchored")

// LogAnchor is what the backend remembers about one add-on's log.
type LogAnchor struct {
	Target     string    `json:"target"`
	Head       string    `json:"head"`
	Records    int64     `json:"records"`
	AnchoredAt time.Time `json:"anchored_at"`
	// The violation fields are set exactly when the anchor refused to move.
	ViolationReason  string     `json:"violation_reason,omitempty"`
	ViolationHead    string     `json:"violation_head,omitempty"`
	ViolationRecords *int64     `json:"violation_records,omitempty"`
	ViolationAt      *time.Time `json:"violation_at,omitempty"`
}

// Compromised reports whether this anchor is carrying an unresolved finding.
func (a LogAnchor) Compromised() bool { return a.ViolationReason != "" }

// ClassifyLogHead is the rule, as a pure function.
//
// Pure so it can be tested for what it is — a comparison — rather than through a
// database. The two healthy cases are stated explicitly rather than falling out
// of a default, because a default here means "anything I did not think of is
// fine", on the check whose whole purpose is noticing something nobody thought
// of.
func ClassifyLogHead(prevHead string, prevRecords int64, head string, records int64) string {
	switch {
	case records < prevRecords:
		return AnchorRecordsDecreased
	case records == prevRecords && head != prevHead:
		return AnchorHeadRewritten
	case records == prevRecords:
		return AnchorUnchanged
	case head == prevHead:
		// More records and the same head. The head is a digest over the chain,
		// so this cannot happen honestly — it is a stalled or fabricated head
		// attached to a growing count, which is the shape of a log being written
		// past without being chained.
		return AnchorHeadRewritten
	default:
		return AnchorExtended
	}
}

// AnchorViolation reports whether a verdict is a finding rather than a healthy
// reading. Exported so the surface and the writer cannot disagree about which
// verdicts are which.
func AnchorViolation(verdict string) bool {
	return verdict == AnchorRecordsDecreased || verdict == AnchorHeadRewritten
}

// RecordLogHead compares a reported head against the anchor and moves the anchor
// only if the report extends it.
//
// The anchor deliberately does not advance past a violation. Advancing would
// adopt the tampered state as the new baseline and report every subsequent read
// as healthy — which is the one thing an anchor must never do, because the whole
// mechanism is a memory of what was true before.
func RecordLogHead(ctx context.Context, target, head string, records int64) (LogAnchor, string, error) {
	switch {
	case strings.TrimSpace(target) == "":
		return LogAnchor{}, "", fmt.Errorf("anchor log head: no target")
	case records < 0:
		return LogAnchor{}, "", fmt.Errorf("anchor log head for %s: negative record count", target)
	}
	// An empty head is NOT refused here, and where that check moved to is the
	// point. It used to sit beside the two above, which meant the single
	// loudest signal this mechanism can receive was discarded before it was
	// read: a log that has been deleted outright reports no head and no
	// records, and against an anchor that remembers three records that is a
	// truncation, not an absence of evidence. The refusal now applies only
	// where it is true — the FIRST sighting of a target, where there is nothing
	// to compare against and an empty head recorded as the baseline would
	// compare equal to the next empty one and read as healthy forever.

	tx, err := PG.Begin(ctx)
	if err != nil {
		return LogAnchor{}, "", fmt.Errorf("begin anchor tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Locked, so two readers of the same target cannot both decide they are the
	// first sighting and both write one.
	//
	// COALESCE, because `violation_reason` is NULL on every healthy anchor and
	// a NULL will not scan into a string. Without it this read failed for every
	// target that had never been tampered with — which is every target — so the
	// anchor was written once at first sighting and never compared against
	// anything again. A tamper-evidence mechanism that stops after its first
	// observation is worse than none: it reports a clean chain forever.
	const read = `
		SELECT head, records, COALESCE(violation_reason, '')
		  FROM addon_log_anchors
		 WHERE target = $1
		 FOR UPDATE`
	var prevHead, prevReason string
	var prevRecords int64
	err = tx.QueryRow(ctx, read, target).Scan(&prevHead, &prevRecords, &prevReason)

	verdict := AnchorFirstSighting
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		if strings.TrimSpace(head) == "" {
			return LogAnchor{}, "", fmt.Errorf("anchor log head for %s: the target reported no chain head and there is nothing to compare it against", target)
		}
		const insert = `
			INSERT INTO addon_log_anchors (target, head, records)
			VALUES ($1, $2, $3)`
		if _, err := tx.Exec(ctx, insert, target, head, records); err != nil {
			return LogAnchor{}, "", fmt.Errorf("anchor %s: %w", target, err)
		}
	case err != nil:
		return LogAnchor{}, "", fmt.Errorf("read anchor for %s: %w", target, err)
	default:
		verdict = ClassifyLogHead(prevHead, prevRecords, head, records)
		if AnchorViolation(verdict) {
			// The anchor stays where it is. Only the finding is written.
			const flag = `
				UPDATE addon_log_anchors
				   SET violation_reason = $2, violation_head = $3,
				       violation_records = $4, violation_at = NOW()
				 WHERE target = $1`
			if _, err := tx.Exec(ctx, flag, target, verdict, head, records); err != nil {
				return LogAnchor{}, "", fmt.Errorf("record log violation for %s: %w", target, err)
			}
		} else if prevReason == "" {
			// Moved only while the target is clean. An anchor carrying an
			// unresolved finding must not quietly catch up to the tampered chain
			// the next time it happens to extend.
			const advance = `
				UPDATE addon_log_anchors
				   SET head = $2, records = $3, anchored_at = NOW()
				 WHERE target = $1`
			if _, err := tx.Exec(ctx, advance, target, head, records); err != nil {
				return LogAnchor{}, "", fmt.Errorf("advance anchor for %s: %w", target, err)
			}
		}
	}

	anchor, err := readAnchorTx(ctx, tx, target)
	if err != nil {
		return LogAnchor{}, "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return LogAnchor{}, "", fmt.Errorf("commit anchor: %w", err)
	}
	return anchor, verdict, nil
}

// GetLogAnchor reads one target's anchor.
func GetLogAnchor(ctx context.Context, target string) (LogAnchor, bool, error) {
	a, err := readAnchor(ctx, querier(ctx).QueryRow(ctx, anchorSelect, target))
	if errors.Is(err, pgx.ErrNoRows) {
		return LogAnchor{}, false, nil
	}
	if err != nil {
		return LogAnchor{}, false, fmt.Errorf("read anchor for %s: %w", target, err)
	}
	return a, true, nil
}

// There was a ListCompromisedLogs here, for a cross-target findings surface
// that was never built. It extended anchorSelect — whose `WHERE ($1 = '' OR
// target = $1)` needs an argument — and passed none, so pgx refused every call
// it was ever given. Having no caller is what hid that: dead code cannot fail a
// test, and a broken read behind a surface nobody opens is indistinguishable
// from a working one.
//
// Deleted rather than repaired. A finding already reaches the operator on the
// target's own health card, which is where they act on it; a second listing
// with no screen is a second thing to keep correct for no reader. It comes back
// when there is a surface that wants it, and then it comes back with a test.

// anchorSelect ends in a predicate the callers extend, so the column list and
// the scan order are written once.
const anchorSelect = `
	SELECT target, head, records, anchored_at,
	       violation_reason, violation_head, violation_records, violation_at
	  FROM addon_log_anchors
	 WHERE ($1 = '' OR target = $1)`

type scanner interface{ Scan(dest ...any) error }

func readAnchor(_ context.Context, row scanner) (LogAnchor, error) {
	var a LogAnchor
	var reason, vHead *string
	if err := row.Scan(&a.Target, &a.Head, &a.Records, &a.AnchoredAt,
		&reason, &vHead, &a.ViolationRecords, &a.ViolationAt); err != nil {
		return LogAnchor{}, err
	}
	if reason != nil {
		a.ViolationReason = *reason
	}
	if vHead != nil {
		a.ViolationHead = *vHead
	}
	return a, nil
}

func readAnchorTx(ctx context.Context, tx pgx.Tx, target string) (LogAnchor, error) {
	a, err := readAnchor(ctx, tx.QueryRow(ctx, anchorSelect, target))
	if err != nil {
		return LogAnchor{}, fmt.Errorf("read anchor back for %s: %w", target, err)
	}
	return a, nil
}
