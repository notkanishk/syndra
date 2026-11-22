package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"syndra/addons/strictjson"
)

// The HTTP surface (design §5).
//
//	GET  /capabilities   entitlement schema, operation set, product + version
//	GET  /subjects       full state read, feeding the backend's drift sweep
//	GET  /values/{field} what a mapping may bind that field to
//	GET  /health         reachability, version, key expiry, log head, lifecycle
//	POST /apply          converge one subject's resolved entitlement state
//	POST /lifecycle      set active / draining / read_only at runtime
//	POST /operations/*   one-shot operation from the manifest
//
// Every route is behind the same authenticator, applied to the mux rather than
// per handler: one route wrapped and the next not is the arrangement that fails
// on the endpoint somebody adds in a hurry, and on this service that endpoint
// is reachable by anything on the internal network.

type server struct {
	auth      *authenticator
	nas       *NAS
	store     *Store
	log       *MutationLog
	life      *lifecycle
	keyExpiry time.Time
	// keyNeverExpires is the operator saying so, rather than this add-on
	// inferring it from an absent date. A key with no expiry and a key whose
	// expiry nobody wrote down look identical from here, and only one of them
	// is a problem.
	keyNeverExpires bool
	// connection is how a member reaches the target, for the instructions on
	// their own page. Nil when the deployment has not said, and the manifest
	// then omits it rather than guessing from the API URL — a share path that
	// does not work teaches a member to distrust the whole page.
	connection *Connection

	// elevated opens a one-off session under a credential the backend injects
	// for a single call. A seam because a test must be able to observe that the
	// key is used once and kept nowhere — and because the long-lived session
	// must never become delete-capable, which is the property the separate
	// dialer exists to hold.
	elevated func(apiKey string) (rpc, error)

	// idFloor and idCeiling bound the uids and gids this add-on hands out
	// (identity.go). Zero means the defaults.
	idFloor, idCeiling int64

	// shares is the set of SMB shares Syndra considers, nil for every enabled
	// one. A share outside it is not read, offered, counted or audited.
	shares map[string]bool

	// fresh keeps the last read of each thing a read may be answered from
	// (fresh.go, feature freshness).
	fresh fresher

	// calls is every write executing now, by call_id, and subjects the one
	// write per subject allowed at a time (calls.go).
	calls    callLeases
	subjects subjectLocks

	// stats is the zone statistics count days in and the clock
	// (stats.go); the collector itself is stats_collect.go.
	stats statsClock
}

func (s *server) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /capabilities", s.auth.authenticated(s.handleCapabilities))
	mux.HandleFunc("GET /health", s.auth.authenticated(s.handleHealth))
	mux.HandleFunc("GET /subjects", s.auth.authenticated(s.handleSubjects))
	// What a field's values may be, so the backend can check that a mapping
	// names something real. A read rather than manifest content: group
	// membership is runtime state, and a cached manifest would refuse a group
	// created five minutes ago.
	mux.HandleFunc("GET /values/{field}", s.auth.authenticated(s.handleValues))
	// What each group opens, read from the folder ACLs (reach.go).
	mux.HandleFunc("GET /reach", s.auth.authenticated(s.handleReach))
	// System-wide choices: where home folders go (settings.go).
	mux.HandleFunc("GET /settings", s.auth.authenticated(s.handleSettings))
	// Every home folder's use and limit (home.go).
	mux.HandleFunc("GET /homes", s.auth.authenticated(s.handleHomes))
	mux.HandleFunc("PUT /settings/{name}", s.auth.authenticated(s.handlePutSetting))
	mux.HandleFunc("POST /apply", s.auth.authenticated(s.handleApply))
	mux.HandleFunc("POST /plan", s.auth.authenticated(s.handlePlan))
	// The runtime setter §18 says the three states need. Without it the only
	// one was an environment variable read at startup — which is the redeploy
	// the design says a maintenance mode must not require.
	mux.HandleFunc("POST /lifecycle", s.auth.authenticated(s.handleLifecycle))
	// The path segment is the operation NAME. The dedup token is the call id,
	// inside the body. They were briefly the same word and that would have cost
	// a debugging session, so nothing here lets one be passed where the other
	// is meant.
	mux.HandleFunc("POST /operations/{name}", s.auth.authenticated(s.handleOperation))
	// What was recorded for a write, by its call id (feature calls).
	mux.HandleFunc("GET /calls/{call_id}", s.auth.authenticated(s.handleCall))
	// One metric's daily points from the buffer (feature stats).
	mux.HandleFunc("GET /stats", s.auth.authenticated(s.handleStats))
	// Everything else, authenticated like the rest, is a registered refusal:
	// every non-2xx is a problem with a code (design §11: an undeclared
	// feature's route simply answers 404, and the backend reads that 404).
	mux.HandleFunc("/", s.auth.authenticated(func(w http.ResponseWriter, _ *http.Request, _ []byte) {
		writeError(w, http.StatusNotFound, CodeUnknownRoute, "This add-on has no such route.", nil)
	}))
	return mux
}

// handleCapabilities serves the manifest.
//
// Served even when the NAS is unreachable, and that is deliberate. A capability
// set that vanished during an outage would make the backend withdraw operations
// that are merely unobservable — the same mistake as concluding an absence from
// a read that could not happen.
func (s *server) handleCapabilities(w http.ResponseWriter, r *http.Request, _ []byte) {
	writeJSON(w, http.StatusOK, manifest(productName, s.nas.Version(), s, s.connection, s.fresh.cooldownISO(), s.stats.loc().String()))
}

// availability answers the capability probe for one operation (4.7).
//
// Per operation rather than per target version, because a supported release may
// still lack a specific method: the research behind this design found methods
// moving across TrueNAS releases, and UniFi Access carries per-feature floors
// throughout. An operation the target cannot perform is declared unavailable
// with a reason and rendered disabled-and-explained, rather than omitted —
// which leaves an operator wondering whether the feature exists — or left to
// fail on use.
//
// The reasons read differently to an operator and are kept apart for that:
// an untested major is "we will not", a missing method is "it cannot".
func (s *server) availability(operationID string) (bool, string) {
	code, why := s.unavailable(operationID)
	return code == "", why
}

// unavailable is why an operation cannot run against this target now, as the
// code a call to it is refused with and the reason; "" when it can. The one
// source for the manifest's `available` and for handleOperation's gate, so the
// manifest never offers what the route then refuses.
func (s *server) unavailable(operationID string) (code, why string) {
	methods := methodsFor(operationID)
	if len(methods) == 0 {
		// Local only (account.release): nothing on the target is involved, so
		// nothing about the target's version or reachability can withdraw it.
		return "", ""
	}
	if supported, why := s.nas.MajorSupported(); !supported {
		return CodeTargetVersionUnsupported, why
	}
	for _, method := range methods {
		if !s.nas.MethodPresent(method) {
			return CodeOperationFailed, fmt.Sprintf("this target does not expose %s", method)
		}
	}
	return "", ""
}

// methodsFor names the target methods an operation depends on.
//
// A table rather than a probe per call site, so adding an operation without
// declaring what it needs is visible here instead of surfacing as a runtime
// failure on the day somebody uses it against an older release.
func methodsFor(operationID string) []string {
	switch operationID {
	case "password.set", "password.rotate":
		return []string{"user.update"}
	case "account.purge":
		return []string{"user.delete"}
	case "account.release":
		// NO target methods. Releasing is a local decision to stop claiming an
		// account, and declaring a dependency it does not have would make it
		// unavailable on a target that is merely unreachable — which is exactly
		// when an operator is most likely to need it.
		return nil
	case "account.adopt":
		// The read it verifies the account with. It writes nothing on the
		// target — the binding is local — so this is the whole dependency.
		return []string{"user.query"}
	case "activity.get":
		// Both, because a report that could read the audit log but not the
		// share list would be the silently-incomplete answer this operation
		// exists to avoid giving.
		return []string{"audit.query", "sharing.smb.query"}
	case "health.get":
		return []string{"system.info", "alert.list", "pool.query", "service.query"}
	case "storage.status":
		// The account read is what the operation is FOR — a member who cannot
		// be told their usage can still be told whether their account works,
		// and that is the more urgent half. The quota read is best-effort
		// inside the handler and is declared so a target lacking it says so at
		// registration rather than at the moment a member opens the page.
		return []string{"user.query", "sharing.smb.query", "pool.dataset.get_quota"}
	}
	return nil
}

// Health is what an operator reads and what the backend anchors.
type Health struct {
	Reachable      bool    `json:"reachable"`
	Product        string  `json:"product"`
	ProductVersion string  `json:"product_version"`
	VersionTested  bool    `json:"version_tested"`
	VersionNote    string  `json:"version_note,omitempty"`
	LastReadAt     *string `json:"last_read_at,omitempty"`
	// CredentialExpiresAt is the API key's expiry (contract 2's name for what
	// was key_expires_at).
	CredentialExpiresAt *string `json:"credential_expires_at,omitempty"`
	// CredentialExpiry says which of three states the credential is in, so an
	// absent date is never read as a probe that failed. `set` carries a date
	// above; `none` is a key the operator deliberately issued without an
	// expiry; `unrecorded` is a date nobody told this add-on, which is the one
	// an operator should act on — a key CAN expire without Syndra knowing, and
	// a silently expired key looks exactly like an outage.
	CredentialExpiry string `json:"credential_expiry"`
	// UnauditedShares names SMB shares with auditing switched off.
	//
	// On the health surface rather than only inside `activity.get`, because an
	// activity report that comes back empty is indistinguishable from a member
	// who did nothing — and the operator only learns the difference by running
	// the report they had no reason to run. Every share unaudited means the
	// feature cannot work at all, and that is a deployment fact worth knowing
	// before somebody depends on it.
	UnauditedShares []string `json:"unaudited_shares,omitempty"`
	// SharesReadable is whether the share list could be read at all. Without
	// it, "no unaudited shares" and "could not look" are the same empty list.
	SharesReadable bool `json:"shares_readable"`
	// CircuitOpen says the add-on is refusing its own calls. Distinct from
	// unreachable: an operator seeing only "unreachable" looks at the network,
	// when what happened is that this backed off to avoid a lockout.
	CircuitOpen   bool      `json:"circuit_open"`
	Lifecycle     string    `json:"lifecycle"`
	LifecycleNote string    `json:"lifecycle_note,omitempty"`
	InFlight      int64     `json:"in_flight"`
	Drained       bool      `json:"drained"`
	LogHead       string    `json:"log_head"`
	LogRecords    uint64    `json:"log_records"`
	SnapshotAge   *string   `json:"snapshot_taken_at,omitempty"`
	CheckedAt     time.Time `json:"checked_at"`
	// TargetReads is every read of the NAS since the add-on started, this
	// health read's own included (feature freshness; NAS.reads).
	TargetReads uint64 `json:"target_reads"`
}

// healthReachWithin bounds every call /health makes to the NAS. A health
// endpoint answers fast and bounds each dependency it checks (the Kubernetes
// probe guidance; the health-check API pattern). Syndra's probe gives /health
// 5 s (probeTimeout, backend/internal/addons/deps.go), and a /health that
// waited out a blackholed NAS's dial answered after it, so the backend
// blamed the add-on for the NAS (the 8g staging run, 22 Nov 2026). 2 s
// leaves the rest of the 5 for TLS and the network; the guide's bound at the
// kit is 3 s (AUTHORING.md, GET /health). Keep it under both.
const healthReachWithin = 2 * time.Second

// within answers fn's result, or done false once ctx ends first. fn runs on,
// and its late answer is dropped: a NAS call cannot be cancelled, only not
// waited for. Each ends by its own timeouts (nas.go): session() dials for one
// caller at a time, and those waiting on it answer at once on the reconnect
// cooldown when the dial fails.
func within[T any](ctx context.Context, fn func() T) (v T, done bool) {
	got := make(chan T, 1)
	go func() { got <- fn() }()
	select {
	case v = <-got:
		return v, true
	case <-ctx.Done():
		return v, false
	}
}

// handleHealth composes the operator shape, degrading per source rather than
// failing whole.
//
// A health endpoint that returns 500 because one of five reads failed tells an
// operator nothing about the other four, which are the ones that would have
// explained it.
func (s *server) handleHealth(w http.ResponseWriter, r *http.Request, _ []byte) {
	state, note := s.life.State()
	supported, versionNote := s.nas.MajorSupported()
	head, count := s.log.Head()

	h := Health{
		Product:        productName,
		ProductVersion: s.nas.Version(),
		VersionTested:  supported,
		VersionNote:    versionNote,
		CircuitOpen:    s.nas.CircuitOpen(),
		Lifecycle:      state,
		LifecycleNote:  note,
		InFlight:       s.life.InFlight(),
		Drained:        s.life.Drained(),
		LogHead:        head,
		LogRecords:     count,
		CheckedAt:      time.Now().UTC(),
	}

	// A real call, not a cached opinion. Reachability that reports the last
	// known answer is the field most likely to be wrong exactly when it is read.
	// Bounded: a NAS that hangs is not reachable, and says so in time.
	ctx, cancel := context.WithTimeout(r.Context(), healthReachWithin)
	defer cancel()
	h.Reachable, _ = within(ctx, func() bool { _, err := s.nas.Ping(); return err == nil })
	if t := s.nas.LastRead(); !t.IsZero() {
		formatted := t.UTC().Format(time.RFC3339)
		h.LastReadAt = &formatted
	}
	switch {
	case !s.keyExpiry.IsZero():
		formatted := s.keyExpiry.UTC().Format(time.RFC3339)
		h.CredentialExpiresAt = &formatted
		h.CredentialExpiry = "set"
	case s.keyNeverExpires:
		h.CredentialExpiry = "none"
	default:
		h.CredentialExpiry = "unrecorded"
	}
	// Read on the health path so it is seen without anyone asking for it, and
	// only when the target is answering — a share list that could not be read
	// is reported as unreadable rather than as "nothing is unaudited".
	if h.Reachable {
		type read struct {
			shares []string
			err    error
		}
		got, done := within(ctx, func() read { shares, err := s.unauditedShares(); return read{shares, err} })
		if done && got.err == nil {
			h.SharesReadable, h.UnauditedShares = true, got.shares
		}
	}
	if snap, found, err := s.store.GetSnapshot(); err == nil && found {
		formatted := snap.TakenAt.UTC().Format(time.RFC3339)
		h.SnapshotAge = &formatted
	}
	h.TargetReads = s.nas.reads.Load()
	writeJSON(w, http.StatusOK, h)
}

// SubjectsResponse carries the read AND how much to trust it.
type SubjectsResponse struct {
	Subjects []SubjectView `json:"subjects"`
	// Observed's Current says this came from the target just now: a read that
	// began after the request arrived (design §4). The backend's drift sweep
	// consumes only current reads: comparing desired state against an ageing
	// mirror would report every intervening change as out-of-band, so an
	// outage would manufacture findings rather than reporting itself.
	Observed
	// Truncated says the read hit its cap. A capped read is current and still
	// unusable for concluding an ABSENCE, which is what half the drift diff
	// does — so the flag travels separately from `Current`.
	Truncated bool `json:"truncated"`
	// Conflicts are the bindings that name no account in this read because another account holds
	// their name or number (attribute): never absent, and never a row's subject.
	Conflicts []SubjectConflict `json:"conflicts"`
}

// Stale is why a read reply is not current (common/stale.json): `unreachable`
// when the live read failed and the last one answers, `cached` when /reach
// serves its five-minute cache on purpose (a deliberate mirror answer is never
// current, design §4).
type Stale struct {
	Reason     string `json:"reason"`
	RetryAfter int    `json:"retry_after,omitempty"`
	Note       string `json:"note,omitempty"`
}

func unreachableSince(at time.Time) *Stale {
	return &Stale{Reason: "unreachable",
		Note: "The NAS could not be read just now; this is the last read, from " + at.UTC().Format(time.RFC3339) + "."}
}

// Standing is why a converged account can or cannot be used yet, and who is
// waited on (design §6). Observed, never desired.
type Standing struct {
	Usable    bool   `json:"usable"`
	WaitingOn string `json:"waiting_on"`
	Note      string `json:"note,omitempty"`
	Next      string `json:"next,omitempty"`
}

// standingOf is the account's standing, from what the NAS reported. The
// sentence for a missing password is smbWaiting's, the one the outcome detail
// already uses, so the member page and the apply never disagree.
func standingOf(s *Subject) *Standing {
	switch {
	case s.Self:
		// The add-on's own service account, used through its API key.
		return &Standing{Usable: true, WaitingOn: "none"}
	case !s.Enabled && s.unbound:
		// Nobody in Syndra is assigned to it, so nothing in Syndra waits on anyone for it: `none`, as
		// AUTHORING's standing rule allows for an unassigned off account, but said as what it is and
		// never as Syndra's doing.
		return &Standing{WaitingOn: "none", Note: "Switched off on TrueNAS. No one in Syndra is assigned to it.",
			Next: "Nothing for you to do unless you assign it to a person."}
	case !s.Enabled && s.wantsEnabled != nil && *s.wantsEnabled:
		// Syndra wants it on and TrueNAS has it off: a difference made outside Syndra. The way back is
		// Syndra's own check for differences, which re-sends what it wants.
		return &Standing{WaitingOn: "operator", Note: "Switched off on TrueNAS, outside Syndra.",
			Next: "Makerspace staff can use Check for differences on the system's Mappings page to bring it back in line."}
	case !s.Enabled && s.wantsEnabled != nil:
		// Syndra asked for it off: what it asked for is what it has, so nobody is waited on.
		// `next` is shown to the member whose account it is as well as to the operator, so it reads
		// right to both.
		return &Standing{WaitingOn: "none",
			Next: "Nothing for you to do. It stays off while the person it belongs to is on hold or deactivated, or holds no role that reaches TrueNAS."}
	case !s.Enabled:
		// A bound account whose wish is unknown (a binding never applied since wishes were kept): no
		// claim either way. The wish fills in on its next apply (rememberWishes). No `next` of ours:
		// Syndra supplies one for each audience (an operator is sent to the comparison, a member to
		// makerspace staff), which a line written once here could not do.
		return &Standing{WaitingOn: "none", Note: "Switched off on TrueNAS."}
	case !s.PasswordSet:
		return &Standing{WaitingOn: "member", Note: smbWaiting(s), Next: "Set a storage password on your storage page."}
	case s.needsNewPassword():
		return &Standing{WaitingOn: "member", Note: smbWaiting(s), Next: "Set a new storage password on your storage page."}
	}
	return &Standing{Usable: true, WaitingOn: "none"}
}

// accountID is the app's id for an account, as contract 2 names it (`account_id`, a string,
// decision 26): TrueNAS's is the unix uid, and nothing else is ever stamped as one. Unique within
// one read and kept across a rename; not an identity across reads (AUTHORING, chapter 2).
func accountID(uid int64) string { return strconv.FormatInt(uid, 10) }

// SubjectView is one account as served: what the NAS reports, plus the same
// state expressed in ENTITLEMENT vocabulary, plus its standing.
//
// `state` exists because reconciliation compares three values per field and had
// only two — and the third comparison has to be against the target's current
// value, which the backend could not read off this response. It could have been
// mapped there, from `groups` to `group` and `locked` to `enabled`; that would
// put "what TrueNAS calls things" in the component that must not know, which is
// the translation `readSubjects` already refuses to leak.
//
// Its own type rather than the stored Subject: the wire is contract 2's
// (`account_id`, `standing`, no `uid` or `password_set`), and the snapshot in
// the store keeps the shape an older binary reads.
type SubjectView struct {
	ID        apiID  `json:"id"`
	Username  string `json:"username"`
	AccountID string `json:"account_id"`
	// Subject is the subject this add-on's binding names the account for (attribute), absent when
	// none does: SCIM's externalId to the add-on's id (RFC 7643 §3.1). Never inferred from a uid.
	Subject    string         `json:"subject,omitempty"`
	Groups     []string       `json:"groups"`
	Home       string         `json:"home,omitempty"`
	Enabled    bool           `json:"enabled"`
	SMBEnabled bool           `json:"smb_enabled"`
	Self       bool           `json:"self,omitempty"`
	State      map[string]any `json:"state"`
	Standing   *Standing      `json:"standing"`
}

// subjectViews serves subjects; limits holds each account's effective home
// folder limit where it was read (homeLimits), and an account absent from it
// reports no home_quota, which is "not known", never "no limit". wishes is
// the bindings' wishes by uid (Store.Wishes), so a standing says when an
// account waits on a new password, and whether an account that is off was
// switched off by Syndra, as storage.status does. An account with no entry
// is bound to nobody; a nil map is "not read", and says nothing of either.
// subjectOf is attribute's: the subject each row's account is bound to.
func subjectViews(subjects []Subject, limits map[int64]string, wishes map[int64]Wish, subjectOf map[rowKey]string) []SubjectView {
	out := make([]SubjectView, 0, len(subjects))
	for i := range subjects {
		s := subjects[i]
		s.applyWish(wishes)
		state := entitlementState(&s)
		if limit, ok := limits[s.UID]; ok {
			state[FieldHomeQuota] = []string{limit}
		}
		out = append(out, SubjectView{
			ID: s.ID, Username: s.Username, AccountID: accountID(s.UID), Subject: subjectOf[rowOf(&s)],
			Groups: append([]string{}, s.Groups...), Home: s.Home,
			Enabled: s.Enabled, SMBEnabled: s.SMBEnabled, Self: s.Self,
			State: state, Standing: standingOf(&s),
		})
	}
	return out
}

// wantsFresh is the request asking for a read of the NAS now (`?fresh=true`,
// feature freshness). Only the literal true asks.
func wantsFresh(r *http.Request) bool { return r.URL.Query().Get("fresh") == "true" }

// subjectsRead is one read of every account, as served: the rows and the conflicts, attributed
// over that read (attribute). Kept as the copy with it: every write forgets the copy (fresh.go),
// a binding's included, so a copy never outlives the bindings it was attributed by.
type subjectsRead struct {
	views     []SubjectView
	conflicts []SubjectConflict
	truncated bool
}

// attributed is one read of the accounts with the bindings over it: each row's subject and the
// conflicts. A store that cannot be read is errStoreUnreadable, never a read with no row bound and
// no conflict, which would say every subject's account is gone (Syndra reads a missing account as
// deleted on the NAS).
func (s *server) attributed(subjects []Subject, limits map[int64]string, truncated bool) (subjectsRead, error) {
	at, err := s.attribute(subjects)
	if err != nil {
		return subjectsRead{}, unreadable(err)
	}
	return subjectsRead{subjectViews(subjects, limits, at.wishes(), at.subjectOf), at.conflicts, truncated}, nil
}

// handleSubjects serves the full state read: the copy, a read of the NAS now,
// or the mirror on disk when the NAS cannot be read and nothing newer is kept
// (fresh.go) — labelled whichever it is. Bindings that cannot be read are 500
// STORE_UNREADABLE, as /plan answers it, unless a copy attributed earlier is
// kept (the copy, labelled unreachable).
func (s *server) handleSubjects(w http.ResponseWriter, r *http.Request, _ []byte) {
	val, obs, err := s.fresh.read("subjects", wantsFresh(r), func() (any, time.Time, error) {
		snap, err := s.readSubjects()
		if err != nil {
			return nil, time.Time{}, err
		}
		if putErr := s.store.PutSnapshot(snap); putErr != nil {
			// The read is good; only the mirror failed. Logged and served,
			// because refusing a current read to protect a backstop inverts
			// which of the two matters.
			log.Printf("[STORE] could not persist snapshot: %v", putErr)
		}
		read, err := s.attributed(snap.Subjects, s.homeLimits(snap.Subjects), snap.Truncated)
		return read, snap.TakenAt, err
	})
	if writeCooldown(w, err) {
		return
	}
	if err == nil {
		read := val.(subjectsRead)
		writeJSON(w, http.StatusOK, SubjectsResponse{Subjects: read.views, Conflicts: read.conflicts, Truncated: read.truncated, Observed: obs})
		return
	}
	if errors.Is(err, errStoreUnreadable) {
		writeStoreUnreadable(w, err)
		return
	}

	cached, found, cacheErr := s.store.GetSnapshot()
	if cacheErr != nil || !found {
		// Nothing current and nothing remembered: an error, never an empty
		// list, which is a statement that the target holds no accounts and the
		// drift sweep would act on it. TARGET_UNREACHABLE when no request
		// reached the NAS (B3), NO_CURRENT_READ when it answered and failed.
		log.Printf("[SUBJECTS] live read failed (%v) and no snapshot is available", err)
		writeNotRead(w, err)
		return
	}
	// The mirror holds no limits: they are read live or not reported.
	read, err := s.attributed(cached.Subjects, nil, cached.Truncated)
	if err != nil {
		writeStoreUnreadable(w, err)
		return
	}
	writeJSON(w, http.StatusOK, SubjectsResponse{
		Subjects: read.views, Conflicts: read.conflicts, Truncated: read.truncated,
		Observed: Observed{TakenAt: rfc3339(cached.TakenAt), Stale: unreachableSince(cached.TakenAt)},
	})
}

// writeStoreUnreadable answers a read whose bindings could not be read.
func writeStoreUnreadable(w http.ResponseWriter, err error) {
	log.Printf("[STORE] the bindings could not be read for /subjects: %v", err)
	writeError(w, http.StatusInternalServerError, CodeStoreUnreadable,
		subjectsStoreUnreadable, nil)
}

// The idempotency namespace's type tags. One namespace so a call id reused
// anywhere is caught, and a tag so a cached result is never decoded as the
// wrong shape — which is a zero-valued success, the worst answer available.
const (
	kindApply     = "apply"
	kindOperation = "operation:"
	kindSetting   = "setting:"
	kindLifecycle = "lifecycle"
)

// decodeStrict parses a request body and refuses what it does not understand:
// a member it has no field for, a member whose name is a case or fold variant
// of a field's (RFC 8259 §4: names compare code point for code point), the
// same member twice (RFC 7493 §2.3), and anything after the body.
//
// The project's rule for its own mutation endpoints, applied here for a sharper
// reason: this boundary is between two separately deployed binaries, and a
// field the sender thinks it is honouring and this one silently drops is
// exactly the skew the contract version exists to surface.
func decodeStrict(body []byte, into any) error { return strictjson.Decode(body, into) }

// decodeRequest is decodeStrict answering its own refusal: an unknown member
// is 400 UNKNOWN_REQUEST_FIELD, and any other undecodable body (a duplicate
// member among them) is 400 under the route's own code (BAD_REQUEST, or
// INVALID_BODY for a setting).
func decodeRequest(w http.ResponseWriter, body []byte, into any, malformed string) bool {
	err := decodeStrict(body, into)
	var unknown *strictjson.UnknownMemberError
	var duplicate *strictjson.DuplicateMemberError
	switch {
	case err == nil:
		return true
	case errors.As(err, &unknown):
		writeError(w, http.StatusBadRequest, CodeUnknownRequestField,
			"The request carries a field this add-on does not know ("+unknown.Error()+").", nil)
	case errors.As(err, &duplicate):
		writeError(w, http.StatusBadRequest, malformed, "The body is not the JSON this route takes ("+duplicate.Error()+").", nil)
	default:
		writeError(w, http.StatusBadRequest, malformed, "The body is not the JSON this route takes.", nil)
	}
	return false
}

// writeContractRefusal enforces the wire version on every request that carries
// a body, and reports whether the caller may proceed.
//
// Registration already refuses an add-on whose manifest declares an unsupported
// version, so this is the second half of the same check rather than the only
// one — and it is the half that survives a deployment where the backend was
// upgraded without a re-registration. It names BOTH versions because "the
// add-on is newer" and "the backend is newer" send an operator to different
// binaries, and a refusal that says only "mismatch" makes them guess.
//
// An absent field reads as version 0 and is refused like any other mismatch: a
// caller that omits it is a caller from before the field existed, which is
// exactly the skew being checked for.
func writeContractRefusal(w http.ResponseWriter, declared int) (ok bool) {
	if declared == ContractVersion {
		return true
	}
	writeError(w, http.StatusUnprocessableEntity, CodeContractVersionMismatch,
		fmt.Sprintf("This add-on speaks contract %d; the request declared %d.", ContractVersion, declared),
		map[string]any{"caller_version": declared, "addon_version": ContractVersion})
	return false
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("[HTTP] could not write response: %v", err)
	}
}

// logStoreFailure records a local-state write that did not land.
//
// Loud, and never fatal to a mutation that already happened: refusing to report
// a completed change because its cache entry failed would lose the only account
// of it the caller gets. What makes these visible rather than merely logged is
// the head digest the backend anchors and the binding conflict the next apply
// would raise.
func logStoreFailure(what, key string, err error) {
	log.Printf("[STORE] %s write failed for %s: %v", what, key, err)
}

// logRefusal records an authentication failure.
//
// Method and path and the reason, never the body or the headers. The body is
// where a member's credential is, and an unauthenticated request is exactly the
// one somebody might send a credential to on purpose.
func logRefusal(r *http.Request, err error) {
	log.Printf("[AUTH] refused %s %s: %v", r.Method, r.URL.Path, err)
}
