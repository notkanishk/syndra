package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// The HTTP surface (design §5).
//
//	GET  /capabilities   entitlement schema, operation set, product + version
//	GET  /subjects       full state read, feeding the backend's drift sweep
//	GET  /health         reachability, version, key expiry, log head, lifecycle
//	POST /apply          converge one subject's resolved entitlement state
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
	product   string

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
	mux.HandleFunc("POST /apply", s.auth.authenticated(s.handleApply))
	mux.HandleFunc("POST /plan", s.auth.authenticated(s.handlePlan))
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
	writeJSON(w, http.StatusOK, manifest(s.product, s.nas.Version(), s))
}

// availability answers the capability probe for one operation.
//
// Two reasons an operation is unavailable, and they read differently to an
// operator: the target's major is outside the tested range, or the operation
// needs a privilege this add-on's credential deliberately excludes.
func (s *server) availability(operationID string) (bool, string) {
	if supported, why := s.nas.MajorSupported(); !supported {
		return false, why
	}
	if operationID == "account.purge" {
		// The add-on's own key cannot delete. Purge runs on a second elevated
		// credential the backend injects into that single call, so a compromised
		// add-on can misassign, disable and rotate but cannot delete an account
		// on its own (design §10).
		return true, ""
	}
	return true, ""
}

// Health is what an operator reads and what the backend anchors.
type Health struct {
	Reachable      bool      `json:"reachable"`
	Product        string    `json:"product"`
	ProductVersion string    `json:"product_version"`
	VersionTested  bool      `json:"version_tested"`
	VersionNote    string    `json:"version_note,omitempty"`
	LastReadAt     *string   `json:"last_read_at,omitempty"`
	KeyExpiresAt   *string   `json:"key_expires_at,omitempty"`
	Lifecycle      string    `json:"lifecycle"`
	LifecycleNote  string    `json:"lifecycle_note,omitempty"`
	InFlight       int64     `json:"in_flight"`
	Drained        bool      `json:"drained"`
	LogHead        string    `json:"log_head"`
	LogRecords     uint64    `json:"log_records"`
	SnapshotAge    *string   `json:"snapshot_taken_at,omitempty"`
	CheckedAt      time.Time `json:"checked_at"`
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
	if !s.keyExpiry.IsZero() {
		formatted := s.keyExpiry.UTC().Format(time.RFC3339)
		h.KeyExpiresAt = &formatted
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
