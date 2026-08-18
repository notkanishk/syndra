package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"
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
	product         string
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
	return mux
}

// handleCapabilities serves the manifest.
//
// Served even when the NAS is unreachable, and that is deliberate. A capability
// set that vanished during an outage would make the backend withdraw operations
// that are merely unobservable — the same mistake as concluding an absence from
// a read that could not happen.
func (s *server) handleCapabilities(w http.ResponseWriter, r *http.Request, _ []byte) {
	writeJSON(w, http.StatusOK, manifest(s.product, s.nas.Version(), s, s.connection))
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
	if supported, why := s.nas.MajorSupported(); !supported {
		return false, why
	}
	for _, method := range methodsFor(operationID) {
		if !s.nas.MethodPresent(method) {
			return false, fmt.Sprintf("this target does not expose %s", method)
		}
	}
	return true, ""
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
	KeyExpiresAt   *string `json:"key_expires_at,omitempty"`
	// KeyExpiry says which of three states the credential is in, so an absent
	// date is never read as a probe that failed. `set` carries a date above;
	// `none` is a key the operator deliberately issued without an expiry;
	// `unrecorded` is a date nobody told this add-on, which is the one an
	// operator should act on — a key CAN expire without Syndra knowing, and a
	// silently expired key looks exactly like an outage.
	KeyExpiry string `json:"key_expiry"`
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
		Product:        s.product,
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
	if _, err := s.nas.Ping(); err == nil {
		h.Reachable = true
	}
	if t := s.nas.LastRead(); !t.IsZero() {
		formatted := t.UTC().Format(time.RFC3339)
		h.LastReadAt = &formatted
	}
	switch {
	case !s.keyExpiry.IsZero():
		formatted := s.keyExpiry.UTC().Format(time.RFC3339)
		h.KeyExpiresAt = &formatted
		h.KeyExpiry = "set"
	case s.keyNeverExpires:
		h.KeyExpiry = "none"
	default:
		h.KeyExpiry = "unrecorded"
	}
	// Read on the health path so it is seen without anyone asking for it, and
	// only when the target is answering — a share list that could not be read
	// is reported as unreadable rather than as "nothing is unaudited".
	if h.Reachable {
		if shares, err := s.unauditedShares(); err == nil {
			h.SharesReadable, h.UnauditedShares = true, shares
		}
	}
	if snap, found, err := s.store.GetSnapshot(); err == nil && found {
		formatted := snap.TakenAt.UTC().Format(time.RFC3339)
		h.SnapshotAge = &formatted
	}
	writeJSON(w, http.StatusOK, h)
}

// SubjectsResponse carries the read AND how much to trust it.
type SubjectsResponse struct {
	Subjects []Subject `json:"subjects"`
	// Current says this came from the target just now. The backend's drift
	// sweep consumes only current reads: comparing desired state against an
	// ageing mirror would report every intervening change as out-of-band, so an
	// outage would manufacture findings rather than reporting itself.
	Current bool   `json:"current"`
	TakenAt string `json:"taken_at"`
	// Truncated says the read hit its cap. A capped read is current and still
	// unusable for concluding an ABSENCE, which is what half the drift diff
	// does — so the flag travels separately from `Current`.
	Truncated bool `json:"truncated"`
}

// handleSubjects serves the full state read, live if it can and from the mirror
// if it cannot — labelled either way.
func (s *server) handleSubjects(w http.ResponseWriter, r *http.Request, _ []byte) {
	snap, err := s.readSubjects()
	if err == nil {
		if putErr := s.store.PutSnapshot(snap); putErr != nil {
			// The read is good; only the mirror failed. Logged and served,
			// because refusing a current read to protect a backstop inverts
			// which of the two matters.
			log.Printf("[STORE] could not persist snapshot: %v", putErr)
		}
		writeJSON(w, http.StatusOK, SubjectsResponse{
			Subjects: snap.Subjects, Current: true,
			TakenAt: snap.TakenAt.UTC().Format(time.RFC3339), Truncated: snap.Truncated,
		})
		return
	}

	cached, found, cacheErr := s.store.GetSnapshot()
	if cacheErr != nil || !found {
		// Nothing current and nothing remembered. 503 rather than an empty
		// list: an empty list is a statement that the target holds no accounts,
		// and the drift sweep would act on it.
		log.Printf("[SUBJECTS] live read failed (%v) and no snapshot is available", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "NO_CURRENT_READ"})
		return
	}
	writeJSON(w, http.StatusOK, SubjectsResponse{
		Subjects: cached.Subjects, Current: false,
		TakenAt: cached.TakenAt.UTC().Format(time.RFC3339), Truncated: cached.Truncated,
	})
}

// The idempotency namespace's type tags. One namespace so a call id reused
// anywhere is caught, and a tag so a cached result is never decoded as the
// wrong shape — which is a zero-valued success, the worst answer available.
const (
	kindApply     = "apply"
	kindOperation = "operation:"
)

// decodeStrict parses a request body and refuses what it does not understand.
//
// The project's rule for its own mutation endpoints, applied here for a sharper
// reason: this boundary is between two separately deployed binaries, and a
// field the sender thinks it is honouring and this one silently drops is
// exactly the skew the contract version exists to surface.
func decodeStrict(body []byte, into any) error {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(into); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("the body carries more than one JSON document")
	}
	return nil
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
	writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
		"error":          "CONTRACT_VERSION_MISMATCH",
		"caller_version": declared,
		"addon_version":  ContractVersion,
	})
	return false
}

// writeRecallFailure separates "the store is broken" from "this call id was
// minted for something else". The second is the caller's mistake and a retry
// will not fix it, so it must not read as a transient failure.
func writeRecallFailure(w http.ResponseWriter, err error) {
	if errors.Is(err, errKindMismatch) {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "CALL_ID_REUSED"})
		return
	}
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "STORE_UNREADABLE"})
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
