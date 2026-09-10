package repoguard

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Who is allowed to ask Zitadel what is true.
//
// `internal/observe` (see its package doc) is the only thing that should call
// ListAllGrants or ListUserGrants on the management client. Everything else is
// supposed to read the store it writes. The incident this guards against: a
// role's own page called one of these live, a list screen called a different
// pipeline, and the two answered "0 people hold this role" beside "4 holders"
// for the same role — because they were never the same pipe.
//
// This guard is precise about what it forbids the way
// TestOnlyOneThingCountsRoleHolders is: it does not ban a FILE for importing
// `zitadel`, or for calling some other method on MgmtClient. It counts exact
// occurrences of the two grant-LISTING calls and fails on any occurrence this
// file's allowlist entry does not already account for — so a new call added
// beside a justified one is caught exactly as fast as a new call in a fresh
// file would be.
var zitadelGrantListing = regexp.MustCompile(`MgmtClient\.(ListAllGrants|ListUserGrants)\(`)

// zitadelReadExemption is one file's justified departure from "only the
// observer reads Zitadel live". allow is the exact number of matching calls
// the file is trusted to hold — not "some", so a second call added next to a
// justified one still fails the count and has to be looked at.
type zitadelReadExemption struct {
	reason string
	allow  int
}

var mayReadZitadelGrantsLive = map[string]zitadelReadExemption{
	// The outbox drain reads Zitadel live at the two moments the design calls
	// out by name (design.md, "Many checks, at the three moments that
	// matter"): BEFORE a write, to avoid one that is no longer needed, and
	// AFTER a write (`observedAfterWrite`, drain.go), to confirm the write
	// actually landed rather than trusting a 2xx. Both gate MUTATION
	// semantics on a single row, mid-batch, in a retryable drain loop — they
	// are not a display read a person waits on.
	//
	// Routing this through `observe.User` was considered and rejected: that
	// call is a FULL paginated read across every project the person holds
	// anything in, and a complete one DELETES this person's rows not seen —
	// the store's whole safety property (db.RecordUserObservation). This
	// probe (`liveUserGrantRoles`, Limit:100, ONE page, no continuation) is
	// deliberately narrower and does not declare completeness the way the
	// observer's contract requires; feeding its result through
	// RecordUserObservation would let a person with more than 100 grants
	// across projects have real rows deleted because a single-project
	// write-confirmation check only ever saw the first page. The two reads
	// answer different questions — "did MY write land" vs "what does this
	// person hold, everywhere, right now" — and only one of them is the
	// observer's job.
	"backend/internal/services/propagation/deps.go": {
		reason: "write-path correctness reads (pre-flight + read-back after write), not a display — see the comment above this entry",
		allow:  2, // zitadelReachable (ListAllGrants, Limit:1 probe) + liveUserGrantRoles (ListUserGrants)
	},

	// A liveness probe for the governance banner: Limit:1, result discarded,
	// asking only "did this call succeed" — never "what does it say". Routing
	// a health check through observe.Org would turn a once-per-30-seconds
	// ping, polled by every open dashboard, into a full paginated org-wide
	// grant listing on the same cadence.
	"backend/internal/services/zitadel_probe.go": {
		reason: "reachability probe only (Limit:1, discards the result) — not a second reader of grant state",
		allow:  1,
	},

	// The last remaining live read (one-truth-many-checks, "The last two
	// readers" — the drift sweep and reconciliation's own diff are both
	// migrated as of this pass; the drift sweep's live listing is gone from
	// drift/deps.go entirely, and reconciliation.go's handler now calls
	// observeOrg + reads the store, same as discovery.go's grant-listing
	// routes). What is left is zitadel_grant_lookup.go's fallback
	// (enrichGrantPayload, called from the webhook's own event handling when
	// the local index has no row for a grant aggregate ID yet) — a single
	// grant lookup made to enrich ONE event as it arrives, never a "how many
	// people hold this" answer rendered to a screen. Out of this pass's scope;
	// tracked in openspec/NEXT.md pending migration.
	"backend/internal/handlers/deps.go": {
		reason: "KNOWN GAP: the webhook's grant-lookup fallback still reads live — tracked in openspec/NEXT.md",
		allow:  1, // zitadelListUserGrants
	},
}

func TestOnlyTheObserverListsZitadelGrantsLive(t *testing.T) {
	root := repoRoot(t)

	var offenders []string
	counts := map[string]int{}
	err := filepath.Walk(filepath.Join(root, "backend"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch name := info.Name(); name {
			case ".gomodcache", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		// The observer and the Zitadel client package itself are where this
		// call is SUPPOSED to live — not exemptions, the destination.
		if strings.HasPrefix(rel, "backend/internal/observe/") || strings.HasPrefix(rel, "backend/internal/zitadel/") {
			return nil
		}
		b, err := os.ReadFile(path) // #nosec G304 -- walking the repo's own tree
		if err != nil {
			return err
		}
		n := len(zitadelGrantListing.FindAllString(string(b), -1))
		if n == 0 {
			return nil
		}
		counts[rel] = n
		if exemption, ok := mayReadZitadelGrantsLive[rel]; ok && exemption.allow == n {
			return nil
		}
		offenders = append(offenders, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk backend: %v", err)
	}

	// An allowlist entry for a file that no longer exists, or whose count no
	// longer matches, is either granting nobody anything (dead entry, drop
	// it) or under-counting a call that has since grown a sibling (stale
	// entry, hiding a second pipe behind the first one's excuse).
	for rel, exemption := range mayReadZitadelGrantsLive {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if _, statErr := os.Stat(full); statErr != nil {
			t.Errorf("allowlisted file does not exist, drop the entry: %s", rel)
			continue
		}
		if got := counts[rel]; got != exemption.allow {
			t.Errorf("allowlist for %s expects exactly %d live grant-listing call(s), found %d — "+
				"update the count (and re-justify it) rather than leaving it stale", rel, exemption.allow, got)
		}
	}

	if len(offenders) > 0 {
		sort.Strings(offenders)
		t.Errorf("a live ListAllGrants/ListUserGrants call outside internal/observe and internal/zitadel:\n  %s\n\n"+
			"A second reader of Zitadel grant state is a second pipe, and two pipes answering one "+
			"question is how a role came to read \"4 holders\" on a list and \"0 people hold this "+
			"role\" on the page it opens. Route the read through observe.Org / observe.User, or — if "+
			"it is a write-path correctness check rather than a display read (see the propagation "+
			"entry above) — add it to mayReadZitadelGrantsLive with the argument written out.",
			strings.Join(offenders, "\n  "))
	}
}
